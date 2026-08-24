package file

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

const fileSelectColumns = `
	f.id,
	f.filename,
	f.content_type,
	f.size_bytes,
	f.organization_id,
	f.organization_user_id,
	f.network_id,
	f.created_at,
	f.updated_at,
	f.deleted_at,
	n.user_id`

type store interface {
	// Insert creates file metadata. created is false when an existing row
	// was resolved via idempotency_key within the same network.
	Insert(ctx context.Context, f *File) (id string, created bool, err error)
	UpdateSize(ctx context.Context, fileID string, sizeBytes int64) error
	Get(ctx context.Context, fileID string) (*File, bool, error)
	GetByID(ctx context.Context, id string) (*File, error)
	List(ctx context.Context, params listParams) (*listResult, error)
	Delete(ctx context.Context, fileID string) error
	SoftDelete(ctx context.Context, fileID string) (bool, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

// Insert creates file metadata. When IdempotencyKey is set, a conflicting
// (network_id, idempotency_key) resolves to the existing file instead of
// inserting a duplicate. created is false when the existing row was returned.
func (store *postgresStore) Insert(ctx context.Context, f *File) (string, bool, error) {
	const sql = `
		INSERT INTO public.files (
			filename,
			content_type,
			size_bytes,
			organization_id,
			organization_user_id,
			network_id,
			idempotency_key,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, NULLIF($7, ''),
			now(), now()
		)
		ON CONFLICT (network_id, idempotency_key)
			WHERE idempotency_key IS NOT NULL AND deleted_at IS NULL
			DO NOTHING
		RETURNING id`

	var id string
	err := store.db.QueryRow(ctx, sql,
		f.Filename,
		f.ContentType,
		f.SizeBytes,
		f.OrganizationID,
		f.OrganizationUserID,
		f.NetworkID,
		f.IdempotencyKey,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		const selectSQL = `
			SELECT id FROM public.files
			WHERE network_id = $1
			  AND idempotency_key = $2
			  AND deleted_at IS NULL`
		if err := store.db.QueryRow(ctx, selectSQL, f.NetworkID, f.IdempotencyKey).Scan(&id); err != nil {
			return "", false, err
		}
		return id, false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

func (store *postgresStore) UpdateSize(ctx context.Context, fileID string, sizeBytes int64) error {
	const sql = `
		UPDATE public.files
		SET size_bytes = $2, updated_at = now()
		WHERE id = $1
		  AND deleted_at IS NULL`
	_, err := store.db.Exec(ctx, sql, fileID, sizeBytes)
	return err
}

func scanFile(row pgx.Row, f *File) error {
	return row.Scan(
		&f.ID,
		&f.Filename,
		&f.ContentType,
		&f.SizeBytes,
		&f.OrganizationID,
		&f.OrganizationUserID,
		&f.NetworkID,
		&f.CreatedAt,
		&f.UpdatedAt,
		&f.DeletedAt,
		&f.UserID,
	)
}

func collectFiles(rows pgx.Rows) ([]*File, error) {
	defer rows.Close()

	files := make([]*File, 0)
	for rows.Next() {
		f := &File{}
		if err := scanFile(rows, f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func (store *postgresStore) Get(ctx context.Context, fileID string) (*File, bool, error) {
	const sql = `
		SELECT id, filename, content_type, size_bytes,
		       organization_id, organization_user_id, network_id,
		       created_at, updated_at
		FROM public.files
		WHERE id = $1
		  AND deleted_at IS NULL`

	f := &File{}
	err := store.db.QueryRow(ctx, sql, fileID).Scan(
		&f.ID,
		&f.Filename,
		&f.ContentType,
		&f.SizeBytes,
		&f.OrganizationID,
		&f.OrganizationUserID,
		&f.NetworkID,
		&f.CreatedAt,
		&f.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return f, true, nil
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*File, error) {
	const sql = `
		SELECT` + fileSelectColumns + `
		FROM public.files f
		JOIN public.networks n ON n.id = f.network_id
		WHERE f.id = $1
			AND f.deleted_at IS NULL
			AND n.deleted_at IS NULL`

	f := &File{}
	err := scanFile(store.db.QueryRow(ctx, sql, id), f)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("File not found", err)
		}
		return nil, err
	}
	return f, nil
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
	items, err := collectFiles(rows)
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

func (store *postgresStore) Delete(ctx context.Context, fileID string) error {
	const sql = `DELETE FROM public.files WHERE id = $1`
	_, err := store.db.Exec(ctx, sql, fileID)
	return err
}

func (store *postgresStore) SoftDelete(ctx context.Context, fileID string) (bool, error) {
	const sql = `
		UPDATE public.files
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1
		  AND deleted_at IS NULL`
	tag, err := store.db.Exec(ctx, sql, fileID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
