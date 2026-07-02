package organizationuser

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
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
