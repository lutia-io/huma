package pipeline

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, pipeline *pipelineDefinition) (string, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, pipeline *pipelineDefinition) (string, error) {
	const sql = `
		INSERT INTO public.pipeline_definitions (
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
		pipeline.Name,
		pipeline.Slug,
		pipeline.Active,
		pipeline.Internal,
		pipeline.Definition,
		pipeline.SchemaID,
		pipeline.NetworkID,
		pipeline.UserID,
	).Scan(&pipeline.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Pipeline definition already exists", err)
		}
		return "", err
	}
	return pipeline.ID, nil
}
