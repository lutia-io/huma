package workflow

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, workflow *workflowDefinition) (string, error)
	ListActiveBySchemaID(ctx context.Context, schemaID string) ([]*workflowDefinition, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, workflow *workflowDefinition) (string, error) {
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
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			now(), now()
		)
		RETURNING id`

	err := store.db.QueryRow(ctx, sql,
		workflow.Name,
		workflow.Slug,
		workflow.Active,
		workflow.Internal,
		workflow.Definition,
		workflow.SchemaID,
		workflow.NetworkID,
		workflow.UserID,
	).Scan(&workflow.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Workflow already exists", err)
		}
		return "", err
	}
	return workflow.ID, nil
}

func (store *postgresStore) ListActiveBySchemaID(ctx context.Context, schemaID string) ([]*workflowDefinition, error) {
	const sql = `
		SELECT
			id,
			name,
			slug,
			active,
			internal,
			definition,
			schema_id,
			network_id,
			user_id,
			created_at,
			updated_at,
			deleted_at
		FROM public.workflow_definitions
		WHERE schema_id = $1
			AND active = true
			AND deleted_at IS NULL`

	rows, err := store.db.Query(ctx, sql, schemaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []*workflowDefinition
	for rows.Next() {
		wf := &workflowDefinition{}
		if err := rows.Scan(
			&wf.ID,
			&wf.Name,
			&wf.Slug,
			&wf.Active,
			&wf.Internal,
			&wf.Definition,
			&wf.SchemaID,
			&wf.NetworkID,
			&wf.UserID,
			&wf.CreatedAt,
			&wf.UpdatedAt,
			&wf.DeletedAt,
		); err != nil {
			return nil, err
		}
		workflows = append(workflows, wf)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return workflows, nil
}
