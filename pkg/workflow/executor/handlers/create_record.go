package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

// CreateRecord handles action.TypeCreateRecord: create a record in the
// schema from the action context, using the trigger's organization identity.
type CreateRecord struct {
	records RecordService
}

func NewCreateRecord(records RecordService) *CreateRecord {
	return &CreateRecord{records: records}
}

func (h *CreateRecord) Type() action.Type {
	return action.TypeCreateRecord
}

func (h *CreateRecord) Execute(ctx context.Context, execCtx executor.ExecutionContext, act action.Action) (json.RawMessage, error) {
	c, ok := act.Context.(action.CreateRecordContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type %T for CREATE_RECORD", act.Context)
	}
	if c.SchemaID == "" {
		return nil, fmt.Errorf("CREATE_RECORD requires a schemaId")
	}
	return createRecord(ctx, h.records, execCtx, c.SchemaID, c.Data)
}
