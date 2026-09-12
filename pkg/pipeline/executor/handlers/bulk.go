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

const bulkMax = 10000

type Bulk struct {
	records     RecordStore
	systemUsers SystemUsers
}

func NewBulk(records RecordStore, systemUsers SystemUsers) *Bulk {
	return &Bulk{records: records, systemUsers: systemUsers}
}

func (h *Bulk) Type() node.Type {
	return node.TypeBulk
}

func (h *Bulk) Execute(ctx context.Context, execCtx executor.ExecutionContext, n pipeline.SnapshotNode) (executor.Result, error) {
	if h.records == nil {
		return executor.Result{}, fmt.Errorf("BULK node requires a record service")
	}
	def, err := parseDefinition[node.BulkContext](n, node.TypeBulk)
	if err != nil {
		return executor.Result{}, err
	}

	orgUserID, err := resolveSystemUserID(ctx, h.systemUsers, execCtx)
	if err != nil {
		return executor.Result{}, err
	}

	created := make([]map[string]any, 0, len(def.Records))
	total := 0
	for i, group := range def.Records {
		schemaID, schemaErr := resolveString(group.SchemaID, execCtx.Input)
		if schemaErr != nil {
			return executor.Result{}, fmt.Errorf("resolving BULK records[%d].schemaId: %w", i, schemaErr)
		}
		if schemaID == "" {
			return executor.Result{}, fmt.Errorf("BULK records[%d] requires a schemaId", i)
		}

		from, fromErr := resolver.ResolveString(group.From, execCtx.Input)
		if fromErr != nil {
			return executor.Result{}, fmt.Errorf("resolving BULK records[%d].from: %w", i, fromErr)
		}
		items, sliceErr := toSlice(from)
		if sliceErr != nil {
			return executor.Result{}, fmt.Errorf("BULK records[%d].from: %w", i, sliceErr)
		}
		if len(items) > bulkMax {
			return executor.Result{}, fmt.Errorf("BULK records[%d] has %d items, max is %d", i, len(items), bulkMax)
		}

		ids := make([]string, 0, len(items))
		createdCount := 0
		updatedCount := 0
		for j, item := range items {
			extras := map[string]any{group.As: item}
			id, updated, writeErr := h.writeItem(ctx, execCtx, schemaID, orgUserID, def.Operation, group, extras, i, j)
			if writeErr != nil {
				return executor.Result{}, writeErr
			}
			ids = append(ids, id)
			if updated {
				updatedCount++
			} else {
				createdCount++
			}
		}

		total += len(ids)
		created = append(created, map[string]any{
			"schemaId": schemaID,
			"ids":      ids,
			"created":  createdCount,
			"updated":  updatedCount,
			"total":    len(ids),
		})
	}

	return executor.MarshalOutput(map[string]any{
		"created": created,
		"total":   total,
	})
}

func (h *Bulk) writeItem(
	ctx context.Context,
	execCtx executor.ExecutionContext,
	schemaID, orgUserID, operation string,
	group node.BulkRecord,
	extras map[string]any,
	groupIndex, itemIndex int,
) (string, bool, error) {
	if operation == node.BulkOpUpsert {
		recordID, err := resolveStringWith(group.RecordID, execCtx.Input, extras)
		if err != nil {
			return "", false, fmt.Errorf("resolving BULK records[%d] recordId at index %d: %w", groupIndex, itemIndex, err)
		}
		if recordID != "" {
			existing, found, getErr := h.records.Get(ctx, recordID)
			if getErr != nil {
				return "", false, fmt.Errorf("loading record %q: %w", recordID, getErr)
			}
			if found {
				if !inScope(existing, execCtx) {
					return "", false, fmt.Errorf("record %q not found", recordID)
				}
				id, updateErr := h.updateItem(ctx, execCtx, existing, group.Data, extras)
				if updateErr != nil {
					return "", false, fmt.Errorf("updating BULK records[%d] at index %d: %w", groupIndex, itemIndex, updateErr)
				}
				return id, true, nil
			}
		}
	}

	data, err := resolver.ResolveInputWith(group.Data, execCtx.Input, extras)
	if err != nil {
		return "", false, fmt.Errorf("resolving BULK records[%d] data at index %d: %w", groupIndex, itemIndex, err)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return "", false, err
	}
	id, err := h.records.Create(ctx, record.CreateParams{
		SchemaID:           schemaID,
		OrganizationID:     execCtx.OrganizationID,
		OrganizationUserID: orgUserID,
		NetworkID:          execCtx.NetworkID,
		Data:               raw,
		IdempotencyKey:     fmt.Sprintf("%s:%d:%d", execCtx.IdempotencyKey, groupIndex, itemIndex),
	})
	if err != nil {
		return "", false, fmt.Errorf("creating BULK records[%d] at index %d: %w", groupIndex, itemIndex, err)
	}
	return id, false, nil
}

func (h *Bulk) updateItem(
	ctx context.Context,
	execCtx executor.ExecutionContext,
	existing *record.Record,
	data map[string]any,
	extras map[string]any,
) (string, error) {
	existingData, err := existingDataMap(existing.Data)
	if err != nil {
		return "", err
	}
	resolved, err := resolver.ResolveInputWithAndTarget(data, execCtx.Input, extras, resolver.Target{
		ID:   existing.ID,
		Data: existingData,
	})
	if err != nil {
		return "", fmt.Errorf("resolving BULK data: %w", err)
	}
	raw, err := mergeRecordData(existingData, resolved)
	if err != nil {
		return "", err
	}
	if err := h.records.PatchData(ctx, existing, raw); err != nil {
		return "", err
	}
	return existing.ID, nil
}
