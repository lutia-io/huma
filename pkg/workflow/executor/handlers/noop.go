package handlers

import (
	"context"
	"fmt"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/workflow"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

type Noop struct{}

func NewNoop() *Noop {
	return &Noop{}
}

func (h *Noop) Type() node.Type {
	return node.TypeNoop
}

func (h *Noop) Execute(_ context.Context, execCtx executor.ExecutionContext, n workflow.SnapshotNode) (executor.Result, error) {
	ctx, err := parseDefinition[node.NoopContext](n, node.TypeNoop)
	if err != nil {
		return executor.Result{}, err
	}
	message := ctx.Message
	if message != "" {
		resolved, resolveErr := resolveAny(message, execCtx)
		if resolveErr != nil {
			return executor.Result{}, fmt.Errorf("resolving NOOP message: %w", resolveErr)
		}
		message = stringifyResolved(resolved)
	}
	return executor.MarshalOutput(map[string]any{"message": message})
}
