package handlers

import (
	"context"
	"fmt"
	"reflect"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/pipeline/executor"
	"github.com/lutia-io/huma/pkg/resolver"
)

type ListMapper struct{}

func NewListMapper() *ListMapper {
	return &ListMapper{}
}

func (h *ListMapper) Type() node.Type {
	return node.TypeListMapper
}

func (h *ListMapper) Execute(_ context.Context, execCtx executor.ExecutionContext, n pipeline.SnapshotNode) (executor.Result, error) {
	def, err := parseDefinition[node.ListMapperContext](n, node.TypeListMapper)
	if err != nil {
		return executor.Result{}, err
	}

	from, err := resolver.ResolveString(def.From, execCtx.Input)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving LIST_MAPPER from: %w", err)
	}
		items, err := toSlice(from)
	if err != nil {
		return executor.Result{}, fmt.Errorf("LIST_MAPPER from: %w", err)
	}

	out := make([]any, 0, len(items))
	for i, item := range items {
		resolved, resolveErr := resolver.ResolveInputWith(def.Mapping, execCtx.Input, map[string]any{
			def.As: item,
		})
		if resolveErr != nil {
			return executor.Result{}, fmt.Errorf("resolving LIST_MAPPER mapping at index %d: %w", i, resolveErr)
		}
		out = append(out, resolved)
	}
	return executor.MarshalOutput(map[string]any{"items": out})
}

func toSlice(v any) ([]any, error) {
	if v == nil {
		return []any{}, nil
	}
	switch t := v.(type) {
	case []any:
		return t, nil
	case []map[string]any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = item
		}
		return out, nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, fmt.Errorf("expected a list, got %T", v)
	}
	out := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out[i] = rv.Index(i).Interface()
	}
	return out, nil
}
