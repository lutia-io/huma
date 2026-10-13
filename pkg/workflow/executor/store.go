package executor

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkflowStore interface {
	InsertPending(ctx context.Context, workflows []*Workflow) error
	ClaimOne(ctx context.Context, workerID string) (*Workflow, error)
	CompleteNode(ctx context.Context, entry WorkflowNode) error
	FailNode(ctx context.Context, entry WorkflowNode) error
	ListTerminalNodes(ctx context.Context, workflowID string, levelIndex int) ([]TerminalNode, error)
	AdvanceLevel(ctx context.Context, workflowID string, nextLevel int) error
	Finish(ctx context.Context, workflowID string, failed bool, message string) error
	FailExhausted(ctx context.Context) (int64, error)
}

type postgresWorkflowStore struct {
	db    *pgxpool.Pool
	lease time.Duration
}

func NewPostgresWorkflowStore(pool *pgxpool.Pool, lease time.Duration) WorkflowStore {
	return &postgresWorkflowStore{db: pool, lease: lease}
}

func (s *postgresWorkflowStore) InsertPending(ctx context.Context, workflows []*Workflow) error {
	const sql = `
		INSERT INTO public.workflows (
			workflow_definition_id,
			network_id,
			record_id,
			data,
			input,
			organization_id,
			organization_user_id,
			dedupe_key,
			definition,
			status
		) VALUES (
			$1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, $9, 'pending'
		)
		ON CONFLICT (workflow_definition_id, dedupe_key) DO NOTHING`

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, wf := range workflows {
		dataJSON, err := json.Marshal(wf.Data)
		if err != nil {
			return err
		}
		inputJSON, err := json.Marshal(wf.Input)
		if err != nil {
			return err
		}
		defJSON, err := json.Marshal(wf.Definition)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, sql,
			wf.WorkflowDefinitionID,
			wf.NetworkID,
			wf.RecordID,
			dataJSON,
			inputJSON,
			wf.OrganizationID,
			wf.OrganizationUserID,
			wf.DedupeKey,
			defJSON,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

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
			COALESCE(record_id::text, ''),
			data,
			input,
			organization_id,
			organization_user_id,
			dedupe_key,
			definition,
			status,
			current_level,
			attempts,
			max_attempts,
			created_at`

	wf := &Workflow{}
	var dataJSON, inputJSON, defJSON []byte
	var recordID *string
	err := s.db.QueryRow(ctx, sql, workerID, s.lease.Seconds()).Scan(
		&wf.ID,
		&wf.WorkflowDefinitionID,
		&wf.NetworkID,
		&recordID,
		&dataJSON,
		&inputJSON,
		&wf.OrganizationID,
		&wf.OrganizationUserID,
		&wf.DedupeKey,
		&defJSON,
		&wf.Status,
		&wf.CurrentLevel,
		&wf.Attempts,
		&wf.MaxAttempts,
		&wf.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if recordID != nil {
		wf.RecordID = *recordID
	}
	if err := json.Unmarshal(dataJSON, &wf.Data); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(inputJSON, &wf.Input); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(defJSON, &wf.Definition); err != nil {
		return nil, err
	}
	return wf, nil
}

const insertNodeSQL = `
	INSERT INTO public.workflow_nodes (
		workflow_id,
		level_index,
		node_index,
		attempt,
		node_type,
		status,
		input,
		output,
		payload,
		error,
		started_at,
		completed_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), $11, now()
	)`

func (s *postgresWorkflowStore) journalNode(ctx context.Context, entry WorkflowNode, status NodeStatus) error {
	_, err := s.db.Exec(ctx, insertNodeSQL,
		entry.WorkflowID,
		entry.LevelIndex,
		entry.NodeIndex,
		entry.Attempt,
		entry.NodeType,
		status,
		entry.Input,
		entry.Output,
		entry.Payload,
		entry.Error,
		entry.StartedAt,
	)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `UPDATE public.workflows SET locked_at = now() WHERE id = $1`, entry.WorkflowID)
	return err
}

func (s *postgresWorkflowStore) CompleteNode(ctx context.Context, entry WorkflowNode) error {
	return s.journalNode(ctx, entry, NodeStatusCompleted)
}

func (s *postgresWorkflowStore) FailNode(ctx context.Context, entry WorkflowNode) error {
	return s.journalNode(ctx, entry, NodeStatusFailed)
}

func (s *postgresWorkflowStore) ListTerminalNodes(ctx context.Context, workflowID string, levelIndex int) ([]TerminalNode, error) {
	const sql = `
		SELECT DISTINCT ON (node_index) node_index, status, output
		FROM public.workflow_nodes
		WHERE workflow_id = $1 AND level_index = $2
		ORDER BY node_index,
			CASE status WHEN 'completed' THEN 0 ELSE 1 END,
			completed_at DESC`

	rows, err := s.db.Query(ctx, sql, workflowID, levelIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TerminalNode, 0)
	for rows.Next() {
		var n TerminalNode
		var status string
		if err := rows.Scan(&n.NodeIndex, &status, &n.Output); err != nil {
			return nil, err
		}
		n.Status = NodeStatus(status)
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *postgresWorkflowStore) AdvanceLevel(ctx context.Context, workflowID string, nextLevel int) error {
	const sql = `
		UPDATE public.workflows
		SET current_level = $2, locked_at = now()
		WHERE id = $1`
	_, err := s.db.Exec(ctx, sql, workflowID, nextLevel)
	return err
}

func (s *postgresWorkflowStore) Finish(ctx context.Context, workflowID string, failed bool, message string) error {
	status := StatusCompleted
	var errMsg *string
	if failed {
		status = StatusFailed
		errMsg = &message
	}
	const sql = `
		UPDATE public.workflows
		SET status = $2,
			error = $3,
			completed_at = now(),
			locked_by = NULL,
			locked_at = NULL
		WHERE id = $1`
	_, err := s.db.Exec(ctx, sql, workflowID, status, errMsg)
	return err
}

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
