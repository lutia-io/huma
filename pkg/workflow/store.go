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
			network_id,
			user_id,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			now(), now()
		)
		RETURNING id`

	err := store.db.QueryRow(ctx, sql,
		workflow.Name,
		workflow.Slug,
		workflow.Active,
		workflow.Internal,
		workflow.Definition,
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
