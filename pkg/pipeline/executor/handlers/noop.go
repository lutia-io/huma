package handlers

import (
	"context"
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

func (h *Noop) Execute(_ context.Context, execCtx executor.ExecutionContext, n pipeline.SnapshotNode) (executor.Result, error) {
	ctx, err := parseDefinition[node.NoopContext](n, node.TypeNoop)
	if err != nil {
		return executor.Result{}, err
	}
	message := ctx.Message
	if message != "" {
		resolved, resolveErr := resolver.ResolveString(message, execCtx.Input)
		if resolveErr != nil {
			return executor.Result{}, fmt.Errorf("resolving NOOP message: %w", resolveErr)
		}
		if s, ok := resolved.(string); ok {
			message = s
		}
	}
	return executor.MarshalOutput(map[string]any{"message": message})
}
