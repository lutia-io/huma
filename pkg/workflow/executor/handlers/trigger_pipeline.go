package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/resolver"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

// TriggerPipeline handles action.TypeTriggerPipeline. Today it is a stub that
// logs and returns; the durable handoff to the pipeline engine will be a
// published message keyed by the action idempotency key.
type TriggerPipeline struct {
	logger *logger.Logger
}

func NewTriggerPipeline(logger *logger.Logger) *TriggerPipeline {
	return &TriggerPipeline{logger: logger}
}

func (h *TriggerPipeline) Type() action.Type {
	return action.TypeTriggerPipeline
}

// Execute is a stub: it logs the trigger and completes. It will eventually
// publish to a pipelines subject using execCtx.IdempotencyKey as the message
// ID so the broker dedupes replayed attempts; the pipeline engine consumes
// from there. The workflow's responsibility ends at the publish.
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

	h.logger.InfoContext(ctx, "TRIGGER_PIPELINE stub: pipeline trigger not yet implemented",
		logger.KeyID, execCtx.WorkflowID,
		"pipeline", c.Pipeline,
		"record_id", execCtx.TriggerRecordID,
		"idempotency_key", execCtx.IdempotencyKey,
	)

	return json.Marshal(map[string]any{
		"id":    "some-pipeline-id",
		"input": input,
	})
}
