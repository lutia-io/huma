package executor

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PipelineStore interface {
	ClaimOne(ctx context.Context, workerID string) (*Pipeline, error)
	CompleteNode(ctx context.Context, entry PipelineNode) error
	FailNode(ctx context.Context, entry PipelineNode) error
	ListTerminalNodes(ctx context.Context, pipelineID string, levelIndex int) ([]TerminalNode, error)
	AdvanceLevel(ctx context.Context, pipelineID string, nextLevel int) error
	Finish(ctx context.Context, pipelineID string, failed bool, message string) error
	FailExhausted(ctx context.Context) (int64, error)
}

type postgresPipelineStore struct {
	db    *pgxpool.Pool
	lease time.Duration
}

func NewPostgresPipelineStore(pool *pgxpool.Pool, lease time.Duration) PipelineStore {
	return &postgresPipelineStore{db: pool, lease: lease}
}

func (s *postgresPipelineStore) ClaimOne(ctx context.Context, workerID string) (*Pipeline, error) {
	const sql = `
		UPDATE public.pipelines SET
			status = 'running',
			locked_by = $1,
			locked_at = now(),
			attempts = attempts + 1
		WHERE id = (
			SELECT id FROM public.pipelines
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
			pipeline_definition_id,
			network_id,
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

	p := &Pipeline{}
	var inputJSON, defJSON []byte
	err := s.db.QueryRow(ctx, sql, workerID, s.lease.Seconds()).Scan(
		&p.ID,
		&p.PipelineDefinitionID,
		&p.NetworkID,
		&inputJSON,
		&p.OrganizationID,
		&p.OrganizationUserID,
		&p.DedupeKey,
		&defJSON,
		&p.Status,
		&p.CurrentLevel,
		&p.Attempts,
		&p.MaxAttempts,
		&p.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(inputJSON, &p.Input); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(defJSON, &p.Definition); err != nil {
		return nil, err
	}
	return p, nil
}

const insertNodeSQL = `
	INSERT INTO public.pipeline_nodes (
		pipeline_id,
		level_index,
		node_index,
		attempt,
		node_definition_id,
		node_slug,
		node_type,
		status,
		input,
		output,
		error,
		started_at,
		completed_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, ''), $12, now()
	)`

func (s *postgresPipelineStore) journalNode(ctx context.Context, entry PipelineNode, status NodeStatus) error {
	_, err := s.db.Exec(ctx, insertNodeSQL,
		entry.PipelineID,
		entry.LevelIndex,
		entry.NodeIndex,
		entry.Attempt,
		entry.NodeDefinitionID,
		entry.NodeSlug,
		entry.NodeType,
		status,
		entry.Input,
		entry.Output,
		entry.Error,
		entry.StartedAt,
	)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `UPDATE public.pipelines SET locked_at = now() WHERE id = $1`, entry.PipelineID)
	return err
}

func (s *postgresPipelineStore) CompleteNode(ctx context.Context, entry PipelineNode) error {
	return s.journalNode(ctx, entry, NodeStatusCompleted)
}

func (s *postgresPipelineStore) FailNode(ctx context.Context, entry PipelineNode) error {
	return s.journalNode(ctx, entry, NodeStatusFailed)
}

func (s *postgresPipelineStore) ListTerminalNodes(ctx context.Context, pipelineID string, levelIndex int) ([]TerminalNode, error) {
	const sql = `
		SELECT DISTINCT ON (node_index) node_index, status, output
		FROM public.pipeline_nodes
		WHERE pipeline_id = $1 AND level_index = $2
		ORDER BY node_index,
			CASE status WHEN 'completed' THEN 0 ELSE 1 END,
			completed_at DESC`

	rows, err := s.db.Query(ctx, sql, pipelineID, levelIndex)
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

func (s *postgresPipelineStore) AdvanceLevel(ctx context.Context, pipelineID string, nextLevel int) error {
	const sql = `
		UPDATE public.pipelines
		SET current_level = $2, locked_at = now()
		WHERE id = $1`
	_, err := s.db.Exec(ctx, sql, pipelineID, nextLevel)
	return err
}

func (s *postgresPipelineStore) Finish(ctx context.Context, pipelineID string, failed bool, message string) error {
	status := StatusCompleted
	var errMsg *string
	if failed {
		status = StatusFailed
		errMsg = &message
	}
	const sql = `
		UPDATE public.pipelines
		SET status = $2,
			error = $3,
			completed_at = now(),
			locked_by = NULL,
			locked_at = NULL
		WHERE id = $1`
	_, err := s.db.Exec(ctx, sql, pipelineID, status, errMsg)
	return err
}

func (s *postgresPipelineStore) FailExhausted(ctx context.Context) (int64, error) {
	const sql = `
		UPDATE public.pipelines
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
