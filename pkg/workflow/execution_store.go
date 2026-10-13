package workflow

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/lutia-io/huma/pkg/apperror"
)

const workflowSelectColumns = `
	w.id,
	w.workflow_definition_id,
	w.network_id,
	COALESCE(w.record_id::text, ''),
	w.data,
	w.input,
	w.organization_id,
	w.organization_user_id,
	w.definition,
	w.status,
	w.current_level,
	w.attempts,
	w.max_attempts,
	w.error,
	w.created_at,
	w.completed_at,
	n.user_id`

const workflowNodeSelectColumns = `
	id, workflow_id, level_index, node_index, attempt,
	node_slug, node_type, status, input, output, payload, error, started_at, completed_at`

func scanWorkflow(row pgx.Row, wf *Workflow) error {
	var dataJSON, inputJSON, defJSON []byte
	var errMsg *string
	if err := row.Scan(
		&wf.ID,
		&wf.WorkflowDefinitionID,
		&wf.NetworkID,
		&wf.RecordID,
		&dataJSON,
		&inputJSON,
		&wf.OrganizationID,
		&wf.OrganizationUserID,
		&defJSON,
		&wf.Status,
		&wf.CurrentLevel,
		&wf.Attempts,
		&wf.MaxAttempts,
		&errMsg,
		&wf.CreatedAt,
		&wf.CompletedAt,
		&wf.UserID,
	); err != nil {
		return err
	}
	if errMsg != nil {
		wf.Error = *errMsg
	}
	if err := json.Unmarshal(dataJSON, &wf.Data); err != nil {
		return err
	}
	if err := json.Unmarshal(inputJSON, &wf.Input); err != nil {
		return err
	}
	return json.Unmarshal(defJSON, &wf.Definition)
}

func collectWorkflows(rows pgx.Rows) ([]*Workflow, error) {
	defer rows.Close()

	workflows := make([]*Workflow, 0)
	for rows.Next() {
		wf := &Workflow{}
		if err := scanWorkflow(rows, wf); err != nil {
			return nil, err
		}
		workflows = append(workflows, wf)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return workflows, nil
}

func (store *postgresStore) GetWorkflowByID(ctx context.Context, id string) (*Workflow, error) {
	const sql = `
		SELECT` + workflowSelectColumns + `
		FROM public.workflows w
		JOIN public.networks n ON n.id = w.network_id
		WHERE w.id = $1`

	wf := &Workflow{}
	err := scanWorkflow(store.db.QueryRow(ctx, sql, id), wf)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Workflow not found", err)
		}
		return nil, err
	}
	return wf, nil
}

func (store *postgresStore) ListWorkflows(ctx context.Context, params runListParams) (*runListResult, error) {
	countSQL, listSQL, countArgs, listArgs := buildRunListQuery(params)

	var total int
	if err := store.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := store.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, err
	}
	items, err := collectWorkflows(rows)
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

func scanWorkflowNode(row pgx.Row, node *WorkflowNode) error {
	var input, output, payload []byte
	var errMsg *string
	if err := row.Scan(
		&node.ID,
		&node.WorkflowID,
		&node.LevelIndex,
		&node.NodeIndex,
		&node.Attempt,
		&node.NodeSlug,
		&node.NodeType,
		&node.Status,
		&input,
		&output,
		&payload,
		&errMsg,
		&node.StartedAt,
		&node.CompletedAt,
	); err != nil {
		return err
	}
	node.Input = optionalJSON(input)
	node.Output = optionalJSON(output)
	node.Payload = optionalJSON(payload)
	if errMsg != nil {
		node.Error = *errMsg
	}
	return nil
}

func collectWorkflowNodes(rows pgx.Rows) ([]*WorkflowNode, error) {
	defer rows.Close()

	nodes := make([]*WorkflowNode, 0)
	for rows.Next() {
		n := &WorkflowNode{}
		if err := scanWorkflowNode(rows, n); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (store *postgresStore) GetWorkflowNodeByID(ctx context.Context, id string) (*WorkflowNode, error) {
	const sql = `
		SELECT` + workflowNodeSelectColumns + `
		FROM public.workflow_nodes
		WHERE id = $1`

	n := &WorkflowNode{}
	err := scanWorkflowNode(store.db.QueryRow(ctx, sql, id), n)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Workflow node not found", err)
		}
		return nil, err
	}
	return n, nil
}

func (store *postgresStore) ListWorkflowNodesByWorkflowID(ctx context.Context, workflowID string) ([]*WorkflowNode, error) {
	const sql = `
		SELECT` + workflowNodeSelectColumns + `
		FROM public.workflow_nodes
		WHERE workflow_id = $1
		ORDER BY level_index, node_index, attempt`

	rows, err := store.db.Query(ctx, sql, workflowID)
	if err != nil {
		return nil, err
	}
	return collectWorkflowNodes(rows)
}

func (store *postgresStore) InsertPending(ctx context.Context, run *Workflow) (string, error) {
	const insertSQL = `
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
		ON CONFLICT (workflow_definition_id, dedupe_key) DO NOTHING
		RETURNING id`

	dataJSON, err := json.Marshal(run.Data)
	if err != nil {
		return "", err
	}
	inputJSON, err := json.Marshal(run.Input)
	if err != nil {
		return "", err
	}
	defJSON, err := json.Marshal(run.Definition)
	if err != nil {
		return "", err
	}

	err = store.db.QueryRow(ctx, insertSQL,
		run.WorkflowDefinitionID,
		run.NetworkID,
		run.RecordID,
		dataJSON,
		inputJSON,
		run.OrganizationID,
		run.OrganizationUserID,
		run.DedupeKey,
		defJSON,
	).Scan(&run.ID)
	if err == nil {
		return run.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	const selectSQL = `
		SELECT id FROM public.workflows
		WHERE workflow_definition_id = $1 AND dedupe_key = $2`

	if err := store.db.QueryRow(ctx, selectSQL, run.WorkflowDefinitionID, run.DedupeKey).Scan(&run.ID); err != nil {
		return "", err
	}
	return run.ID, nil
}

func (store *postgresStore) RetryFailed(ctx context.Context, id string, definition RunDefinition) error {
	defJSON, err := json.Marshal(definition)
	if err != nil {
		return err
	}
	const sql = `
		UPDATE public.workflows
		SET status = 'pending',
			current_level = 0,
			max_attempts = attempts + 5,
			error = NULL,
			completed_at = NULL,
			locked_by = NULL,
			locked_at = NULL,
			next_attempt_at = now(),
			definition = $2
		WHERE id = $1 AND status = 'failed'`

	tag, err := store.db.Exec(ctx, sql, id, defJSON)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewBadRequestError("Only failed workflows can be retried", nil)
	}
	return nil
}
