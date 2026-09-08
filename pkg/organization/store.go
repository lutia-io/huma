package organization

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/user"
)

const organizationSelectColumns = `
	o.id, o.name, o.slug, o.network_id, o.user_id, o.created_at, o.updated_at, o.deleted_at,
	` + user.SelectSQL

var organizationListSelectColumns = organizationSelectColumns

type store interface {
	Insert(ctx context.Context, organization *organization) (string, error)
	Update(ctx context.Context, organization *organization) error
	Delete(ctx context.Context, id, updatedBy string) error
	GetByID(ctx context.Context, id string) (*organization, error)
	List(ctx context.Context, params listParams) (*listResult, error)
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
			created_by,
			updated_by,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			now(), now()
		)
		RETURNING id`

	err := store.db.QueryRow(ctx, sql,
		organization.Name,
		organization.Slug,
		organization.NetworkID,
		organization.UserID,
		organization.CreatedBy.ID,
		organization.UpdatedBy.ID,
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
		SET name = $2, slug = $3, updated_by = $4, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql, organization.ID, organization.Name, organization.Slug, organization.UpdatedBy.ID)
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

func (store *postgresStore) Delete(ctx context.Context, id, updatedBy string) error {
	const sql = `
		UPDATE public.organizations
		SET deleted_at = now(), updated_at = now(), updated_by = $2
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql, id, updatedBy)
	if err != nil {
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
		&o.CreatedBy.ID,
		&o.CreatedBy.FirstName,
		&o.CreatedBy.LastName,
		&o.CreatedBy.Email,
		&o.UpdatedBy.ID,
		&o.UpdatedBy.FirstName,
		&o.UpdatedBy.LastName,
		&o.UpdatedBy.Email,
	)
}

func collectOrganizations(rows pgx.Rows) ([]*organization, error) {
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

func (store *postgresStore) GetByID(ctx context.Context, id string) (*organization, error) {
	sql := `
		SELECT` + organizationSelectColumns + `
		FROM public.organizations o` + user.JoinSQL("o") + `
		WHERE o.id = $1 AND o.deleted_at IS NULL`

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

func (store *postgresStore) List(ctx context.Context, params listParams) (*listResult, error) {
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)

	var total int
	if err := store.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := store.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, err
	}
	items, err := collectOrganizations(rows)
	if err != nil {
		return nil, err
	}

	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = total
	}

	return &listResult{
		Items:    items,
		Total:    total,
		Page:     params.Page,
		PageSize: pageSize,
	}, nil
}
