// Package handlers implements one executor.Handler per action type. Handlers
// perform side effects only; orchestration, journaling, and retry semantics
// belong to the executor.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/record"
	"github.com/lutia-io/huma/pkg/resolver"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

// RecordService is the narrow record surface handlers depend on, satisfied by
// *record.Service.
type RecordService interface {
	Create(ctx context.Context, params record.CreateParams) (string, error)
	Get(ctx context.Context, recordID string) (*record.Record, bool, error)
	PatchData(ctx context.Context, rec *record.Record, data json.RawMessage) error
}

func trigger(execCtx executor.ExecutionContext) resolver.Trigger {
	return resolver.Trigger{ID: execCtx.TriggerRecordID, Data: execCtx.TriggerData}
}

func resolveRecordID(execCtx executor.ExecutionContext, raw string) (string, error) {
	v, err := resolver.ResolveOne(raw, trigger(execCtx))
	if err != nil {
		return "", fmt.Errorf("resolving recordId: %w", err)
	}
	if v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("recordId resolved to %T, want string", v)
	}
	return s, nil
}

// createRecord is the shared create path for CREATE_RECORD and the
// insert branch of UPSERT_RECORD. Schema validation happens inside
// record.Service.Create.
func createRecord(ctx context.Context, records RecordService, execCtx executor.ExecutionContext, schemaID string, data map[string]any) (json.RawMessage, error) {
	resolved, err := resolver.Resolve(data, trigger(execCtx))
	if err != nil {
		return nil, fmt.Errorf("resolving action data: %w", err)
	}
	raw, err := json.Marshal(resolved)
	if err != nil {
		return nil, fmt.Errorf("marshaling action data: %w", err)
	}

	id, err := records.Create(ctx, record.CreateParams{
		SchemaID:           schemaID,
		OrganizationID:     execCtx.OrganizationID,
		OrganizationUserID: execCtx.OrganizationUserID,
		NetworkID:          execCtx.NetworkID,
		Data:               raw,
		IdempotencyKey:     execCtx.IdempotencyKey,
	})
	if err != nil {
		return nil, fmt.Errorf("creating record: %w", err)
	}
	return json.Marshal(map[string]string{"id": id})
}

// updateRecord is the shared merge-update path for UPDATE_RECORD and the
// update branch of UPSERT_RECORD. It shallow-merges the resolved action data
// over the existing record data, validates the merged document against the
// record's schema, and writes the full document (set-to-value, idempotent).
func updateRecord(ctx context.Context, records RecordService, execCtx executor.ExecutionContext, existing *record.Record, data map[string]any) (json.RawMessage, error) {
	resolved, err := resolver.Resolve(data, trigger(execCtx))
	if err != nil {
		return nil, fmt.Errorf("resolving action data: %w", err)
	}

	var merged map[string]any
	if err := json.Unmarshal(existing.Data, &merged); err != nil {
		return nil, fmt.Errorf("unmarshaling existing record data: %w", err)
	}
	for k, v := range resolved {
		merged[k] = v
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged record data: %w", err)
	}

	if err := records.PatchData(ctx, existing, raw); err != nil {
		return nil, fmt.Errorf("updating record %q: %w", existing.ID, err)
	}
	return json.Marshal(map[string]string{"id": existing.ID})
}
