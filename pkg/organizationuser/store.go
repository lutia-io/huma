package organizationuser

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, organizationUser *organizationUser) (string, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, organizationUser *organizationUser) (string, error) {
	const sql = `
		INSERT INTO public.organization_users (
			first_name,
			last_name,
			email,
			password,
			organization_id,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			now(), now()
		)
		RETURNING id`

	err := store.db.QueryRow(ctx, sql,
		organizationUser.FirstName,
		organizationUser.LastName,
		organizationUser.Email,
		organizationUser.Password,
		organizationUser.OrganizationID,
	).Scan(&organizationUser.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Organization user already exists", err)
		}
		return "", err
	}
	return organizationUser.ID, nil
}
