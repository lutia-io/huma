package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lutia-io/huma/pkg/criteria"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/record"
	"github.com/lutia-io/huma/pkg/workflow"
)

// WorkflowDefinitionStore lists workflow definitions eligible for a trigger,
// satisfied by the workflow package's postgres store.
type WorkflowDefinitionStore interface {
	ListActiveBySchemaID(ctx context.Context, schemaID string) ([]*workflow.WorkflowDefinition, error)
	ListActiveScheduled(ctx context.Context) ([]*workflow.WorkflowDefinition, error)
}

// Enqueuer is the intake half of the engine: it turns trigger events into
// pending workflows. It performs no action side effects; execution belongs to
// the Service's worker pool.
type Enqueuer struct {
	logger              *logger.Logger
	workflowDefinitions WorkflowDefinitionStore
	workflows           WorkflowStore
}

func NewEnqueuer(logger *logger.Logger, workflowDefinitions WorkflowDefinitionStore, workflows WorkflowStore) *Enqueuer {
	return &Enqueuer{
		logger:              logger,
		workflowDefinitions: workflowDefinitions,
		workflows:           workflows,
	}
}

// EvaluateCreated loads active workflow definitions for the record's schema,
// keeps those whose trigger includes created, evaluates criteria against the
// record data, and durably inserts one pending workflow per match.
//
// Criteria are evaluated here, at intake, so the workflows table only ever
// holds real work. The definition and record data are snapshotted onto the
// workflow: crash reclaims execute the actions the workflow started with,
// templated against the data that triggered it. A manual retry keeps the
// trigger data and replaces the definition with the live one.
//
// Dedupe keys for create stay the record ID so redelivery of an existing
// create event does not insert a second run.
//
// There is no loop prevention: a workflow whose actions create or update
// records can re-trigger workflows on those records, including itself. It is
// up to definition authors not to create recursive workflow definitions.
func (e *Enqueuer) EvaluateCreated(ctx context.Context, event record.CreatedEvent) error {
	workflowDefinitions, err := e.workflowDefinitions.ListActiveBySchemaID(ctx, event.SchemaID)
	if err != nil {
		e.logger.ErrorContext(ctx, "Failed to list workflow definitions for schema", "schema_id", event.SchemaID, logger.KeyError, err)
		return err
	}

	var recordData map[string]any
	if err := json.Unmarshal(event.Data, &recordData); err != nil {
		e.logger.ErrorContext(ctx, "Failed to unmarshal record data for workflow intake", logger.KeyID, event.ID, logger.KeyError, err)
		return err
	}

	var workflows []*Workflow
	for _, def := range workflowDefinitions {
		if !def.Definition.Trigger.Includes(workflow.TriggerOnCreated) {
			continue
		}
		if !def.MatchesOrganization(event.OrganizationID) {
			continue
		}
		if !criteria.Match(def.Definition.Criteria, recordData) {
			e.logger.InfoContext(ctx, "Workflow criteria not met", logger.KeyID, def.ID, "record_id", event.ID)
			continue
		}
		workflows = append(workflows, pendingWorkflow(def, event.ID, recordData, event.OrganizationID, event.OrganizationUserID, event.ID))
	}
	return e.insertPending(ctx, event.ID, workflows)
}

// EvaluateUpdated loads active workflow definitions for the record's schema,
// keeps those whose trigger includes updated (and whose changed-field filter
// matches the before/after diff), evaluates criteria against the after
// document, and inserts one pending workflow per match.
//
// Dedupe keys include the update event ID so each real change can run, while
// JetStream redelivery of the same event is absorbed.
func (e *Enqueuer) EvaluateUpdated(ctx context.Context, event record.UpdatedEvent) error {
	workflowDefinitions, err := e.workflowDefinitions.ListActiveBySchemaID(ctx, event.SchemaID)
	if err != nil {
		e.logger.ErrorContext(ctx, "Failed to list workflow definitions for schema", "schema_id", event.SchemaID, logger.KeyError, err)
		return err
	}

	before, err := unmarshalRecordData(event.Before)
	if err != nil {
		e.logger.ErrorContext(ctx, "Failed to unmarshal record before data for workflow intake", logger.KeyID, event.ID, logger.KeyError, err)
		return err
	}
	after, err := unmarshalRecordData(event.After)
	if err != nil {
		e.logger.ErrorContext(ctx, "Failed to unmarshal record after data for workflow intake", logger.KeyID, event.ID, logger.KeyError, err)
		return err
	}
	changed := record.ChangedFields(before, after)
	dedupeKey := fmt.Sprintf("%s:updated:%s", event.ID, event.EventID)

	var workflows []*Workflow
	for _, def := range workflowDefinitions {
		if !def.Definition.Trigger.Includes(workflow.TriggerOnUpdated) {
			continue
		}
		if !def.MatchesOrganization(event.OrganizationID) {
			continue
		}
		if !changedFieldsMatch(def.Definition.Trigger.Changed, changed) {
			e.logger.InfoContext(ctx, "Workflow changed fields not met", logger.KeyID, def.ID, "record_id", event.ID)
			continue
		}
		if !criteria.Match(def.Definition.Criteria, after) {
			e.logger.InfoContext(ctx, "Workflow criteria not met", logger.KeyID, def.ID, "record_id", event.ID)
			continue
		}
		workflows = append(workflows, pendingWorkflow(def, event.ID, after, event.OrganizationID, event.OrganizationUserID, dedupeKey))
	}
	return e.insertPending(ctx, event.ID, workflows)
}

// EvaluateSchedule evaluates one scheduled definition against a page of
// current records. Dedupe keys include the UTC period start so the same tick
// is idempotent and the next period can run again.
func (e *Enqueuer) EvaluateSchedule(ctx context.Context, def *workflow.WorkflowDefinition, records []*record.Record, periodStart time.Time) error {
	if def == nil || len(records) == 0 {
		return nil
	}
	period := periodStart.UTC().Truncate(time.Minute).Format(time.RFC3339)
	var workflows []*Workflow
	for _, rec := range records {
		data, err := unmarshalRecordData(rec.Data)
		if err != nil {
			e.logger.ErrorContext(ctx, "Failed to unmarshal record data for scheduled workflow intake", logger.KeyID, rec.ID, logger.KeyError, err)
			return err
		}
		if !def.MatchesOrganization(rec.OrganizationID) {
			continue
		}
		if !criteria.Match(def.Definition.Criteria, data) {
			e.logger.InfoContext(ctx, "Workflow criteria not met", logger.KeyID, def.ID, "record_id", rec.ID)
			continue
		}
		dedupeKey := fmt.Sprintf("%s:schedule:%s", rec.ID, period)
		workflows = append(workflows, pendingWorkflow(def, rec.ID, data, rec.OrganizationID, rec.OrganizationUserID, dedupeKey))
	}
	return e.insertPending(ctx, def.ID, workflows)
}

func (e *Enqueuer) insertPending(ctx context.Context, logID string, workflows []*Workflow) error {
	if len(workflows) == 0 {
		return nil
	}
	if err := e.workflows.InsertPending(ctx, workflows); err != nil {
		e.logger.ErrorContext(ctx, "Failed to insert pending workflows", logger.KeyID, logID, logger.KeyError, err)
		return err
	}
	e.logger.InfoContext(ctx, "Inserted pending workflows", logger.KeyID, logID, logger.KeyCount, len(workflows))
	return nil
}

func pendingWorkflow(def *workflow.WorkflowDefinition, recordID string, data map[string]any, organizationID, organizationUserID, dedupeKey string) *Workflow {
	return &Workflow{
		WorkflowDefinitionID: def.ID,
		NetworkID:            def.NetworkID,
		RecordID:             recordID,
		Data:                 data,
		OrganizationID:       organizationID,
		OrganizationUserID:   organizationUserID,
		DedupeKey:            dedupeKey,
		Definition:           def.Definition,
	}
}

func unmarshalRecordData(data json.RawMessage) (map[string]any, error) {
	var recordData map[string]any
	if len(data) == 0 || string(data) == "null" {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(data, &recordData); err != nil {
		return nil, err
	}
	if recordData == nil {
		return map[string]any{}, nil
	}
	return recordData, nil
}

func changedFieldsMatch(wanted, actual []string) bool {
	if len(wanted) == 0 {
		return true
	}
	have := make(map[string]struct{}, len(actual))
	for _, field := range actual {
		have[field] = struct{}{}
	}
	for _, field := range wanted {
		if _, ok := have[field]; ok {
			return true
		}
	}
	return false
}
