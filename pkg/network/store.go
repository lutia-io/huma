package network

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, network *network) (string, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, network *network) (string, error) {
	const sql = `
		INSERT INTO public.networks (
			name,
			slug,
			user_id,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, now(), now()
		)
		RETURNING id`

	err := store.db.QueryRow(ctx, sql,
		network.Name,
		network.Slug,
		network.UserID,
	).Scan(&network.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Network already exists", err)
		}
		return "", err
	}
	return network.ID, nil
}
