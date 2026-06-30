package record

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type store interface {
	Insert(ctx context.Context, record *record) (string, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, record *record) (string, error) {
	const sql = `
		INSERT INTO public.records (
			data,
			schema_id,
			organization_id,
			organization_user_id,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4,
			now(), now()
		)
		RETURNING id`

	err := store.db.QueryRow(ctx, sql,
		record.Data,
		record.SchemaID,
		record.OrganizationID,
		record.OrganizationUserID,
	).Scan(&record.ID)
	if err != nil {
		return "", err
	}
	return record.ID, nil
}
