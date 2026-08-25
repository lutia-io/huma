package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/pipeline/executor"
	"github.com/lutia-io/huma/pkg/resolver"
)

type Noop struct{}

func NewNoop() *Noop {
	return &Noop{}
}

func (h *Noop) Type() node.Type {
	return node.TypeNoop
}

func (h *Noop) Execute(_ context.Context, execCtx executor.ExecutionContext, n pipeline.SnapshotNode) (json.RawMessage, error) {
	raw, err := json.Marshal(n.Definition)
	if err != nil {
		return nil, err
	}
	def, err := node.ParseDefinition(node.TypeNoop, raw)
	if err != nil {
		return nil, err
	}
	ctx, ok := def.(node.NoopContext)
	if !ok {
		return nil, fmt.Errorf("invalid NOOP definition type %T", def)
	}
	message := ctx.Message
	if message != "" {
		resolved, err := resolver.ResolveString(message, execCtx.Input)
		if err != nil {
			return nil, fmt.Errorf("resolving NOOP message: %w", err)
		}
		if s, ok := resolved.(string); ok {
			message = s
		}
	}
	return json.Marshal(map[string]any{"message": message})
}
