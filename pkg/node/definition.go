package node

import (
	"encoding/json"
	"fmt"
)

type Type string

const (
	TypeNoop Type = "NOOP"
)

type NoopContext struct {
	Message string `json:"message"`
}

func parseDefinition(t Type, data json.RawMessage) (any, error) {
	switch t {
	case TypeNoop:
		var ctx NoopContext
		if err := json.Unmarshal(data, &ctx); err != nil {
			return nil, fmt.Errorf("invalid NOOP definition: %w", err)
		}
		return ctx, nil
	default:
		return nil, fmt.Errorf("unknown node type %q", t)
	}
}
