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
	GetByEmail(ctx context.Context, email string) (*user, error)
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

func (store *postgresStore) GetByEmail(ctx context.Context, email string) (*user, error) {
	const sql = `
		SELECT id, first_name, last_name, email, password, created_at, updated_at, deleted_at
		FROM public.users
		WHERE email = $1 AND deleted_at IS NULL`

	u := &user{}
	err := store.db.QueryRow(ctx, sql, email).Scan(
		&u.ID,
		&u.FirstName,
		&u.LastName,
		&u.Email,
		&u.Password,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("User not found", err)
		}
		return nil, err
	}
	return u, nil
}
