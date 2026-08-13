package organization

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, organization *organization) (string, error)
	GetByID(ctx context.Context, id string) (*organization, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, organization *organization) (string, error) {
	const sql = `
		INSERT INTO public.organizations (
			name,
			slug,
			network_id,
			user_id,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4,
			now(), now()
		)
		RETURNING id`

	err := store.db.QueryRow(ctx, sql,
		organization.Name,
		organization.Slug,
		organization.NetworkID,
		organization.UserID,
	).Scan(&organization.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Organization already exists", err)
		}
		return "", err
	}
	return organization.ID, nil
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*organization, error) {
	const sql = `
		SELECT id, name, slug, network_id, user_id, created_at, updated_at, deleted_at
		FROM public.organizations
		WHERE id = $1 AND deleted_at IS NULL`

	o := &organization{}
	err := store.db.QueryRow(ctx, sql, id).Scan(
		&o.ID,
		&o.Name,
		&o.Slug,
		&o.NetworkID,
		&o.UserID,
		&o.CreatedAt,
		&o.UpdatedAt,
		&o.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Organization not found", err)
		}
		return nil, err
	}
	return o, nil
}
