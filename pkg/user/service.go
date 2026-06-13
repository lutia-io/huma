package user

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
)

type service struct {
	logger *logger.Logger
	store  store
}

func newService(logger *logger.Logger, store store) *service {
	return &service{
		logger: logger,
		store:  store,
	}
}

func (s *service) Find(ctx context.Context) ([]user, error) {
	users, err := s.store.Find(ctx)
	if err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "Successfully fetched users", logger.KeyCount, len(users))
	return users, nil
}

func (s *service) FindById(ctx context.Context, id string) (*user, error) {
	user, err := s.store.FindById(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to fetch user", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}
	s.logger.InfoContext(ctx, "Successfully fetched user", logger.KeyID, user.ID)
	return user, nil
}

func (s *service) Insert(ctx context.Context, req insertUserRequest) (string, error) {

	firstName := strings.TrimSpace(req.FirstName)
	if firstName == "" {
		s.logger.WarnContext(ctx, "Empty first name")
		return "", apperror.NewBadRequestError("user.service.Insert", "First name is required", nil)
	}

	lastName := strings.TrimSpace(req.LastName)
	if lastName == "" {
		s.logger.WarnContext(ctx, "Empty last name")
		return "", apperror.NewBadRequestError("user.service.Insert", "Last name is required", nil)
	}

	email := strings.TrimSpace(req.Email)
	if email == "" {
		s.logger.WarnContext(ctx, "Empty email")
		return "", apperror.NewBadRequestError("user.service.Insert", "Email is required", nil)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		s.logger.WarnContext(ctx, "Invalid email", logger.KeyEmail, email, logger.KeyError, err)
		return "", apperror.NewBadRequestError("user.service.Insert", "Email is invalid", err)
	}

	if req.Password == "" {
		s.logger.WarnContext(ctx, "Empty password")
		return "", apperror.NewBadRequestError("user.service.Insert", "Password is required", nil)
	}

	// TODO: Hash password

	now := time.Now().UTC()
	user := &user{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
		Password:  req.Password,
		CreatedAt: now,
		UpdatedAt: now,
	}

	id, err := s.store.Insert(ctx, user)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate user", logger.KeyEmail, user.Email)
		}
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created user", logger.KeyID, id, logger.KeyEmail, user.Email)
	return id, nil
}
