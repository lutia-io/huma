package workflow

import (
	"context"
	"encoding/json"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/logger"
)

// ExecuteForRecord loads active workflows for the record's schema, evaluates
// each definition's criteria against the record data, and runs matching actions.
func (s *Service) ExecuteForRecord(ctx context.Context, schemaID, recordID string, data json.RawMessage) error {
	workflows, err := s.store.ListActiveBySchemaID(ctx, schemaID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list workflows for schema", "schema_id", schemaID, logger.KeyError, err)
		return err
	}
	if len(workflows) == 0 {
		return nil
	}

	var recordData map[string]any
	if err := json.Unmarshal(data, &recordData); err != nil {
		s.logger.ErrorContext(ctx, "Failed to unmarshal record data for workflow execution", logger.KeyID, recordID, logger.KeyError, err)
		return err
	}

	for _, wf := range workflows {
		if err := s.executeOne(ctx, wf, recordID, recordData); err != nil {
			s.logger.ErrorContext(ctx, "Workflow execution failed", logger.KeyID, wf.ID, "record_id", recordID, logger.KeyError, err)
			// TODO: do we fail rest or continue other workflows? insert already succeeded
		}
	}
	return nil
}

func (s *Service) executeOne(ctx context.Context, wf *workflowDefinition, recordID string, data map[string]any) error {
	var def definition
	if err := json.Unmarshal(wf.Definition, &def); err != nil {
		s.logger.ErrorContext(ctx, "Failed to unmarshal workflow definition", logger.KeyID, wf.ID, logger.KeyError, err)
		return err
	}

	if !def.Criteria.Match(data) {
		s.logger.InfoContext(ctx, "Workflow criteria not met", logger.KeyID, wf.ID, "record_id", recordID)
		return nil
	}

	s.logger.InfoContext(ctx, "Workflow criteria met, executing actions", logger.KeyID, wf.ID, "record_id", recordID, logger.KeyCount, len(def.Actions))
	for i, act := range def.Actions {
		if err := s.executeAction(ctx, wf.ID, recordID, act); err != nil {
			s.logger.ErrorContext(ctx, "Action execution failed", logger.KeyID, wf.ID, "record_id", recordID, "action_index", i, "action_type", act.Type, logger.KeyError, err)
			return err
		}
	}
	return nil
}

func (s *Service) executeAction(ctx context.Context, workflowID, recordID string, act action.Action) error {
	switch act.Type {
	case action.TypeCreateRecord, action.TypeUpdateRecord, action.TypeUpsertRecord, action.TypeTriggerPipeline:
		// Action side-effects (record writes, pipelines) will be wired next.
		s.logger.InfoContext(ctx, "Action matched for execution", logger.KeyID, workflowID, "record_id", recordID, "action_type", act.Type)
		return nil
	default:
		s.logger.WarnContext(ctx, "Unknown action type", logger.KeyID, workflowID, "action_type", act.Type)
		return nil
	}
}
