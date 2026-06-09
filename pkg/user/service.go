package user

import (
	"context"
	"errors"
	"log/slog"
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

func (s *service) Insert(ctx context.Context, user *user) (string, error) {
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

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
