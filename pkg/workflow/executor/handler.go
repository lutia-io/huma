package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/action"
)

// Handler executes one action type.
//
// Contract: Execute must be idempotent with respect to
// execCtx.IdempotencyKey. A worker can crash after the side effect but before
// the journal commit, in which case the action is re-executed with the same
// key; the handler must resolve to the original result instead of duplicating
// the effect.
type Handler interface {
	Type() action.Type
	// Execute runs the action and returns a JSON-serializable output for the
	// workflow_actions journal (e.g. {"recordId": "..."}).
	Execute(ctx context.Context, execCtx ExecutionContext, act action.Action) (json.RawMessage, error)
}

// Registry dispatches actions to their registered handlers. Adding a new
// action type is registering another Handler — the worker loop does not change.
type Registry struct {
	handlers map[action.Type]Handler
}

// NewRegistry builds a registry from the given handlers. If two handlers
// share a Type, the later one wins.
func NewRegistry(handlers ...Handler) *Registry {
	r := &Registry{handlers: make(map[action.Type]Handler, len(handlers))}
	for _, h := range handlers {
		r.handlers[h.Type()] = h
	}
	return r
}

// Execute looks up the handler for act.Type and runs it. Unknown types are
// a permanent failure for that action (journaled by the worker).
func (r *Registry) Execute(ctx context.Context, execCtx ExecutionContext, act action.Action) (json.RawMessage, error) {
	h, ok := r.handlers[act.Type]
	if !ok {
		return nil, fmt.Errorf("no handler registered for action type %q", act.Type)
	}
	return h.Execute(ctx, execCtx, act)
}
