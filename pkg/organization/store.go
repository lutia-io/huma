package organization

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

const organizationSelectColumns = `
	id, name, slug, network_id, user_id, created_at, updated_at, deleted_at`

type store interface {
	Insert(ctx context.Context, organization *organization) (string, error)
	Update(ctx context.Context, organization *organization) error
	GetByID(ctx context.Context, id string) (*organization, error)
	ListByUserID(ctx context.Context, userID string) ([]*organization, error)
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

func (store *postgresStore) Update(ctx context.Context, organization *organization) error {
	const sql = `
		UPDATE public.organizations
		SET name = $2, slug = $3, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql, organization.ID, organization.Name, organization.Slug)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("Organization already exists", err)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewNotFoundError("Organization not found", nil)
	}
	return nil
}

func scanOrganization(row pgx.Row, o *organization) error {
	return row.Scan(
		&o.ID,
		&o.Name,
		&o.Slug,
		&o.NetworkID,
		&o.UserID,
		&o.CreatedAt,
		&o.UpdatedAt,
		&o.DeletedAt,
	)
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*organization, error) {
	const sql = `
		SELECT` + organizationSelectColumns + `
		FROM public.organizations
		WHERE id = $1 AND deleted_at IS NULL`

	o := &organization{}
	err := scanOrganization(store.db.QueryRow(ctx, sql, id), o)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Organization not found", err)
		}
		return nil, err
	}
	return o, nil
}

func (store *postgresStore) ListByUserID(ctx context.Context, userID string) ([]*organization, error) {
	const sql = `
		SELECT` + organizationSelectColumns + `
		FROM public.organizations
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := store.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	organizations := make([]*organization, 0)
	for rows.Next() {
		o := &organization{}
		if err := scanOrganization(rows, o); err != nil {
			return nil, err
		}
		organizations = append(organizations, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return organizations, nil
}
