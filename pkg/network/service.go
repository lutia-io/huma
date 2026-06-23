package network

import (
	"context"
	"errors"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/slug"
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

func (s *service) Insert(ctx context.Context, req insertNetworkRequest) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		s.logger.WarnContext(ctx, "Empty name")
		return apperror.NewBadRequestError("network.service.Insert", "Name is required", nil)
	}

	slug := slug.Slugify(req.Name)
	if slug == "" {
		s.logger.WarnContext(ctx, "Empty slug")
		return apperror.NewBadRequestError("network.service.Insert", "Slug is required", nil)
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		s.logger.WarnContext(ctx, "Empty user ID")
		return apperror.NewBadRequestError("network.service.Insert", "User ID is required", nil)
	}

	network := &network{
		Name:   name,
		Slug:   slug,
		UserID: req.UserID,
	}

	err := s.store.Insert(ctx, network)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate network", logger.KeySlug, slug)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to insert network", logger.KeySlug, slug, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully created network", logger.KeyID, network.ID)
	return nil
}
