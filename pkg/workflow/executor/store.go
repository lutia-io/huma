package executor

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WorkflowStore persists workflow state and the action journal.
type WorkflowStore interface {
	// InsertPending inserts pending workflows. Workflows whose (definition,
	// dedupe key) pair already exists are silently skipped, which makes
	// intake idempotent under event redelivery.
	InsertPending(ctx context.Context, workflows []*Workflow) error

	// ClaimOne atomically claims the oldest due workflow for a worker,
	// marking it running, stamping the lease, and incrementing attempts.
	// Workflows whose lease has expired (crashed worker) are reclaimed
	// automatically. Returns nil when nothing is claimable.
	ClaimOne(ctx context.Context, workerID string) (*Workflow, error)

	// CompleteAction journals a successful action attempt and advances the
	// workflow's cursor in one transaction. After it commits, the action
	// never re-runs. It also refreshes the lease, acting as a per-action
	// heartbeat.
	CompleteAction(ctx context.Context, entry WorkflowAction) error

	// FailAction journals a failed action attempt and advances the
	// workflow's cursor in one transaction, so execution continues with the
	// remaining actions (continue-on-error policy).
	FailAction(ctx context.Context, entry WorkflowAction) error

	// Finish terminates the workflow after all actions were attempted. The
	// final status is derived from the journal so it stays correct across
	// crash reclaims: failed if any action never completed, else completed.
	Finish(ctx context.Context, workflowID string) (Status, error)

	// FailExhausted marks workflows that exceeded max_attempts (crash loops)
	// as failed so they stop being claimable limbo rows. Returns rows
	// affected.
	FailExhausted(ctx context.Context) (int64, error)
}

type postgresWorkflowStore struct {
	db    *pgxpool.Pool
	lease time.Duration
}

// NewPostgresWorkflowStore returns a WorkflowStore backed by Postgres.
// lease is how long a crashed worker's row stays unclaimable before another
// worker may reclaim it.
func NewPostgresWorkflowStore(pool *pgxpool.Pool, lease time.Duration) WorkflowStore {
	return &postgresWorkflowStore{db: pool, lease: lease}
}

// InsertPending inserts each workflow as pending. Conflicts on
// (workflow_definition_id, dedupe_key) are ignored so event redelivery is safe.
func (s *postgresWorkflowStore) InsertPending(ctx context.Context, workflows []*Workflow) error {
	const sql = `
		INSERT INTO public.workflows (
			workflow_definition_id,
			network_id,
			record_id,
			data,
			organization_id,
			organization_user_id,
			dedupe_key,
			definition,
			status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, 'pending'
		)
		ON CONFLICT (workflow_definition_id, dedupe_key) DO NOTHING`

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, workflow := range workflows {
		dataJSON, err := json.Marshal(workflow.Data)
		if err != nil {
			return err
		}
		defJSON, err := json.Marshal(workflow.Definition)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, sql,
			workflow.WorkflowDefinitionID,
			workflow.NetworkID,
			workflow.RecordID,
			dataJSON,
			workflow.OrganizationID,
			workflow.OrganizationUserID,
			workflow.DedupeKey,
			defJSON,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ClaimOne is a single atomic statement, which is what makes the pool safe
// without any coordinator:
//
//   - FOR UPDATE SKIP LOCKED lets concurrent workers claim different rows
//     without ever blocking or colliding on the same workflow.
//   - The locked_at predicate reclaims workflows whose worker died
//     mid-execution: once the lease ages out, the workflow becomes claimable
//     again and the new worker resumes from current_action.
//   - attempts < max_attempts stops crash-looping workflows from being
//     retried forever; the sweeper later marks them failed.
//
// The claimable set is served by a partial index on next_attempt_at, so this
// stays fast no matter how many terminal rows accumulate.
func (s *postgresWorkflowStore) ClaimOne(ctx context.Context, workerID string) (*Workflow, error) {
	const sql = `
		UPDATE public.workflows SET
			status = 'running',
			locked_by = $1,
			locked_at = now(),
			attempts = attempts + 1
		WHERE id = (
			SELECT id FROM public.workflows
			WHERE status IN ('pending', 'running')
			  AND next_attempt_at <= now()
			  AND (locked_at IS NULL OR locked_at < now() - make_interval(secs => $2))
			  AND attempts < max_attempts
			ORDER BY next_attempt_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING
			id,
			workflow_definition_id,
			network_id,
			record_id,
			data,
			organization_id,
			organization_user_id,
			dedupe_key,
			definition,
			status,
			current_action,
			attempts,
			max_attempts,
			created_at`

	workflow := &Workflow{}
	var dataJSON, defJSON []byte
	err := s.db.QueryRow(ctx, sql, workerID, s.lease.Seconds()).Scan(
		&workflow.ID,
		&workflow.WorkflowDefinitionID,
		&workflow.NetworkID,
		&workflow.RecordID,
		&dataJSON,
		&workflow.OrganizationID,
		&workflow.OrganizationUserID,
		&workflow.DedupeKey,
		&defJSON,
		&workflow.Status,
		&workflow.CurrentAction,
		&workflow.Attempts,
		&workflow.MaxAttempts,
		&workflow.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(dataJSON, &workflow.Data); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(defJSON, &workflow.Definition); err != nil {
		return nil, err
	}
	return workflow, nil
}

const insertActionSQL = `
	INSERT INTO public.workflow_actions (
		workflow_id,
		action_index,
		attempt,
		action_type,
		status,
		input,
		output,
		error,
		started_at,
		completed_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, now()
	)`

func (s *postgresWorkflowStore) CompleteAction(ctx context.Context, entry WorkflowAction) error {
	return s.journalAndAdvance(ctx, entry, ActionStatusCompleted)
}

func (s *postgresWorkflowStore) FailAction(ctx context.Context, entry WorkflowAction) error {
	return s.journalAndAdvance(ctx, entry, ActionStatusFailed)
}

// journalAndAdvance appends the action attempt to the journal and advances
// the workflow's cursor in one transaction. After it commits, the action
// never re-runs, whatever its outcome. It also refreshes the lease so long
// action lists do not expire mid-workflow.
func (s *postgresWorkflowStore) journalAndAdvance(ctx context.Context, entry WorkflowAction, status ActionStatus) error {
	const advanceSQL = `
		UPDATE public.workflows
		SET current_action = $2, locked_at = now()
		WHERE id = $1`

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, insertActionSQL,
		entry.WorkflowID,
		entry.ActionIndex,
		entry.Attempt,
		entry.ActionType,
		status,
		entry.Input,
		entry.Output,
		entry.Error,
		entry.StartedAt,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, advanceSQL, entry.WorkflowID, entry.ActionIndex+1); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Finish derives terminal status from the journal and clears the lease.
// An action index counts as failed only if it never completed: a crash
// between the side effect and the journal commit can leave a failed attempt
// followed by a completed one for the same index.
func (s *postgresWorkflowStore) Finish(ctx context.Context, workflowID string) (Status, error) {
	const sql = `
		WITH failed AS (
			SELECT count(*) AS n FROM (
				SELECT action_index
				FROM public.workflow_actions
				WHERE workflow_id = $1
				GROUP BY action_index
				HAVING bool_and(status = 'failed')
			) f
		)
		UPDATE public.workflows w
		SET status = CASE WHEN failed.n > 0 THEN 'failed' ELSE 'completed' END,
			error = CASE WHEN failed.n > 0 THEN failed.n::text || ' action(s) failed' END,
			completed_at = now(),
			locked_by = NULL,
			locked_at = NULL
		FROM failed
		WHERE w.id = $1
		RETURNING w.status`

	var status Status
	if err := s.db.QueryRow(ctx, sql, workflowID).Scan(&status); err != nil {
		return "", err
	}
	return status, nil
}

// FailExhausted terminalizes workflows that have been claimed MaxAttempts
// times without finishing, once their lease has expired so an in-flight
// worker is not racing the update.
func (s *postgresWorkflowStore) FailExhausted(ctx context.Context) (int64, error) {
	const sql = `
		UPDATE public.workflows
		SET status = 'failed',
			error = 'max attempts exhausted',
			completed_at = now(),
			locked_by = NULL,
			locked_at = NULL
		WHERE status IN ('pending', 'running')
		  AND attempts >= max_attempts
		  AND (locked_at IS NULL OR locked_at < now() - make_interval(secs => $1))`

	tag, err := s.db.Exec(ctx, sql, s.lease.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
