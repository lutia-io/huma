package schema

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, schema *schema) (string, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, schema *schema) (string, error) {
	const sql = `
		INSERT INTO public.schemas (
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
		schema.Name,
		schema.Slug,
		schema.Active,
		schema.Internal,
		schema.Definition,
		schema.NetworkID,
		schema.UserID,
	).Scan(&schema.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("schema.store.Insert", "Schema already exists", err)
		}
		return "", err
	}
	return schema.ID, nil
}
