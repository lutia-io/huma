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

type store interface {
	Insert(ctx context.Context, schema *schema) (string, error)
	GetByID(ctx context.Context, id string) (*schema, error)
	ListByUserID(ctx context.Context, userID string) ([]*schema, error)
	ListVisibleToOrganization(ctx context.Context, networkID, organizationID string) ([]*schema, error)
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

func (store *postgresStore) ListByUserID(ctx context.Context, userID string) ([]*schema, error) {
	const sql = `
		SELECT` + schemaSelectColumns + `
		FROM public.schemas
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := store.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	return collectSchemas(rows)
}

func (store *postgresStore) ListVisibleToOrganization(ctx context.Context, networkID, organizationID string) ([]*schema, error) {
	const sql = `
		SELECT` + schemaSelectColumns + `
		FROM public.schemas
		WHERE network_id = $1
		  AND deleted_at IS NULL
		  AND (organization_id IS NULL OR organization_id = $2)
		ORDER BY created_at DESC`

	rows, err := store.db.Query(ctx, sql, networkID, organizationID)
	if err != nil {
		return nil, err
	}
	return collectSchemas(rows)
}
