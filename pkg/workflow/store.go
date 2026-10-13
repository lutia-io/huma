package workflow

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/user"
)

const workflowDefinitionSelectColumns = `
	wd.id, wd.name, wd.slug, wd.description, wd.active, wd.internal, wd.definition, wd.schema_id, wd.network_id, wd.organization_id,
	wd.user_id, wd.created_at, wd.updated_at,
	` + user.SelectSQL

var workflowDefinitionListSelectColumns = workflowDefinitionSelectColumns

type store interface {
	Insert(ctx context.Context, workflow *WorkflowDefinition) (string, error)
	Update(ctx context.Context, workflow *WorkflowDefinition) error
	GetByID(ctx context.Context, id string) (*WorkflowDefinition, error)
	List(ctx context.Context, params listParams) (*listResult, error)
	ListActiveBySchemaID(ctx context.Context, schemaID string) ([]*WorkflowDefinition, error)
	ListActiveScheduled(ctx context.Context) ([]*WorkflowDefinition, error)

	GetWorkflowByID(ctx context.Context, id string) (*Workflow, error)
	ListWorkflows(ctx context.Context, params runListParams) (*runListResult, error)
	GetWorkflowNodeByID(ctx context.Context, id string) (*WorkflowNode, error)
	ListWorkflowNodesByWorkflowID(ctx context.Context, workflowID string) ([]*WorkflowNode, error)
	InsertPending(ctx context.Context, run *Workflow) (string, error)
	GetBySlug(ctx context.Context, networkID, slug, organizationID string) (*WorkflowDefinition, error)
	RetryFailed(ctx context.Context, id string, definition RunDefinition) error
}

type postgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, workflow *WorkflowDefinition) (string, error) {
	defJSON, err := json.Marshal(workflow.Definition)
	if err != nil {
		return "", err
	}

	schemaID := optionalID(workflow.SchemaID)
	if schemaID != nil {
		const sql = `
		INSERT INTO public.workflow_definitions (
			name, slug, description, active, internal, definition, schema_id,
			network_id, organization_id, user_id, created_by, updated_by, created_at, updated_at
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now(), now()
		FROM public.schemas
		WHERE id = $7 AND network_id = $8
			AND (
				($9::uuid IS NULL AND organization_id IS NULL)
				OR ($9::uuid IS NOT NULL AND (organization_id IS NULL OR organization_id = $9))
			)
		RETURNING id`
		err = store.db.QueryRow(ctx, sql,
			workflow.Name,
			workflow.Slug,
			workflow.Description,
			workflow.Active,
			workflow.Internal,
			defJSON,
			schemaID,
			workflow.NetworkID,
			workflow.OrganizationID,
			workflow.UserID,
			workflow.CreatedBy.ID,
			workflow.UpdatedBy.ID,
		).Scan(&workflow.ID)
	} else {
		const sql = `
		INSERT INTO public.workflow_definitions (
			name, slug, description, active, internal, definition, schema_id,
			network_id, organization_id, user_id, created_by, updated_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NULL, $7, $8, $9, $10, $11, now(), now())
		RETURNING id`
		err = store.db.QueryRow(ctx, sql,
			workflow.Name,
			workflow.Slug,
			workflow.Description,
			workflow.Active,
			workflow.Internal,
			defJSON,
			workflow.NetworkID,
			workflow.OrganizationID,
			workflow.UserID,
			workflow.CreatedBy.ID,
			workflow.UpdatedBy.ID,
		).Scan(&workflow.ID)
	}
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
				return "", apperror.NewBadRequestError("Invalid network, organization, schema, or user", err)
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
			description = $4,
			active = $5,
			definition = $6,
			schema_id = $7,
			updated_by = $9,
			updated_at = now()
		WHERE id = $1
			AND EXISTS (
				SELECT 1
				FROM public.schemas
				WHERE id = $7
					AND network_id = $8
					AND (
						($10::uuid IS NULL AND organization_id IS NULL)
						OR ($10::uuid IS NOT NULL AND (organization_id IS NULL OR organization_id = $10))
					)
			)`

	tag, err := store.db.Exec(ctx, sql,
		workflow.ID,
		workflow.Name,
		workflow.Slug,
		workflow.Description,
		workflow.Active,
		defJSON,
		workflow.SchemaID,
		workflow.NetworkID,
		workflow.UpdatedBy.ID,
		workflow.OrganizationID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return apperror.NewConflictError("Workflow definition already exists", err)
			case "23503":
				return apperror.NewBadRequestError("Invalid network, organization, schema, or user", err)
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
	var schemaID *string
	if err := row.Scan(
		&wf.ID,
		&wf.Name,
		&wf.Slug,
		&wf.Description,
		&wf.Active,
		&wf.Internal,
		&defJSON,
		&schemaID,
		&wf.NetworkID,
		&wf.OrganizationID,
		&wf.UserID,
		&wf.CreatedAt,
		&wf.UpdatedAt,
		&wf.CreatedBy.ID,
		&wf.CreatedBy.FirstName,
		&wf.CreatedBy.LastName,
		&wf.CreatedBy.Email,
		&wf.UpdatedBy.ID,
		&wf.UpdatedBy.FirstName,
		&wf.UpdatedBy.LastName,
		&wf.UpdatedBy.Email,
	); err != nil {
		return err
	}
	if schemaID != nil {
		wf.SchemaID = *schemaID
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
	sql := `
		SELECT` + workflowDefinitionSelectColumns + `
		FROM public.workflow_definitions wd` + user.JoinSQL("wd") + `
		WHERE wd.id = $1`

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

func (store *postgresStore) List(ctx context.Context, params listParams) (*listResult, error) {
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)

	var total int
	if err := store.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := store.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, err
	}
	items, err := collectWorkflowDefinitions(rows)
	if err != nil {
		return nil, err
	}

	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = total
	}

	return &listResult{
		Items:    items,
		Total:    total,
		Page:     params.Page,
		PageSize: pageSize,
	}, nil
}

// ListActiveBySchemaID returns the definitions the engine's intake considers
// for a record event on the given schema.
func (store *postgresStore) ListActiveBySchemaID(ctx context.Context, schemaID string) ([]*WorkflowDefinition, error) {
	sql := `
		SELECT` + workflowDefinitionSelectColumns + `
		FROM public.workflow_definitions wd` + user.JoinSQL("wd") + `
		WHERE wd.schema_id = $1
			AND wd.active = true`

	rows, err := store.db.Query(ctx, sql, schemaID)
	if err != nil {
		return nil, err
	}
	return collectWorkflowDefinitions(rows)
}

// ListActiveScheduled returns published definitions whose trigger includes
// schedule, for the evaluator's cron ticker.
func (store *postgresStore) ListActiveScheduled(ctx context.Context) ([]*WorkflowDefinition, error) {
	sql := `
		SELECT` + workflowDefinitionSelectColumns + `
		FROM public.workflow_definitions wd` + user.JoinSQL("wd") + `
		WHERE wd.active = true
			AND wd.definition->'trigger'->'on' ? 'schedule'`

	rows, err := store.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	return collectWorkflowDefinitions(rows)
}

func (store *postgresStore) GetBySlug(ctx context.Context, networkID, slug, organizationID string) (*WorkflowDefinition, error) {
	sql := `
		SELECT` + workflowDefinitionSelectColumns + `
		FROM public.workflow_definitions wd` + user.JoinSQL("wd") + `
		WHERE wd.network_id = $1
			AND wd.slug = $2
			AND (wd.organization_id IS NULL OR wd.organization_id = NULLIF($3, '')::uuid)
		ORDER BY wd.organization_id NULLS LAST
		LIMIT 1`

	wf := &WorkflowDefinition{}
	err := scanWorkflowDefinition(store.db.QueryRow(ctx, sql, networkID, slug, organizationID), wf)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Workflow definition not found", err)
		}
		return nil, err
	}
	return wf, nil
}
