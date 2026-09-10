package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/resolver"
)

func parseDefinition[T any](n pipeline.SnapshotNode, t node.Type) (T, error) {
	var zero T
	raw, err := json.Marshal(n.Definition)
	if err != nil {
		return zero, err
	}
	def, err := node.ParseDefinition(t, raw)
	if err != nil {
		return zero, err
	}
	typed, ok := def.(T)
	if !ok {
		return zero, fmt.Errorf("invalid %s definition type %T", t, def)
	}
	return typed, nil
}

func resolveString(s string, input map[string]any) (string, error) {
	if s == "" {
		return "", nil
	}
	v, err := resolver.ResolveString(s, input)
	if err != nil {
		return "", err
	}
	return stringifyResolved(v), nil
}

func stringifyResolved(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
