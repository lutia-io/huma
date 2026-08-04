package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/lutia-io/huma/pkg/logger"
)

// worker is one goroutine in the Service pool. It only talks to Postgres
// through WorkflowStore; it does not consume message-broker events.
type worker struct {
	id           string
	service      *Service
	pollInterval time.Duration
}

// workerID identifies who holds a workflow's lease (workflows.locked_by),
// mostly for debugging: it answers "which pod was executing this when it
// died".
func workerID(prefix string, index int) string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return fmt.Sprintf("%s-%s-%d-%d", prefix, host, os.Getpid(), index)
}

// run claims and executes workflows until ctx is cancelled, sleeping
// pollInterval between claims when the queue is empty.
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

// sleep waits pollInterval or until ctx is cancelled. Used when the claim
// queue is empty or a claim query failed transiently.
func (w *worker) sleep(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(w.pollInterval):
	}
}

// execute resumes the workflow from its cursor and executes remaining actions
// in order. Failure policy is continue-on-error: a failed action is journaled
// and logged, and execution proceeds with the remaining actions. The workflow
// is marked failed at the end if any action failed.
func (w *worker) execute(ctx context.Context, workflow *Workflow) {
	actions := workflow.Definition.Actions

	w.service.logger.InfoContext(ctx, "Executing workflow",
		logger.KeyID, workflow.ID,
		"workflow_definition_id", workflow.WorkflowDefinitionID,
		"record_id", workflow.RecordID,
		"current_action", workflow.CurrentAction,
		"attempt", workflow.Attempts,
		logger.KeyCount, len(actions),
	)

	for i := workflow.CurrentAction; i < len(actions); i++ {
		act := actions[i]
		execCtx := ExecutionContext{
			WorkflowID:           workflow.ID,
			WorkflowDefinitionID: workflow.WorkflowDefinitionID,
			NetworkID:            workflow.NetworkID,
			TriggerRecordID:      workflow.RecordID,
			TriggerData:          workflow.Data,
			OrganizationID:       workflow.OrganizationID,
			OrganizationUserID:   workflow.OrganizationUserID,
			ActionIndex:          i,
			// Stable across crash reclaims of this action so side effects
			// (e.g. record inserts) converge instead of duplicating.
			IdempotencyKey: fmt.Sprintf("%s:%d", workflow.ID, i),
		}

		// The journal stores the action as executed, so a definition bug is
		// diagnosable from the workflow history alone.
		input, err := json.Marshal(act)
		if err != nil {
			input = nil
		}
		entry := WorkflowAction{
			WorkflowID:  workflow.ID,
			ActionIndex: i,
			Attempt:     workflow.Attempts,
			ActionType:  string(act.Type),
			Input:       input,
			StartedAt:   time.Now().UTC(),
		}

		output, err := w.service.registry.Execute(ctx, execCtx, act)
		if err != nil {
			entry.Error = err.Error()
			w.service.logger.ErrorContext(ctx, "Workflow action failed, continuing with remaining actions",
				logger.KeyID, workflow.ID,
				"action_index", i,
				"action_type", act.Type,
				logger.KeyError, err,
			)
			if storeErr := w.service.workflows.FailAction(ctx, entry); storeErr != nil {
				// The cursor did not advance; the workflow stays leased as
				// running and is reclaimed after the lease expires, resuming
				// here.
				w.service.logger.ErrorContext(ctx, "Failed to persist workflow action failure", logger.KeyID, workflow.ID, "action_index", i, logger.KeyError, storeErr)
				return
			}
			continue
		}

		entry.Output = output
		if err := w.service.workflows.CompleteAction(ctx, entry); err != nil {
			// The side effect happened but the cursor did not advance. On
			// reclaim, the action re-runs and its idempotency key resolves it
			// to the original result.
			w.service.logger.ErrorContext(ctx, "Failed to persist workflow action completion", logger.KeyID, workflow.ID, "action_index", i, logger.KeyError, err)
			return
		}
	}

	status, err := w.service.workflows.Finish(ctx, workflow.ID)
	if err != nil {
		w.service.logger.ErrorContext(ctx, "Failed to finish workflow", logger.KeyID, workflow.ID, logger.KeyError, err)
		return
	}
	if status == StatusFailed {
		w.service.logger.ErrorContext(ctx, "Workflow finished with failed actions", logger.KeyID, workflow.ID, "record_id", workflow.RecordID)
		return
	}
	w.service.logger.InfoContext(ctx, "Workflow completed", logger.KeyID, workflow.ID, "record_id", workflow.RecordID)
}
