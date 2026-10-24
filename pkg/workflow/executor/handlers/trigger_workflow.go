package handlers

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/uuid"
	"github.com/lutia-io/huma/pkg/workflow"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

type WorkflowEnqueuer interface {
	Enqueue(ctx context.Context, req workflow.EnqueueRequest) (string, error)
}

type TriggerWorkflow struct {
	logger    *logger.Logger
	workflows WorkflowEnqueuer
}

func NewTriggerWorkflow(log *logger.Logger, workflows WorkflowEnqueuer) *TriggerWorkflow {
	return &TriggerWorkflow{logger: log, workflows: workflows}
}

func (h *TriggerWorkflow) Type() node.Type {
	return node.TypeTriggerWorkflow
}

func (h *TriggerWorkflow) Execute(ctx context.Context, execCtx executor.ExecutionContext, n workflow.SnapshotNode) (executor.Result, error) {
	def, err := parseDefinition[node.TriggerWorkflowContext](n, node.TypeTriggerWorkflow)
	if err != nil {
		return executor.Result{}, err
	}

	// Same .Record / .Input contract as every other node.
	input, err := resolveInput(def.Input, execCtx)
	if err != nil {
		return executor.Result{}, err
	}

	target := strings.TrimSpace(def.Workflow)
	req := workflow.EnqueueRequest{
		NetworkID:          execCtx.NetworkID,
		OrganizationID:     execCtx.OrganizationID,
		OrganizationUserID: execCtx.OrganizationUserID,
		Input:              input,
		DedupeKey:          execCtx.IdempotencyKey,
	}
	if uuid.Valid(target) {
		req.WorkflowDefinitionID = target
	} else {
		req.WorkflowSlug = target
	}

	id, err := h.workflows.Enqueue(ctx, req)
	if err != nil {
		return executor.Result{}, err
	}

	out, err := json.Marshal(map[string]any{"id": id, "input": input})
	if err != nil {
		return executor.Result{}, err
	}
	return executor.Result{Output: out}, nil
}
