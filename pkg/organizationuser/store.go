package organizationuser

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, organizationUser *organizationUser) (string, error)
	GetByEmail(ctx context.Context, email, networkID, organizationID string) (*organizationUser, error)
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
			network_id,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, now(), now()
		)
		RETURNING id`

	err := store.db.QueryRow(ctx, sql,
		organizationUser.FirstName,
		organizationUser.LastName,
		organizationUser.Email,
		organizationUser.Password,
		organizationUser.OrganizationID,
		organizationUser.NetworkID,
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

func (store *postgresStore) GetByEmail(ctx context.Context, email, networkID, organizationID string) (*organizationUser, error) {
	const sql = `
		SELECT id, first_name, last_name, email, password, organization_id, network_id,
			created_at, updated_at, deleted_at
		FROM public.organization_users
		WHERE email = $1
			AND network_id = $2
			AND organization_id = $3
			AND deleted_at IS NULL`

	u := &organizationUser{}
	err := store.db.QueryRow(ctx, sql, email, networkID, organizationID).Scan(
		&u.ID,
		&u.FirstName,
		&u.LastName,
		&u.Email,
		&u.Password,
		&u.OrganizationID,
		&u.NetworkID,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Organization user not found", err)
		}
		return nil, err
	}
	return u, nil
}
