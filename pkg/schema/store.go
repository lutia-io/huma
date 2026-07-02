package schema

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, schema *schema) (string, error)
	GetByID(ctx context.Context, id string) (*schema, error)
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
			return "", apperror.NewConflictError("Schema already exists", err)
		}
		return "", err
	}
	return schema.ID, nil
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*schema, error) {
	const sql = `
		SELECT id, definition, active
		FROM public.schemas
		WHERE id = $1
		  AND deleted_at IS NULL`

	var sch schema
	err := store.db.QueryRow(ctx, sql, id).Scan(&sch.ID, &sch.Definition, &sch.Active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Schema not found", err)
		}
		return nil, err
	}
	return &sch, nil
}
