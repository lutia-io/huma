package networkaccess

import (
	"context"
	"net/mail"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/authz"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/slug"
	"github.com/lutia-io/huma/pkg/uuid"
)

type service struct {
	logger *logger.Logger
	store  store
	authz  *authz.Engine
}

func newService(logger *logger.Logger, store store, engine *authz.Engine) *service {
	return &service{logger: logger, store: store, authz: engine}
}

func (s *service) requireVisible(ctx context.Context, p principal.Principal, networkID string) error {
	return s.authz.VisibleNetwork(ctx, p, networkID)
}

func (s *service) requireManage(ctx context.Context, p principal.Principal, networkID string) error {
	return s.authz.RequireManageAccess(ctx, p, networkID)
}

func (s *service) ListGroups(ctx context.Context, p principal.Principal, networkID string) ([]*group, error) {
	if err := s.requireVisible(ctx, p, networkID); err != nil {
		return nil, err
	}
	return s.store.ListGroups(ctx, networkID)
}

func (s *service) GetGroup(ctx context.Context, p principal.Principal, id string) (*group, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid group ID", nil)
	}
	g, err := s.store.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireVisible(ctx, p, g.NetworkID); err != nil {
		return nil, apperror.NewNotFoundError("Group not found", nil)
	}
	return g, nil
}

func (s *service) InsertGroup(ctx context.Context, p principal.Principal, req insertGroupRequest) (string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", apperror.NewBadRequestError("Name is required", nil)
	}
	sl := slug.Slugify(name)
	if sl == "" {
		return "", apperror.NewBadRequestError("Slug is required", nil)
	}
	if sl == authz.SlugOwners || sl == authz.SlugMembers {
		return "", apperror.NewConflictError("Group already exists", nil)
	}
	if err := s.requireManage(ctx, p, req.NetworkID); err != nil {
		return "", err
	}
	return s.store.InsertGroup(ctx, &group{
		Name:        name,
		Slug:        sl,
		Description: strings.TrimSpace(req.Description),
		NetworkID:   req.NetworkID,
	})
}

func (s *service) PatchGroup(ctx context.Context, p principal.Principal, existing *group, req patchGroupRequest) error {
	if err := s.requireManage(ctx, p, existing.NetworkID); err != nil {
		return err
	}
	if req.Name == nil && req.Description == nil {
		return apperror.NewBadRequestError("No fields to update", nil)
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return apperror.NewBadRequestError("Name is required", nil)
		}
		existing.Name = name
		if !existing.System {
			existing.Slug = slug.Slugify(name)
		}
	}
	if req.Description != nil {
		existing.Description = strings.TrimSpace(*req.Description)
	}
	return s.store.UpdateGroup(ctx, existing)
}

func (s *service) DeleteGroup(ctx context.Context, p principal.Principal, existing *group) error {
	if err := s.requireManage(ctx, p, existing.NetworkID); err != nil {
		return err
	}
	if existing.System {
		return apperror.NewBadRequestError("Cannot delete this group", nil)
	}
	return s.store.DeleteGroup(ctx, existing.ID)
}

func (s *service) AddMember(ctx context.Context, p principal.Principal, existing *group, email string) error {
	if err := s.requireManage(ctx, p, existing.NetworkID); err != nil {
		return err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return apperror.NewBadRequestError("Email is required", nil)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return apperror.NewBadRequestError("Email is invalid", err)
	}
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	return s.store.AddMember(ctx, existing.ID, user.ID)
}

func (s *service) InviteMember(ctx context.Context, p principal.Principal, networkID, email string) error {
	if err := s.requireManage(ctx, p, networkID); err != nil {
		return err
	}
	groups, err := s.store.ListGroups(ctx, networkID)
	if err != nil {
		return err
	}
	var members *group
	for _, g := range groups {
		if g.Slug == authz.SlugMembers {
			members = g
			break
		}
	}
	if members == nil {
		return apperror.NewNotFoundError("Members group not found", nil)
	}
	return s.AddMember(ctx, p, members, email)
}

func (s *service) RemoveMember(ctx context.Context, p principal.Principal, existing *group, userID string) error {
	if err := s.authz.RequireCreator(ctx, p, existing.NetworkID); err != nil {
		return err
	}
	if !uuid.Valid(userID) {
		return apperror.NewBadRequestError("Invalid user ID", nil)
	}
	snap, err := s.authz.Load(ctx, p, existing.NetworkID, "")
	if err != nil {
		return err
	}
	if userID == snap.CreatedBy {
		return apperror.NewBadRequestError("Cannot remove the network creator", nil)
	}
	return s.store.RemoveMember(ctx, existing.ID, userID)
}

func (s *service) ListPermissions(ctx context.Context, p principal.Principal, networkID string) ([]*permission, error) {
	if err := s.requireVisible(ctx, p, networkID); err != nil {
		return nil, err
	}
	return s.store.ListPermissions(ctx, networkID)
}

func (s *service) GetPermission(ctx context.Context, p principal.Principal, id string) (*permission, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid permission ID", nil)
	}
	perm, err := s.store.GetPermission(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireVisible(ctx, p, perm.NetworkID); err != nil {
		return nil, apperror.NewNotFoundError("Permission not found", nil)
	}
	return perm, nil
}

func validateGrants(grants []grant) error {
	if len(grants) == 0 {
		return apperror.NewBadRequestError("Grants are required", nil)
	}
	for _, g := range grants {
		if err := authz.ValidateNetworkGrant(g.Resource, g.Actions); err != nil {
			return err
		}
		if g.ResourceID != "" && !uuid.Valid(g.ResourceID) {
			return apperror.NewBadRequestError("Invalid resource ID", nil)
		}
	}
	return nil
}

func (s *service) InsertPermission(ctx context.Context, p principal.Principal, req insertPermissionRequest) (string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", apperror.NewBadRequestError("Name is required", nil)
	}
	sl := slug.Slugify(name)
	if sl == "" {
		return "", apperror.NewBadRequestError("Slug is required", nil)
	}
	if err := validateGrants(req.Grants); err != nil {
		return "", err
	}
	if err := s.requireManage(ctx, p, req.NetworkID); err != nil {
		return "", err
	}
	return s.store.InsertPermission(ctx, &permission{
		Name:        name,
		Slug:        sl,
		Description: strings.TrimSpace(req.Description),
		NetworkID:   req.NetworkID,
		Grants:      req.Grants,
	})
}

func (s *service) PatchPermission(ctx context.Context, p principal.Principal, existing *permission, req patchPermissionRequest) error {
	if err := s.requireManage(ctx, p, existing.NetworkID); err != nil {
		return err
	}
	if req.Name == nil && req.Description == nil && req.Grants == nil {
		return apperror.NewBadRequestError("No fields to update", nil)
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return apperror.NewBadRequestError("Name is required", nil)
		}
		existing.Name = name
		if !existing.System {
			existing.Slug = slug.Slugify(name)
		}
	}
	if req.Description != nil {
		existing.Description = strings.TrimSpace(*req.Description)
	}
	if req.Grants != nil {
		if existing.System {
			return apperror.NewBadRequestError("Cannot change grants on a system permission", nil)
		}
		if err := validateGrants(req.Grants); err != nil {
			return err
		}
		existing.Grants = req.Grants
	}
	return s.store.UpdatePermission(ctx, existing)
}

func (s *service) DeletePermission(ctx context.Context, p principal.Principal, existing *permission) error {
	if err := s.requireManage(ctx, p, existing.NetworkID); err != nil {
		return err
	}
	if existing.System {
		return apperror.NewBadRequestError("Cannot delete this permission", nil)
	}
	return s.store.DeletePermission(ctx, existing.ID)
}

func (s *service) AssignPermission(ctx context.Context, p principal.Principal, existing *group, permissionID string) error {
	if err := s.requireManage(ctx, p, existing.NetworkID); err != nil {
		return err
	}
	if !uuid.Valid(permissionID) {
		return apperror.NewBadRequestError("Invalid permission ID", nil)
	}
	perm, err := s.store.GetPermission(ctx, permissionID)
	if err != nil {
		return err
	}
	if perm.NetworkID != existing.NetworkID {
		return apperror.NewBadRequestError("Permission does not belong to this network", nil)
	}
	return s.store.AssignPermission(ctx, existing.ID, permissionID)
}

func (s *service) UnassignPermission(ctx context.Context, p principal.Principal, existing *group, permissionID string) error {
	if err := s.requireManage(ctx, p, existing.NetworkID); err != nil {
		return err
	}
	return s.store.UnassignPermission(ctx, existing.ID, permissionID)
}
