package organizationaccess

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/uuid"
)

type store interface {
	InsertGroup(ctx context.Context, g *group) (string, error)
	UpdateGroup(ctx context.Context, g *group) error
	DeleteGroup(ctx context.Context, id string) error
	GetGroup(ctx context.Context, id string) (*group, error)
	ListGroups(ctx context.Context, organizationID string) ([]*group, error)
	AddMember(ctx context.Context, groupID, organizationUserID string) error
	RemoveMember(ctx context.Context, groupID, organizationUserID string) error
	OrganizationUserScope(ctx context.Context, organizationUserID string) (organizationID string, internal bool, err error)
	InsertPermission(ctx context.Context, p *permission) (string, error)
	UpdatePermission(ctx context.Context, p *permission) error
	DeletePermission(ctx context.Context, id string) error
	GetPermission(ctx context.Context, id string) (*permission, error)
	ListPermissions(ctx context.Context, organizationID string) ([]*permission, error)
	AssignPermission(ctx context.Context, groupID, permissionID string) error
	UnassignPermission(ctx context.Context, groupID, permissionID string) error
}

type postgresStore struct {
	db *pgxpool.Pool
}

func newPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

const groupSelect = `
	g.id, g.organization_id, g.network_id, g.name, g.slug, g.description, g.system, g.created_at, g.updated_at`

func scanGroup(row pgx.Row, g *group) error {
	return row.Scan(&g.ID, &g.OrganizationID, &g.NetworkID, &g.Name, &g.Slug, &g.Description, &g.System, &g.CreatedAt, &g.UpdatedAt)
}

func (s *postgresStore) InsertGroup(ctx context.Context, g *group) (string, error) {
	const sql = `
		INSERT INTO public.organization_groups (name, slug, description, organization_id, network_id, system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, FALSE, now(), now())
		RETURNING id`
	err := s.db.QueryRow(ctx, sql, g.Name, g.Slug, g.Description, g.OrganizationID, g.NetworkID).Scan(&g.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Group already exists", err)
		}
		return "", err
	}
	return g.ID, nil
}

func (s *postgresStore) UpdateGroup(ctx context.Context, g *group) error {
	const sql = `
		UPDATE public.organization_groups
		SET name = $2, slug = $3, description = $4, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := s.db.Exec(ctx, sql, g.ID, g.Name, g.Slug, g.Description)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("Group already exists", err)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewNotFoundError("Group not found", nil)
	}
	return nil
}

func (s *postgresStore) DeleteGroup(ctx context.Context, id string) error {
	const sql = `
		UPDATE public.organization_groups SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL AND system = FALSE`
	tag, err := s.db.Exec(ctx, sql, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewBadRequestError("Cannot delete this group", nil)
	}
	return nil
}

func (s *postgresStore) GetGroup(ctx context.Context, id string) (*group, error) {
	sql := `SELECT` + groupSelect + ` FROM public.organization_groups g WHERE g.id = $1 AND g.deleted_at IS NULL`
	g := &group{}
	if err := scanGroup(s.db.QueryRow(ctx, sql, id), g); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Group not found", err)
		}
		return nil, err
	}
	if err := s.attachGroup(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *postgresStore) ListGroups(ctx context.Context, organizationID string) ([]*group, error) {
	sql := `SELECT` + groupSelect + `
		FROM public.organization_groups g
		WHERE g.organization_id = $1 AND g.deleted_at IS NULL
		ORDER BY g.system DESC, g.name ASC`
	rows, err := s.db.Query(ctx, sql, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]*group, 0)
	for rows.Next() {
		g := &group{}
		if err := scanGroup(rows, g); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, g := range groups {
		if err := s.attachGroup(ctx, g); err != nil {
			return nil, err
		}
	}
	return groups, nil
}

func (s *postgresStore) attachGroup(ctx context.Context, g *group) error {
	members, err := s.groupMembers(ctx, g.ID)
	if err != nil {
		return err
	}
	g.Members = members
	ids, err := s.groupPermissionIDs(ctx, g.ID)
	if err != nil {
		return err
	}
	g.PermissionIDs = ids
	return nil
}

func (s *postgresStore) groupMembers(ctx context.Context, groupID string) ([]member, error) {
	const sql = `
		SELECT ou.id, ou.first_name, ou.last_name, ou.email
		FROM public.organization_group_members ogm
		JOIN public.organization_users ou ON ou.id = ogm.organization_user_id
		WHERE ogm.group_id = $1 AND ou.internal = FALSE
		ORDER BY ou.first_name, ou.last_name`
	rows, err := s.db.Query(ctx, sql, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := make([]member, 0)
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.ID, &m.FirstName, &m.LastName, &m.Email); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *postgresStore) groupPermissionIDs(ctx context.Context, groupID string) ([]string, error) {
	const sql = `SELECT permission_id FROM public.organization_group_permissions WHERE group_id = $1`
	rows, err := s.db.Query(ctx, sql, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *postgresStore) AddMember(ctx context.Context, groupID, organizationUserID string) error {
	const sql = `
		INSERT INTO public.organization_group_members (group_id, organization_user_id, created_at)
		VALUES ($1, $2, now())`
	_, err := s.db.Exec(ctx, sql, groupID, organizationUserID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("User is already a member of this group", err)
		}
		return err
	}
	return nil
}

func (s *postgresStore) RemoveMember(ctx context.Context, groupID, organizationUserID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM public.organization_group_members WHERE group_id = $1 AND organization_user_id = $2`, groupID, organizationUserID)
	return err
}

func (s *postgresStore) OrganizationUserScope(ctx context.Context, organizationUserID string) (string, bool, error) {
	const sql = `SELECT organization_id, internal FROM public.organization_users WHERE id = $1`
	var organizationID string
	var internal bool
	err := s.db.QueryRow(ctx, sql, organizationUserID).Scan(&organizationID, &internal)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, apperror.NewNotFoundError("Organization user not found", err)
		}
		return "", false, err
	}
	return organizationID, internal, nil
}

func (s *postgresStore) InsertPermission(ctx context.Context, p *permission) (string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	const sql = `
		INSERT INTO public.organization_permissions (name, slug, description, organization_id, network_id, system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, FALSE, now(), now())
		RETURNING id`
	err = tx.QueryRow(ctx, sql, p.Name, p.Slug, p.Description, p.OrganizationID, p.NetworkID).Scan(&p.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Permission already exists", err)
		}
		return "", err
	}
	if err := replaceOrgGrants(ctx, tx, p.ID, p.Grants); err != nil {
		return "", err
	}
	return p.ID, tx.Commit(ctx)
}

func (s *postgresStore) UpdatePermission(ctx context.Context, p *permission) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	const sql = `
		UPDATE public.organization_permissions
		SET name = $2, slug = $3, description = $4, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := tx.Exec(ctx, sql, p.ID, p.Name, p.Slug, p.Description)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("Permission already exists", err)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewNotFoundError("Permission not found", nil)
	}
	if err := replaceOrgGrants(ctx, tx, p.ID, p.Grants); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replaceOrgGrants(ctx context.Context, tx pgx.Tx, permissionID string, grants []grant) error {
	if _, err := tx.Exec(ctx, `DELETE FROM public.organization_permission_grants WHERE permission_id = $1`, permissionID); err != nil {
		return err
	}
	for _, g := range grants {
		grantID := uuid.MustNew()
		var resourceID, schemaID *string
		if g.ResourceID != "" {
			id := g.ResourceID
			resourceID = &id
		}
		if g.SchemaID != "" {
			id := g.SchemaID
			schemaID = &id
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.organization_permission_grants (id, permission_id, resource, actions, resource_id, schema_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, now())`,
			grantID, permissionID, g.Resource, g.Actions, resourceID, schemaID); err != nil {
			return err
		}
		for _, f := range g.Fields {
			if _, err := tx.Exec(ctx, `
				INSERT INTO public.organization_permission_fields (grant_id, field_name, access)
				VALUES ($1, $2, $3)`, grantID, f.Name, f.Access); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *postgresStore) DeletePermission(ctx context.Context, id string) error {
	const sql = `
		UPDATE public.organization_permissions SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL AND system = FALSE`
	tag, err := s.db.Exec(ctx, sql, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewBadRequestError("Cannot delete this permission", nil)
	}
	return nil
}

func (s *postgresStore) GetPermission(ctx context.Context, id string) (*permission, error) {
	const sql = `
		SELECT id, organization_id, network_id, name, slug, description, system, created_at, updated_at
		FROM public.organization_permissions
		WHERE id = $1 AND deleted_at IS NULL`
	p := &permission{}
	err := s.db.QueryRow(ctx, sql, id).Scan(&p.ID, &p.OrganizationID, &p.NetworkID, &p.Name, &p.Slug, &p.Description, &p.System, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Permission not found", err)
		}
		return nil, err
	}
	grants, err := s.permissionGrants(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	p.Grants = grants
	return p, nil
}

func (s *postgresStore) ListPermissions(ctx context.Context, organizationID string) ([]*permission, error) {
	const sql = `
		SELECT id, organization_id, network_id, name, slug, description, system, created_at, updated_at
		FROM public.organization_permissions
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY system DESC, name ASC`
	rows, err := s.db.Query(ctx, sql, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*permission, 0)
	for rows.Next() {
		p := &permission{}
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.NetworkID, &p.Name, &p.Slug, &p.Description, &p.System, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, p := range items {
		grants, err := s.permissionGrants(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		p.Grants = grants
	}
	return items, nil
}

func (s *postgresStore) permissionGrants(ctx context.Context, permissionID string) ([]grant, error) {
	const sql = `
		SELECT id, resource, actions, COALESCE(resource_id::text, ''), COALESCE(schema_id::text, '')
		FROM public.organization_permission_grants
		WHERE permission_id = $1`
	rows, err := s.db.Query(ctx, sql, permissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		id string
		g  grant
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
	fields, err := s.grantFields(ctx, ids)
	if err != nil {
		return nil, err
	}
	grants := make([]grant, 0, len(collected))
	for _, r := range collected {
		r.g.Fields = fields[r.id]
		grants = append(grants, r.g)
	}
	return grants, nil
}

func (s *postgresStore) grantFields(ctx context.Context, grantIDs []string) (map[string][]fieldGrant, error) {
	out := make(map[string][]fieldGrant)
	if len(grantIDs) == 0 {
		return out, nil
	}
	const sql = `SELECT grant_id, field_name, access FROM public.organization_permission_fields WHERE grant_id = ANY($1)`
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
		out[id] = append(out[id], fieldGrant{Name: name, Access: access})
	}
	return out, rows.Err()
}

func (s *postgresStore) AssignPermission(ctx context.Context, groupID, permissionID string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO public.organization_group_permissions (group_id, permission_id)
		VALUES ($1, $2)`, groupID, permissionID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("Permission already assigned", err)
		}
		return err
	}
	return nil
}

func (s *postgresStore) UnassignPermission(ctx context.Context, groupID, permissionID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM public.organization_group_permissions WHERE group_id = $1 AND permission_id = $2`, groupID, permissionID)
	return err
}
