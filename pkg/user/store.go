package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, user *user) (string, error)
	GetByID(ctx context.Context, id string) (*user, error)
	GetByEmail(ctx context.Context, email string) (*user, error)
	Update(ctx context.Context, user *user) error
	UpdatePassword(ctx context.Context, id, hashedPassword string) error
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, user *user) (string, error) {
	const sql = `
		INSERT INTO public.users (
			first_name,
			last_name,
			email,
			password,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4,
			now(), now()
		)
		RETURNING id`

	err := store.db.QueryRow(ctx, sql,
		user.FirstName,
		user.LastName,
		user.Email,
		user.Password,
	).Scan(&user.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("User already exists", err)
		}
		return "", err
	}
	return user.ID, nil
}

func scanUser(row pgx.Row, u *user) error {
	return row.Scan(
		&u.ID,
		&u.FirstName,
		&u.LastName,
		&u.Email,
		&u.Password,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
	)
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*user, error) {
	const sql = `
		SELECT id, first_name, last_name, email, password, created_at, updated_at, deleted_at
		FROM public.users
		WHERE id = $1 AND deleted_at IS NULL`

	u := &user{}
	err := scanUser(store.db.QueryRow(ctx, sql, id), u)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("User not found", err)
		}
		return nil, err
	}
	return u, nil
}

func (store *postgresStore) GetByEmail(ctx context.Context, email string) (*user, error) {
	const sql = `
		SELECT id, first_name, last_name, email, password, created_at, updated_at, deleted_at
		FROM public.users
		WHERE email = $1 AND deleted_at IS NULL`

	u := &user{}
	err := scanUser(store.db.QueryRow(ctx, sql, email), u)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("User not found", err)
		}
		return nil, err
	}
	return u, nil
}

func (store *postgresStore) Update(ctx context.Context, user *user) error {
	const sql = `
		UPDATE public.users
		SET first_name = $2,
			last_name = $3,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql,
		user.ID,
		user.FirstName,
		user.LastName,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewNotFoundError("User not found", nil)
	}
	return nil
}

func (store *postgresStore) UpdatePassword(ctx context.Context, id, hashedPassword string) error {
	const sql = `
		UPDATE public.users
		SET password = $2,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql, id, hashedPassword)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewNotFoundError("User not found", nil)
	}
	return nil
}
