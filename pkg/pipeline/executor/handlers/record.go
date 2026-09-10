package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/pipeline/executor"
	"github.com/lutia-io/huma/pkg/record"
	"github.com/lutia-io/huma/pkg/resolver"
)

const (
	recordListPageSize = 100
	recordListMax      = 10000
)

type RecordStore interface {
	Create(ctx context.Context, params record.CreateParams) (string, error)
	Get(ctx context.Context, recordID string) (*record.Record, bool, error)
	PatchData(ctx context.Context, rec *record.Record, data json.RawMessage) error
	ListForOrg(ctx context.Context, params record.ListForOrgParams) ([]*record.Record, int, error)
}

// SystemUsers looks up the hidden system organization user that owns records
// created by RECORD nodes.
type SystemUsers interface {
	SystemUserID(ctx context.Context, organizationID, networkID string) (string, error)
}

type Record struct {
	records     RecordStore
	systemUsers SystemUsers
}

func NewRecord(records RecordStore, systemUsers SystemUsers) *Record {
	return &Record{records: records, systemUsers: systemUsers}
}

func (h *Record) Type() node.Type {
	return node.TypeRecord
}

func (h *Record) Execute(ctx context.Context, execCtx executor.ExecutionContext, n pipeline.SnapshotNode) (executor.Result, error) {
	if h.records == nil {
		return executor.Result{}, fmt.Errorf("RECORD node requires a record service")
	}
	def, err := parseDefinition[node.RecordContext](n, node.TypeRecord)
	if err != nil {
		return executor.Result{}, err
	}
	switch def.Operation {
	case node.RecordOpList:
		return h.list(ctx, execCtx, def)
	case node.RecordOpGet:
		return h.get(ctx, execCtx, def)
	case node.RecordOpCreate:
		return h.create(ctx, execCtx, def)
	case node.RecordOpUpdate:
		return h.update(ctx, execCtx, def)
	case node.RecordOpUpsert:
		return h.upsert(ctx, execCtx, def)
	default:
		return executor.Result{}, fmt.Errorf("unsupported RECORD operation %q", def.Operation)
	}
}

func (h *Record) list(ctx context.Context, execCtx executor.ExecutionContext, def node.RecordContext) (executor.Result, error) {
	schemaID, err := resolveString(def.SchemaID, execCtx.Input)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving RECORD schemaId: %w", err)
	}
	if schemaID == "" {
		return executor.Result{}, fmt.Errorf("RECORD LIST requires a schemaId")
	}

	filters := make([]record.ListFieldFilter, 0, len(def.Filters))
	for _, filter := range def.Filters {
		field, fieldErr := resolveString(filter.Field, execCtx.Input)
		if fieldErr != nil {
			return executor.Result{}, fmt.Errorf("resolving RECORD filter field: %w", fieldErr)
		}
		if field == "" {
			continue
		}
		op, opErr := resolveString(filter.Op, execCtx.Input)
		if opErr != nil {
			return executor.Result{}, fmt.Errorf("resolving RECORD filter op: %w", opErr)
		}
		value, valueErr := resolveString(filter.Value, execCtx.Input)
		if valueErr != nil {
			return executor.Result{}, fmt.Errorf("resolving RECORD filter value: %w", valueErr)
		}
		filters = append(filters, record.ListFieldFilter{Name: field, Op: op, Value: value})
	}

	items := make([]*record.Record, 0)
	page := 1
	total := 0
	for {
		pageItems, pageTotal, listErr := h.records.ListForOrg(ctx, record.ListForOrgParams{
			SchemaID:       schemaID,
			NetworkID:      execCtx.NetworkID,
			OrganizationID: execCtx.OrganizationID,
			Fields:         filters,
			Page:           page,
			PageSize:       recordListPageSize,
		})
		if listErr != nil {
			return executor.Result{}, fmt.Errorf("listing records: %w", listErr)
		}
		total = pageTotal
		if total > recordListMax {
			return executor.Result{}, fmt.Errorf("RECORD LIST matched %d records, max is %d", total, recordListMax)
		}
		items = append(items, pageItems...)
		if len(items) >= total || len(pageItems) == 0 {
			break
		}
		page++
	}

	return executor.MarshalOutput(map[string]any{
		"records": items,
		"total":   total,
	})
}

func (h *Record) get(ctx context.Context, execCtx executor.ExecutionContext, def node.RecordContext) (executor.Result, error) {
	recordID, err := resolveString(def.RecordID, execCtx.Input)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving RECORD recordId: %w", err)
	}
	rec, err := h.loadInScope(ctx, execCtx, recordID)
	if err != nil {
		return executor.Result{}, err
	}
	return executor.MarshalOutput(map[string]any{"record": rec})
}

func (h *Record) create(ctx context.Context, execCtx executor.ExecutionContext, def node.RecordContext) (executor.Result, error) {
	schemaID, err := resolveString(def.SchemaID, execCtx.Input)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving RECORD schemaId: %w", err)
	}
	if schemaID == "" {
		return executor.Result{}, fmt.Errorf("RECORD CREATE requires a schemaId")
	}
	data, err := resolver.ResolveInput(def.Data, execCtx.Input)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving RECORD data: %w", err)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return executor.Result{}, err
	}
	orgUserID, err := resolveSystemUserID(ctx, h.systemUsers, execCtx)
	if err != nil {
		return executor.Result{}, err
	}
	id, err := h.records.Create(ctx, record.CreateParams{
		SchemaID:           schemaID,
		OrganizationID:     execCtx.OrganizationID,
		OrganizationUserID: orgUserID,
		NetworkID:          execCtx.NetworkID,
		Data:               raw,
		IdempotencyKey:     execCtx.IdempotencyKey,
	})
	if err != nil {
		return executor.Result{}, fmt.Errorf("creating record: %w", err)
	}
	return executor.MarshalOutput(map[string]any{"id": id})
}

func (h *Record) update(ctx context.Context, execCtx executor.ExecutionContext, def node.RecordContext) (executor.Result, error) {
	recordID, err := resolveString(def.RecordID, execCtx.Input)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving RECORD recordId: %w", err)
	}
	if recordID == "" {
		return executor.Result{}, fmt.Errorf("RECORD UPDATE requires a recordId")
	}
	existing, err := h.loadInScope(ctx, execCtx, recordID)
	if err != nil {
		return executor.Result{}, err
	}

	var existingData map[string]any
	if len(existing.Data) > 0 {
		if err := json.Unmarshal(existing.Data, &existingData); err != nil {
			return executor.Result{}, fmt.Errorf("unmarshaling existing record data: %w", err)
		}
	}
	if existingData == nil {
		existingData = map[string]any{}
	}

	resolved, err := resolver.ResolveInputWithTarget(def.Data, execCtx.Input, resolver.Target{
		ID:   existing.ID,
		Data: existingData,
	})
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving RECORD data: %w", err)
	}

	merged := make(map[string]any, len(existingData)+len(resolved))
	for k, v := range existingData {
		merged[k] = v
	}
	for k, v := range resolved {
		merged[k] = v
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return executor.Result{}, err
	}
	if err := h.records.PatchData(ctx, existing, raw); err != nil {
		return executor.Result{}, fmt.Errorf("updating record %q: %w", existing.ID, err)
	}
	return executor.MarshalOutput(map[string]any{"id": existing.ID})
}

func (h *Record) upsert(ctx context.Context, execCtx executor.ExecutionContext, def node.RecordContext) (executor.Result, error) {
	recordID, err := resolveString(def.RecordID, execCtx.Input)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving RECORD recordId: %w", err)
	}
	if recordID != "" {
		existing, found, getErr := h.records.Get(ctx, recordID)
		if getErr != nil {
			return executor.Result{}, fmt.Errorf("loading record %q: %w", recordID, getErr)
		}
		if found {
			if !inScope(existing, execCtx) {
				return executor.Result{}, fmt.Errorf("record %q not found", recordID)
			}
			return h.update(ctx, execCtx, def)
		}
	}
	return h.create(ctx, execCtx, def)
}

func (h *Record) loadInScope(ctx context.Context, execCtx executor.ExecutionContext, recordID string) (*record.Record, error) {
	if recordID == "" {
		return nil, fmt.Errorf("recordId is required")
	}
	rec, found, err := h.records.Get(ctx, recordID)
	if err != nil {
		return nil, fmt.Errorf("loading record %q: %w", recordID, err)
	}
	if !found || !inScope(rec, execCtx) {
		return nil, fmt.Errorf("record %q not found", recordID)
	}
	return rec, nil
}

func inScope(rec *record.Record, execCtx executor.ExecutionContext) bool {
	return rec != nil && rec.NetworkID == execCtx.NetworkID && rec.OrganizationID == execCtx.OrganizationID
}

func resolveSystemUserID(ctx context.Context, systemUsers SystemUsers, execCtx executor.ExecutionContext) (string, error) {
	if systemUsers == nil {
		return "", fmt.Errorf("system user lookup is required")
	}
	id, err := systemUsers.SystemUserID(ctx, execCtx.OrganizationID, execCtx.NetworkID)
	if err != nil {
		return "", fmt.Errorf("loading system organization user: %w", err)
	}
	if id == "" {
		return "", fmt.Errorf("system organization user not found")
	}
	return id, nil
}
