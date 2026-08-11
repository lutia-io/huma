package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/uuid"
)

type tokenRow struct {
	ID             string
	TokenHash      string
	FamilyID       string
	PrincipalType  string
	PrincipalID    string
	NetworkID      *string
	OrganizationID *string
	ExpiresAt      time.Time
	RevokedAt      *time.Time
	ReplacedBy     *string
	CreatedAt      time.Time
}

type identityUser struct {
	ID       string
	Email    string
	Password string
}

type identityOrganizationUser struct {
	ID             string
	Email          string
	Password       string
	OrganizationID string
	NetworkID      string
}

type identityOrganization struct {
	ID        string
	NetworkID string
}

type store interface {
	GetUserByEmail(ctx context.Context, email string) (*identityUser, error)
	GetOrganizationUserByEmail(ctx context.Context, email, networkID, organizationID string) (*identityOrganizationUser, error)
	GetOrganizationByID(ctx context.Context, id string) (*identityOrganization, error)
	NetworkExists(ctx context.Context, id string) (bool, error)
	InsertToken(ctx context.Context, row *tokenRow) error
	GetTokenByHash(ctx context.Context, hash string) (*tokenRow, error)
	RevokeFamily(ctx context.Context, familyID string, at time.Time) error
	MarkReplaced(ctx context.Context, id, replacedBy string, at time.Time) error
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (s *postgresStore) GetUserByEmail(ctx context.Context, email string) (*identityUser, error) {
	const sql = `
		SELECT id, email, password
		FROM public.users
		WHERE email = $1 AND deleted_at IS NULL`
	u := &identityUser{}
	err := s.db.QueryRow(ctx, sql, email).Scan(&u.ID, &u.Email, &u.Password)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("User not found", err)
		}
		return nil, err
	}
	return u, nil
}

func (s *postgresStore) GetOrganizationUserByEmail(ctx context.Context, email, networkID, organizationID string) (*identityOrganizationUser, error) {
	const sql = `
		SELECT id, email, password, organization_id, network_id
		FROM public.organization_users
		WHERE email = $1
			AND network_id = $2
			AND organization_id = $3
			AND deleted_at IS NULL`
	u := &identityOrganizationUser{}
	err := s.db.QueryRow(ctx, sql, email, networkID, organizationID).Scan(
		&u.ID, &u.Email, &u.Password, &u.OrganizationID, &u.NetworkID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Organization user not found", err)
		}
		return nil, err
	}
	return u, nil
}

func (s *postgresStore) GetOrganizationByID(ctx context.Context, id string) (*identityOrganization, error) {
	const sql = `
		SELECT id, network_id
		FROM public.organizations
		WHERE id = $1 AND deleted_at IS NULL`
	o := &identityOrganization{}
	err := s.db.QueryRow(ctx, sql, id).Scan(&o.ID, &o.NetworkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Organization not found", err)
		}
		return nil, err
	}
	return o, nil
}

func (s *postgresStore) NetworkExists(ctx context.Context, id string) (bool, error) {
	const sql = `SELECT 1 FROM public.networks WHERE id = $1 AND deleted_at IS NULL`
	var one int
	err := s.db.QueryRow(ctx, sql, id).Scan(&one)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *postgresStore) InsertToken(ctx context.Context, row *tokenRow) error {
	const sql = `
		INSERT INTO public.tokens (
			id, token_hash, family_id, principal_type, principal_id,
			network_id, organization_id, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := s.db.Exec(ctx, sql,
		row.ID,
		row.TokenHash,
		row.FamilyID,
		row.PrincipalType,
		row.PrincipalID,
		row.NetworkID,
		row.OrganizationID,
		row.ExpiresAt,
		row.CreatedAt,
	)
	return err
}

func (s *postgresStore) GetTokenByHash(ctx context.Context, hash string) (*tokenRow, error) {
	const sql = `
		SELECT id, token_hash, family_id, principal_type, principal_id,
			network_id, organization_id, expires_at, revoked_at, replaced_by, created_at
		FROM public.tokens
		WHERE token_hash = $1`
	row := &tokenRow{}
	err := s.db.QueryRow(ctx, sql, hash).Scan(
		&row.ID,
		&row.TokenHash,
		&row.FamilyID,
		&row.PrincipalType,
		&row.PrincipalID,
		&row.NetworkID,
		&row.OrganizationID,
		&row.ExpiresAt,
		&row.RevokedAt,
		&row.ReplacedBy,
		&row.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewUnauthorizedError("Invalid refresh token", err)
		}
		return nil, err
	}
	return row, nil
}

func (s *postgresStore) RevokeFamily(ctx context.Context, familyID string, at time.Time) error {
	const sql = `
		UPDATE public.tokens
		SET revoked_at = $2
		WHERE family_id = $1 AND revoked_at IS NULL`
	_, err := s.db.Exec(ctx, sql, familyID, at)
	return err
}

func (s *postgresStore) MarkReplaced(ctx context.Context, id, replacedBy string, at time.Time) error {
	const sql = `
		UPDATE public.tokens
		SET revoked_at = $2, replaced_by = $3
		WHERE id = $1`
	_, err := s.db.Exec(ctx, sql, id, at, replacedBy)
	return err
}

func hashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newRefreshToken() (raw string, err error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func newTokenID() (string, error) {
	return uuid.New()
}
