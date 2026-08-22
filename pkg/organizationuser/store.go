package organizationuser

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

const organizationUserSelectColumns = `
	ou.id,
	ou.first_name,
	ou.last_name,
	ou.email,
	ou.organization_id,
	ou.network_id,
	ou.created_at,
	ou.updated_at,
	ou.deleted_at,
	n.user_id`

type store interface {
	Insert(ctx context.Context, organizationUser *organizationUser) (string, error)
	GetByID(ctx context.Context, id string) (*organizationUser, error)
	GetByEmail(ctx context.Context, email, networkID, organizationID string) (*organizationUser, error)
	ListByUserID(ctx context.Context, userID string) ([]*organizationUser, error)
	ListByOrganization(ctx context.Context, networkID, organizationID string) ([]*organizationUser, error)
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
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return "", apperror.NewConflictError("Organization user already exists", err)
			case "23503":
				return "", apperror.NewBadRequestError("Invalid network or organization", err)
			}
		}
		return "", err
	}
	return organizationUser.ID, nil
}

func scanOrganizationUser(row pgx.Row, u *organizationUser) error {
	return row.Scan(
		&u.ID,
		&u.FirstName,
		&u.LastName,
		&u.Email,
		&u.OrganizationID,
		&u.NetworkID,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
		&u.UserID,
	)
}

func collectOrganizationUsers(rows pgx.Rows) ([]*organizationUser, error) {
	defer rows.Close()

	organizationUsers := make([]*organizationUser, 0)
	for rows.Next() {
		u := &organizationUser{}
		if err := scanOrganizationUser(rows, u); err != nil {
			return nil, err
		}
		organizationUsers = append(organizationUsers, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return organizationUsers, nil
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*organizationUser, error) {
	const sql = `
		SELECT` + organizationUserSelectColumns + `
		FROM public.organization_users ou
		JOIN public.networks n ON n.id = ou.network_id
		WHERE ou.id = $1
			AND ou.deleted_at IS NULL
			AND n.deleted_at IS NULL`

	u := &organizationUser{}
	err := scanOrganizationUser(store.db.QueryRow(ctx, sql, id), u)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Organization user not found", err)
		}
		return nil, err
	}
	return u, nil
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

func (store *postgresStore) ListByUserID(ctx context.Context, userID string) ([]*organizationUser, error) {
	const sql = `
		SELECT` + organizationUserSelectColumns + `
		FROM public.organization_users ou
		JOIN public.networks n ON n.id = ou.network_id
		WHERE n.user_id = $1
			AND ou.deleted_at IS NULL
			AND n.deleted_at IS NULL
		ORDER BY ou.created_at DESC`

	rows, err := store.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	return collectOrganizationUsers(rows)
}

func (store *postgresStore) ListByOrganization(ctx context.Context, networkID, organizationID string) ([]*organizationUser, error) {
	const sql = `
		SELECT` + organizationUserSelectColumns + `
		FROM public.organization_users ou
		JOIN public.networks n ON n.id = ou.network_id
		WHERE ou.network_id = $1
			AND ou.organization_id = $2
			AND ou.deleted_at IS NULL
			AND n.deleted_at IS NULL
		ORDER BY ou.created_at DESC`

	rows, err := store.db.Query(ctx, sql, networkID, organizationID)
	if err != nil {
		return nil, err
	}
	return collectOrganizationUsers(rows)
}
