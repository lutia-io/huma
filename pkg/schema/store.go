package schema

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

const schemaSelectColumns = `
	id, name, slug, active, internal, definition, network_id, organization_id,
	user_id, created_at, updated_at, deleted_at`

const schemaListSelectColumns = `
	s.id, s.name, s.slug, s.active, s.internal, s.definition, s.network_id, s.organization_id,
	s.user_id, s.created_at, s.updated_at, s.deleted_at`

type store interface {
	Insert(ctx context.Context, schema *schema) (string, error)
	Update(ctx context.Context, schema *schema) error
	GetByID(ctx context.Context, id string) (*schema, error)
	List(ctx context.Context, params listParams) (*listResult, error)
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
			organization_id,
			user_id,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
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
		schema.OrganizationID,
		schema.UserID,
	).Scan(&schema.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return "", apperror.NewConflictError("Schema already exists", err)
			case "23503":
				return "", apperror.NewBadRequestError("Invalid network or organization", err)
			}
		}
		return "", err
	}
	return schema.ID, nil
}

func (store *postgresStore) Update(ctx context.Context, schema *schema) error {
	const sql = `
		UPDATE public.schemas
		SET name = $2, slug = $3, active = $4, definition = $5, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql,
		schema.ID,
		schema.Name,
		schema.Slug,
		schema.Active,
		schema.Definition,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("Schema already exists", err)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewNotFoundError("Schema not found", nil)
	}
	return nil
}

func scanSchema(row pgx.Row, sch *schema) error {
	return row.Scan(
		&sch.ID,
		&sch.Name,
		&sch.Slug,
		&sch.Active,
		&sch.Internal,
		&sch.Definition,
		&sch.NetworkID,
		&sch.OrganizationID,
		&sch.UserID,
		&sch.CreatedAt,
		&sch.UpdatedAt,
		&sch.DeletedAt,
	)
}

func collectSchemas(rows pgx.Rows) ([]*schema, error) {
	defer rows.Close()

	schemas := make([]*schema, 0)
	for rows.Next() {
		sch := &schema{}
		if err := scanSchema(rows, sch); err != nil {
			return nil, err
		}
		schemas = append(schemas, sch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return schemas, nil
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*schema, error) {
	const sql = `
		SELECT` + schemaSelectColumns + `
		FROM public.schemas
		WHERE id = $1 AND deleted_at IS NULL`

	sch := &schema{}
	err := scanSchema(store.db.QueryRow(ctx, sql, id), sch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Schema not found", err)
		}
		return nil, err
	}
	return sch, nil
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
	items, err := collectSchemas(rows)
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
