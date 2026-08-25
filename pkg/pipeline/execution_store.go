package pipeline

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/lutia-io/huma/pkg/apperror"
)

const pipelineRunSelectColumns = `
	p.id,
	p.pipeline_definition_id,
	p.network_id,
	p.input,
	p.organization_id,
	p.organization_user_id,
	p.definition,
	p.status,
	p.current_level,
	p.attempts,
	p.max_attempts,
	p.error,
	p.created_at,
	p.completed_at,
	n.user_id`

const pipelineNodeSelectColumns = `
	id, pipeline_id, level_index, node_index, attempt, node_definition_id,
	node_slug, node_type, status, input, output, error, started_at, completed_at`

func (store *postgresStore) InsertPending(ctx context.Context, p *Pipeline) (string, error) {
	const insertSQL = `
		INSERT INTO public.pipelines (
			pipeline_definition_id,
			network_id,
			organization_id,
			organization_user_id,
			dedupe_key,
			input,
			definition,
			status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, 'pending'
		)
		ON CONFLICT (pipeline_definition_id, dedupe_key) DO NOTHING
		RETURNING id`

	inputJSON, err := json.Marshal(p.Input)
	if err != nil {
		return "", err
	}
	defJSON, err := json.Marshal(p.Definition)
	if err != nil {
		return "", err
	}

	err = store.db.QueryRow(ctx, insertSQL,
		p.PipelineDefinitionID,
		p.NetworkID,
		p.OrganizationID,
		p.OrganizationUserID,
		p.DedupeKey,
		inputJSON,
		defJSON,
	).Scan(&p.ID)
	if err == nil {
		return p.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	const selectSQL = `
		SELECT id FROM public.pipelines
		WHERE pipeline_definition_id = $1 AND dedupe_key = $2`

	if err := store.db.QueryRow(ctx, selectSQL, p.PipelineDefinitionID, p.DedupeKey).Scan(&p.ID); err != nil {
		return "", err
	}
	return p.ID, nil
}

func scanPipeline(row pgx.Row, p *Pipeline) error {
	var inputJSON, defJSON []byte
	var errMsg *string
	if err := row.Scan(
		&p.ID,
		&p.PipelineDefinitionID,
		&p.NetworkID,
		&inputJSON,
		&p.OrganizationID,
		&p.OrganizationUserID,
		&defJSON,
		&p.Status,
		&p.CurrentLevel,
		&p.Attempts,
		&p.MaxAttempts,
		&errMsg,
		&p.CreatedAt,
		&p.CompletedAt,
		&p.UserID,
	); err != nil {
		return err
	}
	if errMsg != nil {
		p.Error = *errMsg
	}
	if err := json.Unmarshal(inputJSON, &p.Input); err != nil {
		return err
	}
	return json.Unmarshal(defJSON, &p.Definition)
}

func collectPipelines(rows pgx.Rows) ([]*Pipeline, error) {
	defer rows.Close()

	pipelines := make([]*Pipeline, 0)
	for rows.Next() {
		p := &Pipeline{}
		if err := scanPipeline(rows, p); err != nil {
			return nil, err
		}
		pipelines = append(pipelines, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pipelines, nil
}

func (store *postgresStore) GetPipelineByID(ctx context.Context, id string) (*Pipeline, error) {
	const sql = `
		SELECT` + pipelineRunSelectColumns + `
		FROM public.pipelines p
		JOIN public.networks n ON n.id = p.network_id
		WHERE p.id = $1 AND n.deleted_at IS NULL`

	p := &Pipeline{}
	err := scanPipeline(store.db.QueryRow(ctx, sql, id), p)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Pipeline not found", err)
		}
		return nil, err
	}
	return p, nil
}

func (store *postgresStore) ListPipelines(ctx context.Context, params runListParams) (*runListResult, error) {
	countSQL, listSQL, countArgs, listArgs := buildRunListQuery(params)

	var total int
	if err := store.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := store.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, err
	}
	items, err := collectPipelines(rows)
	if err != nil {
		return nil, err
	}

	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = total
	}

	return &runListResult{
		Items:    items,
		Total:    total,
		Page:     params.Page,
		PageSize: pageSize,
	}, nil
}

func optionalJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return json.RawMessage(raw)
}

func scanPipelineNode(row pgx.Row, n *PipelineNode) error {
	var input, output []byte
	var errMsg *string
	if err := row.Scan(
		&n.ID,
		&n.PipelineID,
		&n.LevelIndex,
		&n.NodeIndex,
		&n.Attempt,
		&n.NodeDefinitionID,
		&n.NodeSlug,
		&n.NodeType,
		&n.Status,
		&input,
		&output,
		&errMsg,
		&n.StartedAt,
		&n.CompletedAt,
	); err != nil {
		return err
	}
	n.Input = optionalJSON(input)
	n.Output = optionalJSON(output)
	if errMsg != nil {
		n.Error = *errMsg
	}
	return nil
}

func collectPipelineNodes(rows pgx.Rows) ([]*PipelineNode, error) {
	defer rows.Close()

	nodes := make([]*PipelineNode, 0)
	for rows.Next() {
		n := &PipelineNode{}
		if err := scanPipelineNode(rows, n); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (store *postgresStore) GetPipelineNodeByID(ctx context.Context, id string) (*PipelineNode, error) {
	const sql = `
		SELECT` + pipelineNodeSelectColumns + `
		FROM public.pipeline_nodes
		WHERE id = $1`

	n := &PipelineNode{}
	err := scanPipelineNode(store.db.QueryRow(ctx, sql, id), n)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Pipeline node not found", err)
		}
		return nil, err
	}
	return n, nil
}

func (store *postgresStore) ListPipelineNodesByPipelineID(ctx context.Context, pipelineID string) ([]*PipelineNode, error) {
	const sql = `
		SELECT` + pipelineNodeSelectColumns + `
		FROM public.pipeline_nodes
		WHERE pipeline_id = $1
		ORDER BY level_index, node_index, attempt`

	rows, err := store.db.Query(ctx, sql, pipelineID)
	if err != nil {
		return nil, err
	}
	return collectPipelineNodes(rows)
}
