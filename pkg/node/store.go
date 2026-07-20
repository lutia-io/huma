package node

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, node *nodeDefinition) (string, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, node *nodeDefinition) (string, error) {
	const sql = `
		INSERT INTO public.node_definitions (
			name,
			slug,
			active,
			internal,
			type,
			definition,
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
		node.Name,
		node.Slug,
		node.Active,
		node.Internal,
		node.Type,
		node.Definition,
		node.NetworkID,
		node.UserID,
	).Scan(&node.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Node definition already exists", err)
		}
		return "", err
	}
	return node.ID, nil
}
