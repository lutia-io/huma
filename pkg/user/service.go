package user

import (
	"context"
	"errors"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/lutia-io/huma/pkg/apperror"
)

type service struct {
	logger *slog.Logger
	store  store
}

func NewService(logger *slog.Logger, store store) *service {
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
	s.logger.InfoContext(ctx, "Successfully fetched users", "count", len(users))
	return users, nil
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
		s.logger.WarnContext(ctx, "Invalid email", "email", email, "error", err)
		return "", apperror.NewBadRequestError("user.service.Insert", "Email is invalid", err)
	}

	if req.Password == "" {
		s.logger.WarnContext(ctx, "Empty password")
		return "", apperror.NewBadRequestError("user.service.Insert", "Password is required", nil)
	}

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
			s.logger.WarnContext(ctx, "Rejected duplicate user", "email", user.Email)
		}
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created user", "id", id, "email", user.Email)
	return id, nil
}
