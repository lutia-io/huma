package executor

import (
	"context"
	"encoding/json"

	"github.com/lutia-io/huma/pkg/criteria"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/record"
	"github.com/lutia-io/huma/pkg/workflow"
)

// WorkflowDefinitionStore lists workflow definitions eligible for a trigger,
// satisfied by the workflow package's postgres store.
type WorkflowDefinitionStore interface {
	ListActiveBySchemaID(ctx context.Context, schemaID string) ([]*workflow.WorkflowDefinition, error)
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

// EvaluateRecord loads active workflow definitions for the record's schema,
// evaluates each definition's criteria against the record data, and durably
// inserts one pending workflow per match.
//
// Criteria are evaluated here, at intake, so the workflows table only ever
// holds real work. The definition and record data are snapshotted onto the
// workflow: retries always execute the actions the workflow started with,
// templated against the data that triggered it, regardless of later edits to
// either.
//
// There is no loop prevention: a workflow whose actions create records can
// re-trigger workflows on those records, including itself. It is up to
// definition authors not to create recursive workflow definitions.
func (e *Enqueuer) EvaluateRecord(ctx context.Context, event record.CreatedEvent) error {
	workflowDefinitions, err := e.workflowDefinitions.ListActiveBySchemaID(ctx, event.SchemaID)
	if err != nil {
		e.logger.ErrorContext(ctx, "Failed to list workflow definitions for schema", "schema_id", event.SchemaID, logger.KeyError, err)
		return err
	}
	if len(workflowDefinitions) == 0 {
		return nil
	}

	var recordData map[string]any
	if err := json.Unmarshal(event.Data, &recordData); err != nil {
		e.logger.ErrorContext(ctx, "Failed to unmarshal record data for workflow intake", logger.KeyID, event.ID, logger.KeyError, err)
		return err
	}

	var workflows []*Workflow
	for _, def := range workflowDefinitions {
		if !criteria.Match(def.Definition.Criteria, recordData) {
			e.logger.InfoContext(ctx, "Workflow criteria not met", logger.KeyID, def.ID, "record_id", event.ID)
			continue
		}
		workflows = append(workflows, &Workflow{
			WorkflowDefinitionID: def.ID,
			NetworkID:            def.NetworkID,
			RecordID:             event.ID,
			Data:                 recordData,
			OrganizationID:       event.OrganizationID,
			OrganizationUserID:   event.OrganizationUserID,
			DedupeKey:            event.ID,
			Definition:           def.Definition,
		})
	}
	if len(workflows) == 0 {
		return nil
	}

	if err := e.workflows.InsertPending(ctx, workflows); err != nil {
		e.logger.ErrorContext(ctx, "Failed to insert pending workflows", "record_id", event.ID, logger.KeyError, err)
		return err
	}
	e.logger.InfoContext(ctx, "Inserted pending workflows", "record_id", event.ID, logger.KeyCount, len(workflows))
	return nil
}
