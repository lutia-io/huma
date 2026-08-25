package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/pipeline"
)

type worker struct {
	id           string
	service      *Service
	pollInterval time.Duration
}

func workerID(prefix string, index int) string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return fmt.Sprintf("%s-%s-%d-%d", prefix, host, os.Getpid(), index)
}

func (w *worker) run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		claimed, err := w.service.pipelines.ClaimOne(ctx, w.id)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.service.logger.ErrorContext(ctx, "Failed to claim pipeline", "worker_id", w.id, logger.KeyError, err)
			w.sleep(ctx)
			continue
		}
		if claimed == nil {
			w.sleep(ctx)
			continue
		}
		w.execute(ctx, claimed)
	}
}

func (w *worker) sleep(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(w.pollInterval):
	}
}

func (w *worker) execute(ctx context.Context, p *Pipeline) {
	levels := p.Definition.Nodes
	w.service.logger.InfoContext(ctx, "Executing pipeline",
		logger.KeyID, p.ID,
		"pipeline_definition_id", p.PipelineDefinitionID,
		"current_level", p.CurrentLevel,
		"attempt", p.Attempts,
		logger.KeyCount, len(levels),
	)

	input := p.Input
	if p.CurrentLevel > 0 {
		// Rebuild input from the previous level's completed outputs so a
		// reclaim mid-pipeline still sees the same next-level payload.
		prev, err := w.service.pipelines.ListTerminalNodes(ctx, p.ID, p.CurrentLevel-1)
		if err != nil {
			w.service.logger.ErrorContext(ctx, "Failed to load previous level outputs", logger.KeyID, p.ID, logger.KeyError, err)
			return
		}
		outputs := make(map[int]json.RawMessage, len(prev))
		for _, n := range prev {
			if n.Status == NodeStatusCompleted {
				outputs[n.NodeIndex] = n.Output
			}
		}
		input = indexedOutput(outputs)
	}

	for level := p.CurrentLevel; level < len(levels); level++ {
		nodes := levels[level]
		terminals, err := w.service.pipelines.ListTerminalNodes(ctx, p.ID, level)
		if err != nil {
			w.service.logger.ErrorContext(ctx, "Failed to list terminal pipeline nodes", logger.KeyID, p.ID, "level", level, logger.KeyError, err)
			return
		}
		done := make(map[int]TerminalNode, len(terminals))
		for _, t := range terminals {
			done[t.NodeIndex] = t
		}

		outputs := make(map[int]json.RawMessage, len(nodes))
		var mu sync.Mutex
		levelFailed := false

		var wg sync.WaitGroup
		for i, n := range nodes {
			if t, ok := done[i]; ok {
				if t.Status == NodeStatusFailed {
					levelFailed = true
				}
				outputs[i] = t.Output
				continue
			}
			wg.Add(1)
			go func(i int, n pipeline.SnapshotNode) {
				defer wg.Done()
				output, execErr := w.executeNode(ctx, p, level, i, n, input)
				mu.Lock()
				defer mu.Unlock()
				if execErr != nil {
					levelFailed = true
					return
				}
				outputs[i] = output
			}(i, n)
		}
		wg.Wait()

		if levelFailed {
			w.service.logger.ErrorContext(ctx, "Pipeline level failed", logger.KeyID, p.ID, "level", level)
			if err := w.service.pipelines.Finish(ctx, p.ID, true, fmt.Sprintf("level %d failed", level)); err != nil {
				w.service.logger.ErrorContext(ctx, "Failed to finish pipeline", logger.KeyID, p.ID, logger.KeyError, err)
			}
			return
		}

		if err := w.service.pipelines.AdvanceLevel(ctx, p.ID, level+1); err != nil {
			w.service.logger.ErrorContext(ctx, "Failed to advance pipeline level", logger.KeyID, p.ID, logger.KeyError, err)
			return
		}
		input = indexedOutput(outputs)
	}

	if err := w.service.pipelines.Finish(ctx, p.ID, false, ""); err != nil {
		w.service.logger.ErrorContext(ctx, "Failed to finish pipeline", logger.KeyID, p.ID, logger.KeyError, err)
		return
	}
	w.service.logger.InfoContext(ctx, "Pipeline completed", logger.KeyID, p.ID)
}

func (w *worker) executeNode(ctx context.Context, p *Pipeline, level, index int, n pipeline.SnapshotNode, input map[string]any) (json.RawMessage, error) {
	execCtx := ExecutionContext{
		PipelineID:           p.ID,
		PipelineDefinitionID: p.PipelineDefinitionID,
		NetworkID:            p.NetworkID,
		OrganizationID:       p.OrganizationID,
		OrganizationUserID:   p.OrganizationUserID,
		LevelIndex:           level,
		NodeIndex:            index,
		Input:                input,
		IdempotencyKey:       fmt.Sprintf("%s:%d:%d", p.ID, level, index),
	}
	entry := PipelineNode{
		PipelineID:       p.ID,
		LevelIndex:       level,
		NodeIndex:        index,
		Attempt:          p.Attempts,
		NodeDefinitionID: n.ID,
		NodeSlug:         n.Slug,
		NodeType:         string(n.Type),
		Input:            nodeInputJSON(input),
		StartedAt:        time.Now().UTC(),
	}

	output, err := w.service.registry.Execute(ctx, execCtx, n)
	if err != nil {
		entry.Error = err.Error()
		w.service.logger.ErrorContext(ctx, "Pipeline node failed",
			logger.KeyID, p.ID,
			"level", level,
			"node_index", index,
			"node_type", n.Type,
			logger.KeyError, err,
		)
		if storeErr := w.service.pipelines.FailNode(ctx, entry); storeErr != nil {
			w.service.logger.ErrorContext(ctx, "Failed to persist pipeline node failure", logger.KeyID, p.ID, logger.KeyError, storeErr)
			return nil, storeErr
		}
		return nil, err
	}

	entry.Output = output
	if err := w.service.pipelines.CompleteNode(ctx, entry); err != nil {
		w.service.logger.ErrorContext(ctx, "Failed to persist pipeline node completion", logger.KeyID, p.ID, logger.KeyError, err)
		return nil, err
	}
	return output, nil
}
