package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/resolver"
	"github.com/lutia-io/huma/pkg/workflow"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

func parseDefinition[T any](n workflow.SnapshotNode, t node.Type) (T, error) {
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

// withTriggerRecord always exposes .Record for pipeline templates. Enqueue still
// flattens trigger fields onto .Input; .Record is the structured trigger row
// ({ id, data }) so every node type shares one contract.
func withTriggerRecord(execCtx executor.ExecutionContext, extras map[string]any) map[string]any {
	data := execCtx.TriggerData
	if data == nil {
		data = map[string]any{}
	}
	out := map[string]any{
		"Record": map[string]any{
			"id":   execCtx.TriggerRecordID,
			"data": data,
		},
	}
	for k, v := range extras {
		out[k] = v
	}
	return out
}

func resolveString(s string, execCtx executor.ExecutionContext) (string, error) {
	return resolveStringWith(s, execCtx, nil)
}

func resolveStringWith(s string, execCtx executor.ExecutionContext, extras map[string]any) (string, error) {
	if s == "" {
		return "", nil
	}
	v, err := resolver.ResolveStringWith(s, execCtx.Input, withTriggerRecord(execCtx, extras))
	if err != nil {
		return "", err
	}
	return stringifyResolved(v), nil
}

func resolveInput(data map[string]any, execCtx executor.ExecutionContext) (map[string]any, error) {
	return resolver.ResolveInputWith(data, execCtx.Input, withTriggerRecord(execCtx, nil))
}

func resolveInputWith(data map[string]any, execCtx executor.ExecutionContext, extras map[string]any) (map[string]any, error) {
	return resolver.ResolveInputWith(data, execCtx.Input, withTriggerRecord(execCtx, extras))
}

func resolveInputWithTarget(data map[string]any, execCtx executor.ExecutionContext, target resolver.Target) (map[string]any, error) {
	return resolver.ResolveInputWithAndTarget(data, execCtx.Input, withTriggerRecord(execCtx, nil), target)
}

func resolveInputWithAndTarget(data map[string]any, execCtx executor.ExecutionContext, extras map[string]any, target resolver.Target) (map[string]any, error) {
	return resolver.ResolveInputWithAndTarget(data, execCtx.Input, withTriggerRecord(execCtx, extras), target)
}

func resolveAny(s string, execCtx executor.ExecutionContext) (any, error) {
	if s == "" {
		return "", nil
	}
	return resolver.ResolveStringWith(s, execCtx.Input, withTriggerRecord(execCtx, nil))
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

// contentString turns a resolved FILE content value into bytes for storage.
// Strings stay as-is; other JSON values are marshaled so writers can pass
// objects/arrays via a pure template path.
func contentString(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", nil
	case string:
		return t, nil
	case []byte:
		return string(t), nil
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
}
