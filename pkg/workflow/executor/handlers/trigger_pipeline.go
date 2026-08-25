package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/resolver"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

type PipelineEnqueuer interface {
	Enqueue(ctx context.Context, req pipeline.EnqueueRequest) (string, error)
}

type TriggerPipeline struct {
	logger   *logger.Logger
	enqueuer PipelineEnqueuer
}

func NewTriggerPipeline(logger *logger.Logger, enqueuer PipelineEnqueuer) *TriggerPipeline {
	return &TriggerPipeline{logger: logger, enqueuer: enqueuer}
}

func (h *TriggerPipeline) Type() action.Type {
	return action.TypeTriggerPipeline
}

func (h *TriggerPipeline) Execute(ctx context.Context, execCtx executor.ExecutionContext, act action.Action) (json.RawMessage, error) {
	c, ok := act.Context.(action.TriggerPipelineContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type %T for TRIGGER_PIPELINE", act.Context)
	}
	if c.Pipeline == "" {
		return nil, fmt.Errorf("TRIGGER_PIPELINE requires a pipeline")
	}

	input, err := resolver.Resolve(c.Input, execCtx.TriggerData)
	if err != nil {
		return nil, fmt.Errorf("resolving pipeline input: %w", err)
	}

	id, err := h.enqueuer.Enqueue(ctx, pipeline.EnqueueRequest{
		PipelineSlug:       c.Pipeline,
		NetworkID:          execCtx.NetworkID,
		OrganizationID:     execCtx.OrganizationID,
		OrganizationUserID: execCtx.OrganizationUserID,
		Input:              input,
		DedupeKey:          execCtx.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}

	h.logger.InfoContext(ctx, "Enqueued pipeline from workflow",
		logger.KeyID, execCtx.WorkflowID,
		"pipeline", c.Pipeline,
		"pipeline_id", id,
		"record_id", execCtx.TriggerRecordID,
	)

	return json.Marshal(map[string]any{
		"id":    id,
		"input": input,
	})
}
