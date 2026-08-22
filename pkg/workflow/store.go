package workflow

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

const workflowDefinitionSelectColumns = `
	id, name, slug, active, internal, definition, schema_id, network_id,
	user_id, created_at, updated_at, deleted_at`

type store interface {
	Insert(ctx context.Context, workflow *WorkflowDefinition) (string, error)
	Update(ctx context.Context, workflow *WorkflowDefinition) error
	GetByID(ctx context.Context, id string) (*WorkflowDefinition, error)
	ListByUserID(ctx context.Context, userID string) ([]*WorkflowDefinition, error)
	ListVisibleToOrganization(ctx context.Context, networkID, organizationID string) ([]*WorkflowDefinition, error)
	ListActiveBySchemaID(ctx context.Context, schemaID string) ([]*WorkflowDefinition, error)
	SchemaVisibleToOrganization(ctx context.Context, schemaID, networkID, organizationID string) (bool, error)

	GetWorkflowByID(ctx context.Context, id string) (*Workflow, error)
	ListWorkflowsByUserID(ctx context.Context, userID string) ([]*Workflow, error)
	ListWorkflowsByOrganization(ctx context.Context, networkID, organizationID string) ([]*Workflow, error)
	GetWorkflowActionByID(ctx context.Context, id string) (*WorkflowAction, error)
	ListWorkflowActionsByWorkflowID(ctx context.Context, workflowID string) ([]*WorkflowAction, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, workflow *WorkflowDefinition) (string, error) {
	const sql = `
		INSERT INTO public.workflow_definitions (
			name,
			slug,
			active,
			internal,
			definition,
			schema_id,
			network_id,
			user_id,
			created_at,
			updated_at
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, now(), now()
		FROM public.schemas
		WHERE id = $6
			AND network_id = $7
			AND deleted_at IS NULL
		RETURNING id`

	defJSON, err := json.Marshal(workflow.Definition)
	if err != nil {
		return "", err
	}

	err = store.db.QueryRow(ctx, sql,
		workflow.Name,
		workflow.Slug,
		workflow.Active,
		workflow.Internal,
		defJSON,
		workflow.SchemaID,
		workflow.NetworkID,
		workflow.UserID,
	).Scan(&workflow.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperror.NewBadRequestError("Schema not found in this network", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return "", apperror.NewConflictError("Workflow definition already exists", err)
			case "23503":
				return "", apperror.NewBadRequestError("Invalid network, schema, or user", err)
			}
		}
		return "", err
	}
	return workflow.ID, nil
}

func (store *postgresStore) Update(ctx context.Context, workflow *WorkflowDefinition) error {
	defJSON, err := json.Marshal(workflow.Definition)
	if err != nil {
		return err
	}

	const sql = `
		UPDATE public.workflow_definitions
		SET name = $2,
			slug = $3,
			active = $4,
			definition = $5,
			schema_id = $6,
			updated_at = now()
		WHERE id = $1
			AND deleted_at IS NULL
			AND EXISTS (
				SELECT 1
				FROM public.schemas
				WHERE id = $6
					AND network_id = $7
					AND deleted_at IS NULL
			)`

	tag, err := store.db.Exec(ctx, sql,
		workflow.ID,
		workflow.Name,
		workflow.Slug,
		workflow.Active,
		defJSON,
		workflow.SchemaID,
		workflow.NetworkID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return apperror.NewConflictError("Workflow definition already exists", err)
			case "23503":
				return apperror.NewBadRequestError("Invalid network, schema, or user", err)
			}
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		if _, err := store.GetByID(ctx, workflow.ID); err != nil {
			return err
		}
		return apperror.NewBadRequestError("Schema not found in this network", nil)
	}
	return nil
}

func scanWorkflowDefinition(row pgx.Row, wf *WorkflowDefinition) error {
	var defJSON []byte
	if err := row.Scan(
		&wf.ID,
		&wf.Name,
		&wf.Slug,
		&wf.Active,
		&wf.Internal,
		&defJSON,
		&wf.SchemaID,
		&wf.NetworkID,
		&wf.UserID,
		&wf.CreatedAt,
		&wf.UpdatedAt,
		&wf.DeletedAt,
	); err != nil {
		return err
	}
	return json.Unmarshal(defJSON, &wf.Definition)
}

func collectWorkflowDefinitions(rows pgx.Rows) ([]*WorkflowDefinition, error) {
	defer rows.Close()

	workflows := make([]*WorkflowDefinition, 0)
	for rows.Next() {
		wf := &WorkflowDefinition{}
		if err := scanWorkflowDefinition(rows, wf); err != nil {
			return nil, err
		}
		workflows = append(workflows, wf)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return workflows, nil
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*WorkflowDefinition, error) {
	const sql = `
		SELECT` + workflowDefinitionSelectColumns + `
		FROM public.workflow_definitions
		WHERE id = $1 AND deleted_at IS NULL`

	wf := &WorkflowDefinition{}
	err := scanWorkflowDefinition(store.db.QueryRow(ctx, sql, id), wf)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Workflow definition not found", err)
		}
		return nil, err
	}
	return wf, nil
}

func (store *postgresStore) ListByUserID(ctx context.Context, userID string) ([]*WorkflowDefinition, error) {
	const sql = `
		SELECT` + workflowDefinitionSelectColumns + `
		FROM public.workflow_definitions
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := store.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	return collectWorkflowDefinitions(rows)
}

func (store *postgresStore) ListVisibleToOrganization(ctx context.Context, networkID, organizationID string) ([]*WorkflowDefinition, error) {
	const sql = `
		SELECT
			wd.id,
			wd.name,
			wd.slug,
			wd.active,
			wd.internal,
			wd.definition,
			wd.schema_id,
			wd.network_id,
			wd.user_id,
			wd.created_at,
			wd.updated_at,
			wd.deleted_at
		FROM public.workflow_definitions wd
		JOIN public.schemas s ON s.id = wd.schema_id
		WHERE wd.network_id = $1
			AND wd.deleted_at IS NULL
			AND s.deleted_at IS NULL
			AND (s.organization_id IS NULL OR s.organization_id = $2)
		ORDER BY wd.created_at DESC`

	rows, err := store.db.Query(ctx, sql, networkID, organizationID)
	if err != nil {
		return nil, err
	}
	return collectWorkflowDefinitions(rows)
}

// ListActiveBySchemaID returns the definitions the engine's intake considers
// for a record event on the given schema.
func (store *postgresStore) ListActiveBySchemaID(ctx context.Context, schemaID string) ([]*WorkflowDefinition, error) {
	const sql = `
		SELECT` + workflowDefinitionSelectColumns + `
		FROM public.workflow_definitions
		WHERE schema_id = $1
			AND active = true
			AND deleted_at IS NULL`

	rows, err := store.db.Query(ctx, sql, schemaID)
	if err != nil {
		return nil, err
	}
	return collectWorkflowDefinitions(rows)
}

func (store *postgresStore) SchemaVisibleToOrganization(ctx context.Context, schemaID, networkID, organizationID string) (bool, error) {
	const sql = `
		SELECT 1
		FROM public.schemas
		WHERE id = $1
			AND network_id = $2
			AND deleted_at IS NULL
			AND (organization_id IS NULL OR organization_id = $3)`

	var present int
	err := store.db.QueryRow(ctx, sql, schemaID, networkID, organizationID).Scan(&present)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
