package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

// UpdateRecord handles action.TypeUpdateRecord: load the target record,
// shallow-merge action data onto it, validate, and write the full document.
type UpdateRecord struct {
	records RecordService
	schemas SchemaService
}

func NewUpdateRecord(records RecordService, schemas SchemaService) *UpdateRecord {
	return &UpdateRecord{records: records, schemas: schemas}
}

func (h *UpdateRecord) Type() action.Type {
	return action.TypeUpdateRecord
}

func (h *UpdateRecord) Execute(ctx context.Context, execCtx executor.ExecutionContext, act action.Action) (json.RawMessage, error) {
	c, ok := act.Context.(action.UpdateRecordContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type %T for UPDATE_RECORD", act.Context)
	}
	if c.RecordID == "" {
		return nil, fmt.Errorf("UPDATE_RECORD requires a recordId")
	}

	existing, found, err := h.records.Get(ctx, c.RecordID)
	if err != nil {
		return nil, fmt.Errorf("loading record %q: %w", c.RecordID, err)
	}
	if !found {
		return nil, fmt.Errorf("record %q not found", c.RecordID)
	}
	return updateRecord(ctx, h.records, h.schemas, execCtx, existing, c.Data)
}
