package user

import (
	"context"
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

func (s *service) Insert(ctx context.Context, req insertUserRequest) (string, error) {
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

	user := &user{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
		Password:  hashedPassword,
	}

	_, err = s.store.Insert(ctx, user)
	if err != nil {
		if apperror.IsConflict(err) {
			s.logger.WarnContext(ctx, "Rejected duplicate user", logger.KeyEmail, user.Email)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert user", logger.KeyEmail, user.Email, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created user", logger.KeyEmail, user.Email)
	return user.ID, nil
}

func (s *service) Get(ctx context.Context, p principal.Principal, id string) (*user, error) {
	if err := principal.RequireUser(p, ""); err != nil {
		return nil, err
	}
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid user ID", nil)
	}
	if id != p.ID {
		return nil, apperror.NewNotFoundError("User not found", nil)
	}

	u, err := s.store.GetByID(ctx, id)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get user", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}
	return u, nil
}

func (s *service) Patch(ctx context.Context, existing *user, req patchUserRequest) error {
	if req.FirstName == nil && req.LastName == nil {
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

	if err := s.store.Update(ctx, existing); err != nil {
		s.logger.ErrorContext(ctx, "Failed to update user", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated user", logger.KeyID, existing.ID)
	return nil
}

func (s *service) UpdatePassword(ctx context.Context, existing *user, req updatePasswordRequest) error {
	if req.CurrentPassword == "" {
		return apperror.NewBadRequestError("Current password is required", nil)
	}
	if req.NewPassword == "" {
		return apperror.NewBadRequestError("New password is required", nil)
	}
	if req.NewPassword == req.CurrentPassword {
		return apperror.NewBadRequestError("New password must be different", nil)
	}

	ok, err := s.hasher.Compare(req.CurrentPassword, existing.Password)
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
