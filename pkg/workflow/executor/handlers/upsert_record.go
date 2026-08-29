package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

// UpsertRecord handles action.TypeUpsertRecord: update when recordId exists,
// otherwise create under schemaId.
type UpsertRecord struct {
	records RecordService
}

func NewUpsertRecord(records RecordService) *UpsertRecord {
	return &UpsertRecord{records: records}
}

func (h *UpsertRecord) Type() action.Type {
	return action.TypeUpsertRecord
}

// Execute updates the record when recordId is provided and exists; otherwise
// it creates a new record. The create branch reuses the action's idempotency
// key, so a replayed attempt converges on the same record.
func (h *UpsertRecord) Execute(ctx context.Context, execCtx executor.ExecutionContext, act action.Action) (json.RawMessage, error) {
	c, ok := act.Context.(action.UpsertRecordContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type %T for UPSERT_RECORD", act.Context)
	}

	recordID, err := resolveRecordID(execCtx, c.RecordID)
	if err != nil {
		return nil, err
	}
	if recordID != "" {
		existing, found, err := h.records.Get(ctx, recordID)
		if err != nil {
			return nil, fmt.Errorf("loading record %q: %w", recordID, err)
		}
		if found {
			return updateRecord(ctx, h.records, execCtx, existing, c.Data)
		}
	}

	if c.SchemaID == "" {
		return nil, fmt.Errorf("UPSERT_RECORD requires a schemaId to create a record")
	}
	return createRecord(ctx, h.records, execCtx, c.SchemaID, c.Data)
}
