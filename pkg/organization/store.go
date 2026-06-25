package organization

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, organization *organization) error
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, organization *organization) error {
	const sql = `
		INSERT INTO public.organizations (
			name,
			slug,
			network_id,
			user_id,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			now(), now()
		)`

	_, err := store.db.Exec(ctx, sql,
		organization.Name,
		organization.Slug,
		organization.NetworkID,
		organization.UserID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("organization.store.Insert", "Organization already exists", err)
		}
		return err
	}
	return nil
}
