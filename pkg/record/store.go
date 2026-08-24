package record

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

const recordSelectColumns = `
	r.id,
	r.data,
	r.schema_id,
	r.organization_id,
	r.organization_user_id,
	r.network_id,
	r.created_at,
	r.updated_at,
	r.deleted_at,
	n.user_id`

type store interface {
	Insert(ctx context.Context, rec *Record) (string, error)
	Get(ctx context.Context, recordID string) (*Record, bool, error)
	GetByID(ctx context.Context, id string) (*Record, error)
	List(ctx context.Context, params listParams) (*listResult, error)
	UpdateData(ctx context.Context, recordID string, data json.RawMessage) (bool, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

// Insert creates a record. When IdempotencyKey is set, a conflicting key
// resolves to the existing record instead of inserting a duplicate.
func (store *postgresStore) Insert(ctx context.Context, rec *Record) (string, error) {
	const sql = `
		INSERT INTO public.records (
			data,
			schema_id,
			organization_id,
			organization_user_id,
			network_id,
			idempotency_key,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, NULLIF($6, ''),
			now(), now()
		)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
		RETURNING id`

	var id string
	err := store.db.QueryRow(ctx, sql,
		rec.Data,
		rec.SchemaID,
		rec.OrganizationID,
		rec.OrganizationUserID,
		rec.NetworkID,
		rec.IdempotencyKey,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		const selectSQL = `SELECT id FROM public.records WHERE idempotency_key = $1`
		if err := store.db.QueryRow(ctx, selectSQL, rec.IdempotencyKey).Scan(&id); err != nil {
			return "", err
		}
		return id, nil
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return "", apperror.NewBadRequestError("Invalid schema, organization, or network", err)
		}
		return "", err
	}
	return id, nil
}

func scanRecord(row pgx.Row, rec *Record) error {
	return row.Scan(
		&rec.ID,
		&rec.Data,
		&rec.SchemaID,
		&rec.OrganizationID,
		&rec.OrganizationUserID,
		&rec.NetworkID,
		&rec.CreatedAt,
		&rec.UpdatedAt,
		&rec.DeletedAt,
		&rec.UserID,
	)
}

func collectRecords(rows pgx.Rows) ([]*Record, error) {
	defer rows.Close()

	records := make([]*Record, 0)
	for rows.Next() {
		rec := &Record{}
		if err := scanRecord(rows, rec); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (store *postgresStore) Get(ctx context.Context, recordID string) (*Record, bool, error) {
	const sql = `
		SELECT id, data, schema_id, organization_id, organization_user_id, network_id,
			created_at, updated_at, deleted_at
		FROM public.records
		WHERE id = $1
		  AND deleted_at IS NULL`

	rec := &Record{}
	err := store.db.QueryRow(ctx, sql, recordID).Scan(
		&rec.ID,
		&rec.Data,
		&rec.SchemaID,
		&rec.OrganizationID,
		&rec.OrganizationUserID,
		&rec.NetworkID,
		&rec.CreatedAt,
		&rec.UpdatedAt,
		&rec.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return rec, true, nil
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*Record, error) {
	const sql = `
		SELECT` + recordSelectColumns + `
		FROM public.records r
		JOIN public.networks n ON n.id = r.network_id
		WHERE r.id = $1
			AND r.deleted_at IS NULL
			AND n.deleted_at IS NULL`

	rec := &Record{}
	err := scanRecord(store.db.QueryRow(ctx, sql, id), rec)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Record not found", err)
		}
		return nil, err
	}
	return rec, nil
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
	items, err := collectRecords(rows)
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

func (store *postgresStore) UpdateData(ctx context.Context, recordID string, data json.RawMessage) (bool, error) {
	const sql = `
		UPDATE public.records
		SET data = $2, updated_at = now()
		WHERE id = $1
		  AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql, recordID, data)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
