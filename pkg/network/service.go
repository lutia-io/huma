package network

import (
	"context"
	"errors"
	"strings"
	"time"

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

func (s *service) Find(ctx context.Context) ([]network, error) {
	networks, err := s.store.Find(ctx)
	if err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "Successfully fetched networks", logger.KeyCount, len(networks))
	return networks, nil
}

func (s *service) FindByID(ctx context.Context, id string) (*network, error) {
	network, err := s.store.FindByID(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to fetch network", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}
	s.logger.InfoContext(ctx, "Successfully fetched network", logger.KeyID, network.ID, logger.KeySlug, network.Slug)
	return network, nil
}

func (s *service) Insert(ctx context.Context, req insertNetworkRequest) (string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		s.logger.WarnContext(ctx, "Empty name")
		return "", apperror.NewBadRequestError("network.service.Insert", "Name is required", nil)
	}

	slug := slug.Slugify(req.Name)
	if slug == "" {
		s.logger.WarnContext(ctx, "Empty slug")
		return "", apperror.NewBadRequestError("network.service.Insert", "Slug is required", nil)
	}

	now := time.Now().UTC()
	network := &network{
		Name:      name,
		Slug:      slug,
		UserID:    req.UserID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	id, err := s.store.Insert(ctx, network)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate network", logger.KeySlug, slug)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert network", logger.KeySlug, slug, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created network", logger.KeyID, id)
	return id, nil
}

func (s *service) UpdateByID(ctx context.Context, id string, req updateNetworkRequest) error {
	network, err := s.store.FindByID(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to fetch network", logger.KeyID, id, logger.KeyError, err)
		return err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		s.logger.WarnContext(ctx, "Empty name")
		return apperror.NewBadRequestError("network.service.Update", "Name is required", nil)
	}

	network.Name = name
	network.UpdatedAt = time.Now().UTC()

	err = s.store.UpdateByID(ctx, id, network)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to update network", logger.KeyID, id, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated network", logger.KeyID, id)
	return nil
}

func (s *service) DeleteByID(ctx context.Context, id string) error {
	if err := s.store.SoftDeleteByID(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "Failed to delete network", logger.KeyID, id, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully deleted network", logger.KeyID, id)
	return nil
}
