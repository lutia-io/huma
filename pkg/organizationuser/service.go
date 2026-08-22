package organizationuser

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/uuid"
)

type service struct {
	logger *logger.Logger
	store  store
	hasher hasher.Hasher
}

func newService(logger *logger.Logger, store store, hasher hasher.Hasher) *service {
	return &service{
		logger: logger,
		store:  store,
		hasher: hasher,
	}
}

func (s *service) Insert(ctx context.Context, req insertOrganizationUserRequest) (string, error) {
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
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate organization user", "organization_id", organizationID, logger.KeyEmail, email)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert organization user", "organization_id", organizationID, logger.KeyEmail, email, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created organization user", logger.KeyID, id)
	return id, nil
}

func (s *service) Patch(ctx context.Context, existing *organizationUser, req patchOrganizationUserRequest) error {
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
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate organization user", logger.KeyID, existing.ID, logger.KeyEmail, existing.Email)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to update organization user", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated organization user", logger.KeyID, existing.ID)
	return nil
}

func (s *service) List(ctx context.Context, p principal.Principal) ([]*organizationUser, error) {
	switch p.Type {
	case principal.TypeUser:
		organizationUsers, err := s.store.ListByUserID(ctx, p.ID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to list organization users", logger.KeyUserID, p.ID, logger.KeyError, err)
			return nil, err
		}
		return organizationUsers, nil
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" || p.OrganizationID == "" {
			return nil, apperror.NewUnauthorizedError("Organization user token missing network or organization", nil)
		}
		organizationUsers, err := s.store.ListByOrganization(ctx, p.NetworkID, p.OrganizationID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to list organization users", logger.KeyError, err)
			return nil, err
		}
		return organizationUsers, nil
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}
}

func (s *service) Get(ctx context.Context, p principal.Principal, id string) (*organizationUser, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid organization user ID", nil)
	}

	u, err := s.store.GetByID(ctx, id)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get organization user", logger.KeyID, id, logger.KeyError, err)
		return nil, err
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
