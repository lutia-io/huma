package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/workflow"
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
		claimed, err := w.service.workflows.ClaimOne(ctx, w.id)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.service.logger.ErrorContext(ctx, "Failed to claim workflow", "worker_id", w.id, logger.KeyError, err)
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

func (w *worker) execute(ctx context.Context, wf *Workflow) {
	levels := wf.Definition.Nodes
	w.service.logger.InfoContext(ctx, "Executing workflow",
		logger.KeyID, wf.ID,
		"workflow_definition_id", wf.WorkflowDefinitionID,
		"current_level", wf.CurrentLevel,
		"attempt", wf.Attempts,
		logger.KeyCount, len(levels),
	)

	input := wf.Input
	if input == nil {
		input = map[string]any{}
	}
	if wf.CurrentLevel > 0 {
		prev, err := w.service.workflows.ListTerminalNodes(ctx, wf.ID, wf.CurrentLevel-1)
		if err != nil {
			w.service.logger.ErrorContext(ctx, "Failed to load previous level outputs", logger.KeyID, wf.ID, logger.KeyError, err)
			return
		}
		outputs := make(map[int]json.RawMessage, len(prev))
		for _, n := range prev {
			if n.Status == NodeStatusCompleted {
				outputs[n.NodeIndex] = n.Output
			}
		}
		input = MergeLevelInput(wf.Input, IndexedOutput(outputs))
	}

	for level := wf.CurrentLevel; level < len(levels); level++ {
		nodes := levels[level]
		terminals, err := w.service.workflows.ListTerminalNodes(ctx, wf.ID, level)
		if err != nil {
			w.service.logger.ErrorContext(ctx, "Failed to list terminal workflow nodes", logger.KeyID, wf.ID, "level", level, logger.KeyError, err)
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
			if t, ok := done[i]; ok && t.Status == NodeStatusCompleted {
				outputs[i] = t.Output
				continue
			}
			wg.Add(1)
			go func(i int, n workflow.SnapshotNode) {
				defer wg.Done()
				output, execErr := w.executeNode(ctx, wf, level, i, n, input)
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
			w.service.logger.ErrorContext(ctx, "Workflow level failed", logger.KeyID, wf.ID, "level", level)
			if err := w.service.workflows.Finish(ctx, wf.ID, true, fmt.Sprintf("level %d failed", level)); err != nil {
				w.service.logger.ErrorContext(ctx, "Failed to finish workflow", logger.KeyID, wf.ID, logger.KeyError, err)
			}
			return
		}

		if err := w.service.workflows.AdvanceLevel(ctx, wf.ID, level+1); err != nil {
			w.service.logger.ErrorContext(ctx, "Failed to advance workflow level", logger.KeyID, wf.ID, logger.KeyError, err)
			return
		}
		input = MergeLevelInput(wf.Input, IndexedOutput(outputs))
	}

	if err := w.service.workflows.Finish(ctx, wf.ID, false, ""); err != nil {
		w.service.logger.ErrorContext(ctx, "Failed to finish workflow", logger.KeyID, wf.ID, logger.KeyError, err)
		return
	}
	w.service.logger.InfoContext(ctx, "Workflow completed", logger.KeyID, wf.ID)
}

func (w *worker) executeNode(ctx context.Context, wf *Workflow, level, index int, n workflow.SnapshotNode, input map[string]any) (json.RawMessage, error) {
	execCtx := ExecutionContext{
		WorkflowID:           wf.ID,
		WorkflowDefinitionID: wf.WorkflowDefinitionID,
		NetworkID:            wf.NetworkID,
		TriggerRecordID:      wf.RecordID,
		TriggerData:          wf.Data,
		OrganizationID:       wf.OrganizationID,
		OrganizationUserID:   wf.OrganizationUserID,
		LevelIndex:           level,
		NodeIndex:            index,
		Input:                input,
		IdempotencyKey:       fmt.Sprintf("%s:%d:%d", wf.ID, level, index),
	}
	entry := WorkflowNode{
		WorkflowID: wf.ID,
		LevelIndex: level,
		NodeIndex:  index,
		Attempt:    wf.Attempts,
		NodeType:   string(n.Type),
		Input:      NodeInputJSON(input),
		StartedAt:  time.Now().UTC(),
	}

	result, err := w.service.registry.Execute(ctx, execCtx, n)
	if err != nil {
		entry.Error = err.Error()
		w.service.logger.ErrorContext(ctx, "Workflow node failed",
			logger.KeyID, wf.ID,
			"level", level,
			"node_index", index,
			"node_type", n.Type,
			logger.KeyError, err,
		)
		if storeErr := w.service.workflows.FailNode(ctx, entry); storeErr != nil {
			w.service.logger.ErrorContext(ctx, "Failed to persist workflow node failure", logger.KeyID, wf.ID, logger.KeyError, storeErr)
			return nil, storeErr
		}
		return nil, err
	}

	entry.Output = result.Output
	if len(result.Payload.Files) > 0 {
		payload, marshalErr := json.Marshal(result.Payload)
		if marshalErr != nil {
			return nil, marshalErr
		}
		entry.Payload = payload
	}
	if err := w.service.workflows.CompleteNode(ctx, entry); err != nil {
		w.service.logger.ErrorContext(ctx, "Failed to persist workflow node completion", logger.KeyID, wf.ID, logger.KeyError, err)
		return nil, err
	}
	return result.Output, nil
}
