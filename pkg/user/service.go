package user

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

func (s *service) Insert(ctx context.Context, req insertUserRequest) error {
	firstName := strings.TrimSpace(req.FirstName)
	if firstName == "" {
		s.logger.WarnContext(ctx, "Empty first name")
		return apperror.NewBadRequestError("user.service.Insert", "First name is required", nil)
	}

	lastName := strings.TrimSpace(req.LastName)
	if lastName == "" {
		s.logger.WarnContext(ctx, "Empty last name")
		return apperror.NewBadRequestError("user.service.Insert", "Last name is required", nil)
	}

	email := strings.TrimSpace(req.Email)
	if email == "" {
		s.logger.WarnContext(ctx, "Empty email")
		return apperror.NewBadRequestError("user.service.Insert", "Email is required", nil)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		s.logger.WarnContext(ctx, "Invalid email", logger.KeyEmail, email, logger.KeyError, err)
		return apperror.NewBadRequestError("user.service.Insert", "Email is invalid", err)
	}

	if req.Password == "" {
		s.logger.WarnContext(ctx, "Empty password")
		return apperror.NewBadRequestError("user.service.Insert", "Password is required", nil)
	}

	hashedPassword, err := s.hasher.Hash(req.Password)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to hash password", logger.KeyError, err)
		return apperror.NewInternalError("user.service.Insert", "Failed to hash password", err)
	}

	user := &user{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
		Password:  hashedPassword,
	}

	err = s.store.Insert(ctx, user)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate user", logger.KeyEmail, user.Email)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to insert user", logger.KeyEmail, user.Email, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully created user", logger.KeyEmail, user.Email)
	return nil
}
