package record

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store interface {
	Insert(ctx context.Context, rec *Record) (string, error)
	Get(ctx context.Context, recordID string) (*Record, bool, error)
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
			idempotency_key,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, NULLIF($5, ''),
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
		return "", err
	}
	return id, nil
}

func (store *postgresStore) Get(ctx context.Context, recordID string) (*Record, bool, error) {
	const sql = `
		SELECT id, schema_id, data
		FROM public.records
		WHERE id = $1
		  AND deleted_at IS NULL`

	rec := &Record{}
	err := store.db.QueryRow(ctx, sql, recordID).Scan(&rec.ID, &rec.SchemaID, &rec.Data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return rec, true, nil
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
