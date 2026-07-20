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

type definition struct {
	Context any `json:"context"`
}

func (d *definition) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type    Type            `json:"type"`
		Context json.RawMessage `json:"context"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw.Type {
	case TypeNoop:
		var ctx NoopContext
		if err := json.Unmarshal(raw.Context, &ctx); err != nil {
			return fmt.Errorf("invalid NOOP context: %w", err)
		}
		d.Context = ctx
	default:
		return fmt.Errorf("unknown node type %q", raw.Type)
	}

	return nil
}
