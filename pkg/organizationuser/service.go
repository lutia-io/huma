package organizationuser

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/mail"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/uuid"
)

const (
	systemUserFirstName = "System"
	systemUserLastName  = "User"
)

func systemUserEmail(organizationID string) string {
	return "system+" + organizationID + "@huma.internal"
}

type Service struct {
	logger *logger.Logger
	store  store
	hasher hasher.Hasher
}

func NewService(logger *logger.Logger, store store, hasher hasher.Hasher) *Service {
	return &Service{
		logger: logger,
		store:  store,
		hasher: hasher,
	}
}

func (s *Service) Insert(ctx context.Context, req insertOrganizationUserRequest) (string, error) {
	firstName := strings.TrimSpace(req.FirstName)
	if firstName == "" {
		s.logger.WarnContext(ctx, "Empty first name")
		return "", apperror.NewBadRequestError("First name is required", nil)
	}

	lastName := strings.TrimSpace(req.LastName)
	if lastName == "" {
		s.logger.WarnContext(ctx, "Empty last name")
		return "", apperror.NewBadRequestError("Last name is required", nil)
	}

	email := strings.TrimSpace(req.Email)
	if email == "" {
		s.logger.WarnContext(ctx, "Empty email")
		return "", apperror.NewBadRequestError("Email is required", nil)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		s.logger.WarnContext(ctx, "Invalid email", logger.KeyEmail, email, logger.KeyError, err)
		return "", apperror.NewBadRequestError("Email is invalid", err)
	}

	if req.Password == "" {
		s.logger.WarnContext(ctx, "Empty password")
		return "", apperror.NewBadRequestError("Password is required", nil)
	}

	hashedPassword, err := s.hasher.Hash(req.Password)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to hash password", logger.KeyError, err)
		return "", apperror.NewInternalError("Failed to hash password", err)
	}

	organizationID := strings.TrimSpace(req.OrganizationID)
	if organizationID == "" {
		s.logger.WarnContext(ctx, "Empty organization ID")
		return "", apperror.NewBadRequestError("Organization ID is required", nil)
	}

	organizationUser := &organizationUser{
		FirstName:      firstName,
		LastName:       lastName,
		Email:          email,
		Password:       hashedPassword,
		OrganizationID: organizationID,
		NetworkID:      req.NetworkID,
	}

	id, err := s.store.Insert(ctx, organizationUser)
	if err != nil {
		if apperror.IsConflict(err) {
			s.logger.WarnContext(ctx, "Rejected duplicate organization user", "organization_id", organizationID, logger.KeyEmail, email)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert organization user", "organization_id", organizationID, logger.KeyEmail, email, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created organization user", logger.KeyID, id)
	return id, nil
}

func (s *Service) Patch(ctx context.Context, existing *organizationUser, req patchOrganizationUserRequest) error {
	if existing.Internal {
		return apperror.NewNotFoundError("Organization user not found", nil)
	}
	if req.FirstName == nil && req.LastName == nil && req.Email == nil && req.Password == nil {
		return apperror.NewBadRequestError("No fields to update", nil)
	}

	if req.FirstName != nil {
		firstName := strings.TrimSpace(*req.FirstName)
		if firstName == "" {
			s.logger.WarnContext(ctx, "Empty first name")
			return apperror.NewBadRequestError("First name is required", nil)
		}
		existing.FirstName = firstName
	}

	if req.LastName != nil {
		lastName := strings.TrimSpace(*req.LastName)
		if lastName == "" {
			s.logger.WarnContext(ctx, "Empty last name")
			return apperror.NewBadRequestError("Last name is required", nil)
		}
		existing.LastName = lastName
	}

	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if email == "" {
			s.logger.WarnContext(ctx, "Empty email")
			return apperror.NewBadRequestError("Email is required", nil)
		}
		if _, err := mail.ParseAddress(email); err != nil {
			s.logger.WarnContext(ctx, "Invalid email", logger.KeyEmail, email, logger.KeyError, err)
			return apperror.NewBadRequestError("Email is invalid", err)
		}
		existing.Email = email
	}

	var hashedPassword *string
	if req.Password != nil {
		if *req.Password == "" {
			s.logger.WarnContext(ctx, "Empty password")
			return apperror.NewBadRequestError("Password is required", nil)
		}
		hashed, err := s.hasher.Hash(*req.Password)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to hash password", logger.KeyError, err)
			return apperror.NewInternalError("Failed to hash password", err)
		}
		hashedPassword = &hashed
	}

	if err := s.store.Update(ctx, existing, hashedPassword); err != nil {
		if apperror.IsConflict(err) {
			s.logger.WarnContext(ctx, "Rejected duplicate organization user", logger.KeyID, existing.ID, logger.KeyEmail, existing.Email)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to update organization user", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated organization user", logger.KeyID, existing.ID)
	return nil
}

func (s *Service) UpdatePassword(ctx context.Context, existing *organizationUser, req updatePasswordRequest) error {
	if existing.Internal {
		return apperror.NewNotFoundError("Organization user not found", nil)
	}
	if req.CurrentPassword == "" {
		return apperror.NewBadRequestError("Current password is required", nil)
	}
	if req.NewPassword == "" {
		return apperror.NewBadRequestError("New password is required", nil)
	}
	if req.NewPassword == req.CurrentPassword {
		return apperror.NewBadRequestError("New password must be different", nil)
	}

	hashedCurrent, err := s.store.GetPasswordByID(ctx, existing.ID)
	if err != nil {
		if apperror.IsNotFound(err) {
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to load current password", logger.KeyID, existing.ID, logger.KeyError, err)
		return apperror.NewInternalError("Failed to update password", err)
	}

	ok, err := s.hasher.Compare(req.CurrentPassword, hashedCurrent)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to compare password", logger.KeyError, err)
		return apperror.NewInternalError("Failed to update password", err)
	}
	if !ok {
		return apperror.NewBadRequestError("Current password is incorrect", nil)
	}

	hashedPassword, err := s.hasher.Hash(req.NewPassword)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to hash password", logger.KeyError, err)
		return apperror.NewInternalError("Failed to hash password", err)
	}

	if err := s.store.UpdatePassword(ctx, existing.ID, hashedPassword); err != nil {
		s.logger.ErrorContext(ctx, "Failed to update password", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated password", logger.KeyID, existing.ID)
	return nil
}

func (s *Service) List(ctx context.Context, p principal.Principal, params listParams) (*listResult, error) {
	switch p.Type {
	case principal.TypeUser:
		params.UserID = p.ID
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" || p.OrganizationID == "" {
			return nil, apperror.NewForbiddenError("Organization user token missing network or organization", nil)
		}
		params.UserID = ""
		params.NetworkID = p.NetworkID
		params.OrganizationID = p.OrganizationID
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	result, err := s.store.List(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list organization users", logger.KeyUserID, p.ID, logger.KeyError, err)
		return nil, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, p principal.Principal, id string) (*organizationUser, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid organization user ID", nil)
	}

	u, err := s.store.GetByID(ctx, id)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get organization user", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}
	if u.Internal {
		return nil, apperror.NewNotFoundError("Organization user not found", nil)
	}

	switch p.Type {
	case principal.TypeUser:
		if u.UserID != p.ID {
			return nil, apperror.NewNotFoundError("Organization user not found", nil)
		}
	case principal.TypeOrganizationUser:
		if u.NetworkID != p.NetworkID || u.OrganizationID != p.OrganizationID {
			return nil, apperror.NewNotFoundError("Organization user not found", nil)
		}
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	return u, nil
}

// Scope returns the network and organization for an organization user.
func (s *Service) Scope(ctx context.Context, id string) (*Scope, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid organization user ID", nil)
	}
	u, err := s.store.GetByID(ctx, id)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get organization user", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}
	if u.Internal {
		return nil, apperror.NewNotFoundError("Organization user not found", nil)
	}
	return &Scope{
		ID:             u.ID,
		OrganizationID: u.OrganizationID,
		NetworkID:      u.NetworkID,
		UserID:         u.UserID,
	}, nil
}

// SystemUserID returns the hidden system organization user for the
// organization, creating it if it does not yet exist.
func (s *Service) SystemUserID(ctx context.Context, organizationID, networkID string) (string, error) {
	return s.EnsureSystemUser(ctx, organizationID, networkID)
}

// EnsureSystemUser inserts the hidden system organization user for an
// organization if one does not already exist, and returns its ID.
func (s *Service) EnsureSystemUser(ctx context.Context, organizationID, networkID string) (string, error) {
	organizationID = strings.TrimSpace(organizationID)
	if !uuid.Valid(organizationID) {
		return "", apperror.NewBadRequestError("Invalid organization ID", nil)
	}
	networkID = strings.TrimSpace(networkID)
	if !uuid.Valid(networkID) {
		return "", apperror.NewBadRequestError("Invalid network ID", nil)
	}

	id, err := s.store.GetSystemUserID(ctx, organizationID, networkID)
	if err == nil {
		return id, nil
	}
	if !apperror.IsNotFound(err) {
		s.logger.ErrorContext(ctx, "Failed to load system organization user", "organization_id", organizationID, logger.KeyError, err)
		return "", err
	}

	password, err := randomPassword()
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to generate system user password", logger.KeyError, err)
		return "", apperror.NewInternalError("Failed to create system organization user", err)
	}
	hashedPassword, err := s.hasher.Hash(password)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to hash system user password", logger.KeyError, err)
		return "", apperror.NewInternalError("Failed to create system organization user", err)
	}

	id, err = s.store.InsertSystemUser(ctx, &organizationUser{
		FirstName:      systemUserFirstName,
		LastName:       systemUserLastName,
		Email:          systemUserEmail(organizationID),
		Password:       hashedPassword,
		OrganizationID: organizationID,
		NetworkID:      networkID,
		Internal:       true,
	})
	if err != nil {
		if apperror.IsConflict(err) {
			existingID, getErr := s.store.GetSystemUserID(ctx, organizationID, networkID)
			if getErr == nil {
				return existingID, nil
			}
		}
		s.logger.ErrorContext(ctx, "Failed to insert system organization user", "organization_id", organizationID, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created system organization user", logger.KeyID, id, "organization_id", organizationID)
	return id, nil
}

func randomPassword() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// ResolveCreateActor returns the organization user a record or file should be
// created as. Organization-user sessions always create as themselves.
// Platform users must pass an organizationUserId they administer.
func ResolveCreateActor(ctx context.Context, p principal.Principal, organizationUserID string, lookup func(context.Context, string) (*Scope, error)) (*Scope, error) {
	requestedID := strings.TrimSpace(organizationUserID)

	switch p.Type {
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" || p.OrganizationID == "" {
			return nil, apperror.NewForbiddenError("Organization user token missing network or organization", nil)
		}
		if requestedID != "" && requestedID != p.ID {
			return nil, apperror.NewForbiddenError("Cannot create as another organization user", nil)
		}
		return &Scope{
			ID:             p.ID,
			OrganizationID: p.OrganizationID,
			NetworkID:      p.NetworkID,
		}, nil
	case principal.TypeUser:
		if requestedID == "" {
			return nil, apperror.NewBadRequestError("Organization user ID is required", nil)
		}
		scope, err := lookup(ctx, requestedID)
		if err != nil {
			return nil, err
		}
		if scope.UserID != p.ID {
			return nil, apperror.NewNotFoundError("Organization user not found", nil)
		}
		if p.NetworkID != "" && p.NetworkID != scope.NetworkID {
			return nil, apperror.NewForbiddenError("Network mismatch", nil)
		}
		return scope, nil
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}
}
