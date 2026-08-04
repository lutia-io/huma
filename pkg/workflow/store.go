package workflow

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, workflow *WorkflowDefinition) (string, error)
	ListActiveBySchemaID(ctx context.Context, schemaID string) ([]*WorkflowDefinition, error)
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
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			now(), now()
		)
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Workflow definition already exists", err)
		}
		return "", err
	}
	return workflow.ID, nil
}

// ListActiveBySchemaID returns the definitions the engine's intake considers
// for a record event on the given schema.
func (store *postgresStore) ListActiveBySchemaID(ctx context.Context, schemaID string) ([]*WorkflowDefinition, error) {
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

	var workflows []*WorkflowDefinition
	for rows.Next() {
		wf := &WorkflowDefinition{}
		var defJSON []byte
		if err := rows.Scan(
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
			return nil, err
		}
		if err := json.Unmarshal(defJSON, &wf.Definition); err != nil {
			return nil, err
		}
		workflows = append(workflows, wf)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return workflows, nil
}
