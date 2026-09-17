package authz

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	NetworkCreatedBy(ctx context.Context, networkID string) (string, error)
	IsNetworkMember(ctx context.Context, userID, networkID string) (bool, error)
	NetworkGrants(ctx context.Context, userID, networkID string) ([]Grant, error)
	OrganizationGrants(ctx context.Context, organizationUserID, organizationID string) ([]Grant, error)
	SeedNetwork(ctx context.Context, networkID, creatorUserID string) error
	SeedOrganization(ctx context.Context, organizationID, networkID string) error
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (s *postgresStore) NetworkCreatedBy(ctx context.Context, networkID string) (string, error) {
	const sql = `SELECT created_by FROM public.networks WHERE id = $1`
	var createdBy string
	err := s.db.QueryRow(ctx, sql, networkID).Scan(&createdBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperror.NewNotFoundError("Network not found", err)
		}
		return "", err
	}
	return createdBy, nil
}

func (s *postgresStore) IsNetworkMember(ctx context.Context, userID, networkID string) (bool, error) {
	sql := `SELECT ` + MemberFilter("$2", "$1")
	var ok bool
	err := s.db.QueryRow(ctx, sql, userID, networkID).Scan(&ok)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (s *postgresStore) NetworkGrants(ctx context.Context, userID, networkID string) ([]Grant, error) {
	const sql = `
		SELECT npg.resource, npg.actions, COALESCE(npg.resource_id::text, '')
		FROM public.network_group_members ngm
		JOIN public.network_groups ng ON ng.id = ngm.group_id AND ng.deleted_at IS NULL
		JOIN public.network_group_permissions ngp ON ngp.group_id = ng.id
		JOIN public.network_permissions np ON np.id = ngp.permission_id AND np.deleted_at IS NULL
		JOIN public.network_permission_grants npg ON npg.permission_id = np.id
		WHERE ng.network_id = $1 AND ngm.user_id = $2`
	rows, err := s.db.Query(ctx, sql, networkID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := make([]Grant, 0)
	for rows.Next() {
		var g Grant
		if err := rows.Scan(&g.Resource, &g.Actions, &g.ResourceID); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

func (s *postgresStore) OrganizationGrants(ctx context.Context, organizationUserID, organizationID string) ([]Grant, error) {
	const sql = `
		SELECT opg.id, opg.resource, opg.actions, COALESCE(opg.resource_id::text, ''), COALESCE(opg.schema_id::text, '')
		FROM public.organization_groups og
		JOIN public.organization_group_permissions ogp ON ogp.group_id = og.id
		JOIN public.organization_permissions op ON op.id = ogp.permission_id AND op.deleted_at IS NULL
		JOIN public.organization_permission_grants opg ON opg.permission_id = op.id
		WHERE og.organization_id = $1
			AND og.deleted_at IS NULL
			AND (
				(og.system AND og.slug = $3)
				OR EXISTS (
					SELECT 1 FROM public.organization_group_members ogm
					WHERE ogm.group_id = og.id AND ogm.organization_user_id = $2
				)
			)`
	rows, err := s.db.Query(ctx, sql, organizationID, organizationUserID, SlugEveryone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type row struct {
		id string
		g  Grant
	}
	collected := make([]row, 0)
	ids := make([]string, 0)
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.g.Resource, &r.g.Actions, &r.g.ResourceID, &r.g.SchemaID); err != nil {
			return nil, err
		}
		collected = append(collected, r)
		ids = append(ids, r.id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	fields, err := s.organizationFields(ctx, ids)
	if err != nil {
		return nil, err
	}
	grants := make([]Grant, 0, len(collected))
	for _, r := range collected {
		r.g.Fields = fields[r.id]
		grants = append(grants, r.g)
	}
	return grants, nil
}

func (s *postgresStore) organizationFields(ctx context.Context, grantIDs []string) (map[string][]FieldGrant, error) {
	out := make(map[string][]FieldGrant)
	if len(grantIDs) == 0 {
		return out, nil
	}
	const sql = `
		SELECT grant_id, field_name, access
		FROM public.organization_permission_fields
		WHERE grant_id = ANY($1)`
	rows, err := s.db.Query(ctx, sql, grantIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, access string
		if err := rows.Scan(&id, &name, &access); err != nil {
			return nil, err
		}
		out[id] = append(out[id], FieldGrant{Name: name, Access: access})
	}
	return out, rows.Err()
}
