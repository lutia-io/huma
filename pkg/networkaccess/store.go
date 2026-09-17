package networkaccess

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
	ListGroups(ctx context.Context, networkID string) ([]*group, error)
	AddMember(ctx context.Context, groupID, userID string) error
	RemoveMember(ctx context.Context, groupID, userID string) error
	GroupHasMember(ctx context.Context, groupID, userID string) (bool, error)
	GetUserByEmail(ctx context.Context, email string) (*member, error)
	InsertPermission(ctx context.Context, p *permission) (string, error)
	UpdatePermission(ctx context.Context, p *permission) error
	DeletePermission(ctx context.Context, id string) error
	GetPermission(ctx context.Context, id string) (*permission, error)
	ListPermissions(ctx context.Context, networkID string) ([]*permission, error)
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
	g.id, g.network_id, g.name, g.slug, g.description, g.system, g.created_at, g.updated_at`

func scanGroup(row pgx.Row, g *group) error {
	return row.Scan(&g.ID, &g.NetworkID, &g.Name, &g.Slug, &g.Description, &g.System, &g.CreatedAt, &g.UpdatedAt)
}

func (s *postgresStore) InsertGroup(ctx context.Context, g *group) (string, error) {
	const sql = `
		INSERT INTO public.network_groups (name, slug, description, network_id, system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, FALSE, now(), now())
		RETURNING id`
	err := s.db.QueryRow(ctx, sql, g.Name, g.Slug, g.Description, g.NetworkID).Scan(&g.ID)
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
		UPDATE public.network_groups
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
		UPDATE public.network_groups SET deleted_at = now(), updated_at = now()
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
	sql := `SELECT` + groupSelect + ` FROM public.network_groups g WHERE g.id = $1 AND g.deleted_at IS NULL`
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

func (s *postgresStore) ListGroups(ctx context.Context, networkID string) ([]*group, error) {
	sql := `SELECT` + groupSelect + `
		FROM public.network_groups g
		WHERE g.network_id = $1 AND g.deleted_at IS NULL
		ORDER BY g.system DESC, g.name ASC`
	rows, err := s.db.Query(ctx, sql, networkID)
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
		SELECT u.id, u.first_name, u.last_name, u.email
		FROM public.network_group_members ngm
		JOIN public.users u ON u.id = ngm.user_id
		WHERE ngm.group_id = $1
		ORDER BY u.first_name, u.last_name`
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
	const sql = `SELECT permission_id FROM public.network_group_permissions WHERE group_id = $1`
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

func (s *postgresStore) AddMember(ctx context.Context, groupID, userID string) error {
	const sql = `
		INSERT INTO public.network_group_members (group_id, user_id, created_at)
		VALUES ($1, $2, now())`
	_, err := s.db.Exec(ctx, sql, groupID, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("User is already a member of this group", err)
		}
		return err
	}
	return nil
}

func (s *postgresStore) RemoveMember(ctx context.Context, groupID, userID string) error {
	const sql = `DELETE FROM public.network_group_members WHERE group_id = $1 AND user_id = $2`
	_, err := s.db.Exec(ctx, sql, groupID, userID)
	return err
}

func (s *postgresStore) GroupHasMember(ctx context.Context, groupID, userID string) (bool, error) {
	const sql = `SELECT 1 FROM public.network_group_members WHERE group_id = $1 AND user_id = $2`
	var one int
	err := s.db.QueryRow(ctx, sql, groupID, userID).Scan(&one)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *postgresStore) GetUserByEmail(ctx context.Context, email string) (*member, error) {
	const sql = `SELECT id, first_name, last_name, email FROM public.users WHERE email = $1`
	m := &member{}
	err := s.db.QueryRow(ctx, sql, email).Scan(&m.ID, &m.FirstName, &m.LastName, &m.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("User not found", err)
		}
		return nil, err
	}
	return m, nil
}

func (s *postgresStore) InsertPermission(ctx context.Context, p *permission) (string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	const sql = `
		INSERT INTO public.network_permissions (name, slug, description, network_id, system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, FALSE, now(), now())
		RETURNING id`
	err = tx.QueryRow(ctx, sql, p.Name, p.Slug, p.Description, p.NetworkID).Scan(&p.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Permission already exists", err)
		}
		return "", err
	}
	if err := replaceNetworkGrants(ctx, tx, p.ID, p.Grants); err != nil {
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
		UPDATE public.network_permissions
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
	if err := replaceNetworkGrants(ctx, tx, p.ID, p.Grants); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replaceNetworkGrants(ctx context.Context, tx pgx.Tx, permissionID string, grants []grant) error {
	if _, err := tx.Exec(ctx, `DELETE FROM public.network_permission_grants WHERE permission_id = $1`, permissionID); err != nil {
		return err
	}
	for _, g := range grants {
		var resourceID *string
		if g.ResourceID != "" {
			id := g.ResourceID
			resourceID = &id
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.network_permission_grants (id, permission_id, resource, actions, resource_id, created_at)
			VALUES ($1, $2, $3, $4, $5, now())`,
			uuid.MustNew(), permissionID, g.Resource, g.Actions, resourceID); err != nil {
			return err
		}
	}
	return nil
}

func (s *postgresStore) DeletePermission(ctx context.Context, id string) error {
	const sql = `
		UPDATE public.network_permissions SET deleted_at = now(), updated_at = now()
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
		SELECT id, network_id, name, slug, description, system, created_at, updated_at
		FROM public.network_permissions
		WHERE id = $1 AND deleted_at IS NULL`
	p := &permission{}
	err := s.db.QueryRow(ctx, sql, id).Scan(&p.ID, &p.NetworkID, &p.Name, &p.Slug, &p.Description, &p.System, &p.CreatedAt, &p.UpdatedAt)
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

func (s *postgresStore) ListPermissions(ctx context.Context, networkID string) ([]*permission, error) {
	const sql = `
		SELECT id, network_id, name, slug, description, system, created_at, updated_at
		FROM public.network_permissions
		WHERE network_id = $1 AND deleted_at IS NULL
		ORDER BY system DESC, name ASC`
	rows, err := s.db.Query(ctx, sql, networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*permission, 0)
	for rows.Next() {
		p := &permission{}
		if err := rows.Scan(&p.ID, &p.NetworkID, &p.Name, &p.Slug, &p.Description, &p.System, &p.CreatedAt, &p.UpdatedAt); err != nil {
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
		SELECT resource, actions, COALESCE(resource_id::text, '')
		FROM public.network_permission_grants
		WHERE permission_id = $1`
	rows, err := s.db.Query(ctx, sql, permissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := make([]grant, 0)
	for rows.Next() {
		var g grant
		if err := rows.Scan(&g.Resource, &g.Actions, &g.ResourceID); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

func (s *postgresStore) AssignPermission(ctx context.Context, groupID, permissionID string) error {
	const sql = `
		INSERT INTO public.network_group_permissions (group_id, permission_id)
		VALUES ($1, $2)`
	_, err := s.db.Exec(ctx, sql, groupID, permissionID)
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
	_, err := s.db.Exec(ctx, `DELETE FROM public.network_group_permissions WHERE group_id = $1 AND permission_id = $2`, groupID, permissionID)
	return err
}
