package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
)

type ExecutionContext struct {
	PipelineID           string
	PipelineDefinitionID string
	NetworkID            string
	OrganizationID       string
	OrganizationUserID   string
	LevelIndex           int
	NodeIndex            int
	Input                map[string]any
	IdempotencyKey       string
}

type Handler interface {
	Type() node.Type
	Execute(ctx context.Context, execCtx ExecutionContext, n pipeline.SnapshotNode) (json.RawMessage, error)
}

type Registry struct {
	handlers map[node.Type]Handler
}

func NewRegistry(handlers ...Handler) *Registry {
	r := &Registry{handlers: make(map[node.Type]Handler, len(handlers))}
	for _, h := range handlers {
		r.handlers[h.Type()] = h
	}
	return r
}

func (r *Registry) Execute(ctx context.Context, execCtx ExecutionContext, n pipeline.SnapshotNode) (json.RawMessage, error) {
	h, ok := r.handlers[n.Type]
	if !ok {
		return nil, fmt.Errorf("node type %q is not implemented", n.Type)
	}
	return h.Execute(ctx, execCtx, n)
}
