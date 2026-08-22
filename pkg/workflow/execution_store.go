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
	w.record_id,
	w.data,
	w.organization_id,
	w.organization_user_id,
	w.definition,
	w.status,
	w.current_action,
	w.attempts,
	w.max_attempts,
	w.error,
	w.created_at,
	w.completed_at,
	n.user_id`

const workflowActionSelectColumns = `
	id, workflow_id, action_index, attempt, action_type, status,
	input, output, error, started_at, completed_at`

func scanWorkflow(row pgx.Row, wf *Workflow) error {
	var dataJSON, defJSON []byte
	var errMsg *string
	if err := row.Scan(
		&wf.ID,
		&wf.WorkflowDefinitionID,
		&wf.NetworkID,
		&wf.RecordID,
		&dataJSON,
		&wf.OrganizationID,
		&wf.OrganizationUserID,
		&defJSON,
		&wf.Status,
		&wf.CurrentAction,
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
		WHERE w.id = $1 AND n.deleted_at IS NULL`

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

func (store *postgresStore) ListWorkflowsByUserID(ctx context.Context, userID string) ([]*Workflow, error) {
	const sql = `
		SELECT` + workflowSelectColumns + `
		FROM public.workflows w
		JOIN public.networks n ON n.id = w.network_id
		WHERE n.user_id = $1 AND n.deleted_at IS NULL
		ORDER BY w.created_at DESC`

	rows, err := store.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	return collectWorkflows(rows)
}

func (store *postgresStore) ListWorkflowsByOrganization(ctx context.Context, networkID, organizationID string) ([]*Workflow, error) {
	const sql = `
		SELECT` + workflowSelectColumns + `
		FROM public.workflows w
		JOIN public.networks n ON n.id = w.network_id
		WHERE w.network_id = $1
			AND w.organization_id = $2
			AND n.deleted_at IS NULL
		ORDER BY w.created_at DESC`

	rows, err := store.db.Query(ctx, sql, networkID, organizationID)
	if err != nil {
		return nil, err
	}
	return collectWorkflows(rows)
}

func optionalJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return json.RawMessage(raw)
}

func scanWorkflowAction(row pgx.Row, action *WorkflowAction) error {
	var input, output []byte
	var errMsg *string
	if err := row.Scan(
		&action.ID,
		&action.WorkflowID,
		&action.ActionIndex,
		&action.Attempt,
		&action.ActionType,
		&action.Status,
		&input,
		&output,
		&errMsg,
		&action.StartedAt,
		&action.CompletedAt,
	); err != nil {
		return err
	}
	action.Input = optionalJSON(input)
	action.Output = optionalJSON(output)
	if errMsg != nil {
		action.Error = *errMsg
	}
	return nil
}

func collectWorkflowActions(rows pgx.Rows) ([]*WorkflowAction, error) {
	defer rows.Close()

	actions := make([]*WorkflowAction, 0)
	for rows.Next() {
		action := &WorkflowAction{}
		if err := scanWorkflowAction(rows, action); err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return actions, nil
}

func (store *postgresStore) GetWorkflowActionByID(ctx context.Context, id string) (*WorkflowAction, error) {
	const sql = `
		SELECT` + workflowActionSelectColumns + `
		FROM public.workflow_actions
		WHERE id = $1`

	action := &WorkflowAction{}
	err := scanWorkflowAction(store.db.QueryRow(ctx, sql, id), action)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Workflow action not found", err)
		}
		return nil, err
	}
	return action, nil
}

func (store *postgresStore) ListWorkflowActionsByWorkflowID(ctx context.Context, workflowID string) ([]*WorkflowAction, error) {
	const sql = `
		SELECT` + workflowActionSelectColumns + `
		FROM public.workflow_actions
		WHERE workflow_id = $1
		ORDER BY action_index, attempt`

	rows, err := store.db.Query(ctx, sql, workflowID)
	if err != nil {
		return nil, err
	}
	return collectWorkflowActions(rows)
}
