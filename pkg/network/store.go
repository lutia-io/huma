package network

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/user"
)

var (
	networkSelectColumns = `
	n.id, n.name, n.slug, n.user_id, n.created_at, n.updated_at, n.deleted_at,
	` + user.SelectSQL

	networkFromSQL = `
	FROM public.networks n` + user.JoinSQL("n")
)

type store interface {
	Insert(ctx context.Context, network *network) (string, error)
	Update(ctx context.Context, network *network) error
	Delete(ctx context.Context, id, updatedBy string) error
	GetByID(ctx context.Context, id string) (*network, error)
	ListByUserID(ctx context.Context, userID string) ([]*network, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, network *network) (string, error) {
	const sql = `
		INSERT INTO public.networks (
			name,
			slug,
			user_id,
			created_by,
			updated_by,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, now(), now()
		)
		RETURNING id`

	err := store.db.QueryRow(ctx, sql,
		network.Name,
		network.Slug,
		network.UserID,
		network.CreatedBy.ID,
		network.UpdatedBy.ID,
	).Scan(&network.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Network already exists", err)
		}
		return "", err
	}
	return network.ID, nil
}

func (store *postgresStore) Update(ctx context.Context, network *network) error {
	const sql = `
		UPDATE public.networks
		SET name = $2, slug = $3, updated_by = $4, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql, network.ID, network.Name, network.Slug, network.UpdatedBy.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("Network already exists", err)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewNotFoundError("Network not found", nil)
	}
	return nil
}

func (store *postgresStore) Delete(ctx context.Context, id, updatedBy string) error {
	const sql = `
		UPDATE public.networks
		SET deleted_at = now(), updated_at = now(), updated_by = $2
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql, id, updatedBy)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewNotFoundError("Network not found", nil)
	}
	return nil
}

func scanNetwork(row pgx.Row, n *network) error {
	return row.Scan(
		&n.ID,
		&n.Name,
		&n.Slug,
		&n.UserID,
		&n.CreatedAt,
		&n.UpdatedAt,
		&n.DeletedAt,
		&n.CreatedBy.ID,
		&n.CreatedBy.FirstName,
		&n.CreatedBy.LastName,
		&n.CreatedBy.Email,
		&n.UpdatedBy.ID,
		&n.UpdatedBy.FirstName,
		&n.UpdatedBy.LastName,
		&n.UpdatedBy.Email,
	)
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*network, error) {
	sql := `
		SELECT` + networkSelectColumns + networkFromSQL + `
		WHERE n.id = $1 AND n.deleted_at IS NULL`

	n := &network{}
	err := scanNetwork(store.db.QueryRow(ctx, sql, id), n)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Network not found", err)
		}
		return nil, err
	}
	return n, nil
}

func (store *postgresStore) ListByUserID(ctx context.Context, userID string) ([]*network, error) {
	sql := `
		SELECT` + networkSelectColumns + networkFromSQL + `
		WHERE n.user_id = $1 AND n.deleted_at IS NULL
		ORDER BY n.created_at DESC`

	rows, err := store.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	networks := make([]*network, 0)
	for rows.Next() {
		n := &network{}
		if err := scanNetwork(rows, n); err != nil {
			return nil, err
		}
		networks = append(networks, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return networks, nil
}
