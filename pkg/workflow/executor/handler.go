package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/workflow"
)

type ExecutionContext struct {
	WorkflowID           string
	WorkflowDefinitionID string
	NetworkID            string
	OrganizationID       string
	OrganizationUserID   string
	TriggerRecordID      string
	TriggerData          map[string]any
	LevelIndex           int
	NodeIndex            int
	Input                map[string]any
	IdempotencyKey       string
}

type FileRef struct {
	FileID      string `json:"fileId"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

type Payload struct {
	Files []FileRef `json:"files,omitempty"`
}

type Result struct {
	Output  json.RawMessage
	Payload Payload
}

func MarshalOutput(v any) (Result, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return Result{}, err
	}
	return Result{Output: raw}, nil
}

type Handler interface {
	Type() node.Type
	Execute(ctx context.Context, execCtx ExecutionContext, n workflow.SnapshotNode) (Result, error)
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

func (r *Registry) Execute(ctx context.Context, execCtx ExecutionContext, n workflow.SnapshotNode) (Result, error) {
	h, ok := r.handlers[n.Type]
	if !ok {
		return Result{}, fmt.Errorf("node type %q is not implemented", n.Type)
	}
	return h.Execute(ctx, execCtx, n)
}
