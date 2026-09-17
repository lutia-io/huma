package authz

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lutia-io/huma/pkg/uuid"
)

func (s *postgresStore) SeedNetwork(ctx context.Context, networkID, creatorUserID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	ownersGroupID, err := insertGroup(ctx, tx, networkID, "Owners", SlugOwners,
		"Full control of this network, including access configuration.", true, now)
	if err != nil {
		return err
	}
	membersGroupID, err := insertGroup(ctx, tx, networkID, "Members", SlugMembers,
		"Can view network configuration. Cannot change definitions or access.", true, now)
	if err != nil {
		return err
	}
	ownersPermID, err := insertNetworkPermission(ctx, tx, networkID, "Owners", SlugOwners,
		"Full control of this network, including access configuration.", true, allNetworkActions(), now)
	if err != nil {
		return err
	}
	membersPermID, err := insertNetworkPermission(ctx, tx, networkID, "Members", SlugMembers,
		"Can view network configuration. Cannot change definitions or access.", true, readNetworkActions(), now)
	if err != nil {
		return err
	}
	if err := attachNetworkPermission(ctx, tx, ownersGroupID, ownersPermID); err != nil {
		return err
	}
	if err := attachNetworkPermission(ctx, tx, membersGroupID, membersPermID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.network_group_members (group_id, user_id, created_at)
		VALUES ($1, $2, $3)`, ownersGroupID, creatorUserID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *postgresStore) SeedOrganization(ctx context.Context, organizationID, networkID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	everyoneID, err := insertOrgGroup(ctx, tx, organizationID, networkID, "Everyone", SlugEveryone,
		"All organization users. Full access to records and files until Access is tightened.", true, now)
	if err != nil {
		return err
	}
	adminsID, err := insertOrgGroup(ctx, tx, organizationID, networkID, "Admins", SlugAdmins,
		"Can invite and manage organization users.", true, now)
	if err != nil {
		return err
	}
	everyonePermID, err := insertOrgPermission(ctx, tx, organizationID, networkID, "Everyone", SlugEveryone,
		"All organization users. Full access to records and files until Access is tightened.", true,
		[]Grant{
			{Resource: ResourceRecord, Actions: []string{ActionCreate, ActionRead, ActionUpdate, ActionDelete}},
			{Resource: ResourceFile, Actions: []string{ActionCreate, ActionRead, ActionDelete}},
		}, now)
	if err != nil {
		return err
	}
	adminsPermID, err := insertOrgPermission(ctx, tx, organizationID, networkID, "Admins", SlugAdmins,
		"Can invite and manage organization users.", true,
		[]Grant{
			{Resource: ResourceOrganizationUser, Actions: []string{ActionCreate, ActionRead, ActionUpdate}},
		}, now)
	if err != nil {
		return err
	}
	if err := attachOrgPermission(ctx, tx, everyoneID, everyonePermID); err != nil {
		return err
	}
	if err := attachOrgPermission(ctx, tx, adminsID, adminsPermID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertGroup(ctx context.Context, tx pgx.Tx, networkID, name, slug, description string, system bool, now time.Time) (string, error) {
	id, err := uuid.New()
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO public.network_groups (id, network_id, name, slug, description, system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`,
		id, networkID, name, slug, description, system, now)
	return id, err
}

func insertNetworkPermission(ctx context.Context, tx pgx.Tx, networkID, name, slug, description string, system bool, grants []Grant, now time.Time) (string, error) {
	id, err := uuid.New()
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO public.network_permissions (id, network_id, name, slug, description, system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`,
		id, networkID, name, slug, description, system, now)
	if err != nil {
		return "", err
	}
	for _, g := range grants {
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.network_permission_grants (id, permission_id, resource, actions, created_at)
			VALUES ($1, $2, $3, $4, $5)`,
			uuid.MustNew(), id, g.Resource, g.Actions, now); err != nil {
			return "", err
		}
	}
	return id, nil
}

func attachNetworkPermission(ctx context.Context, tx pgx.Tx, groupID, permissionID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO public.network_group_permissions (group_id, permission_id)
		VALUES ($1, $2)`, groupID, permissionID)
	return err
}

func insertOrgGroup(ctx context.Context, tx pgx.Tx, organizationID, networkID, name, slug, description string, system bool, now time.Time) (string, error) {
	id, err := uuid.New()
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO public.organization_groups (id, organization_id, network_id, name, slug, description, system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`,
		id, organizationID, networkID, name, slug, description, system, now)
	return id, err
}

func insertOrgPermission(ctx context.Context, tx pgx.Tx, organizationID, networkID, name, slug, description string, system bool, grants []Grant, now time.Time) (string, error) {
	id, err := uuid.New()
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO public.organization_permissions (id, organization_id, network_id, name, slug, description, system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`,
		id, organizationID, networkID, name, slug, description, system, now)
	if err != nil {
		return "", err
	}
	for _, g := range grants {
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.organization_permission_grants (id, permission_id, resource, actions, created_at)
			VALUES ($1, $2, $3, $4, $5)`,
			uuid.MustNew(), id, g.Resource, g.Actions, now); err != nil {
			return "", err
		}
	}
	return id, nil
}

func attachOrgPermission(ctx context.Context, tx pgx.Tx, groupID, permissionID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO public.organization_group_permissions (group_id, permission_id)
		VALUES ($1, $2)`, groupID, permissionID)
	return err
}
