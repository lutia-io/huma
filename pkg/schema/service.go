package schema

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

func (s *service) Insert(ctx context.Context, req insertSchemaRequest) (string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		s.logger.WarnContext(ctx, "Empty name")
		return "", apperror.NewBadRequestError("schema.service.Insert", "Name is required", nil)
	}

	slug := slug.Slugify(req.Name)
	if slug == "" {
		s.logger.WarnContext(ctx, "Empty slug")
		return "", apperror.NewBadRequestError("schema.service.Insert", "Slug is required", nil)
	}

	networkID := strings.TrimSpace(req.NetworkID)
	if networkID == "" {
		s.logger.WarnContext(ctx, "Empty network ID")
		return "", apperror.NewBadRequestError("schema.service.Insert", "Network ID is required", nil)
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		s.logger.WarnContext(ctx, "Empty user ID")
		return "", apperror.NewBadRequestError("schema.service.Insert", "User ID is required", nil)
	}

	schema := &schema{
		Name:       name,
		Slug:       slug,
		Definition: req.Definition,
		NetworkID:  networkID,
		UserID:     userID,
	}

	id, err := s.store.Insert(ctx, schema)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate schema", logger.KeySlug, slug)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert schema", logger.KeySlug, slug, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created schema", logger.KeyID, id)
	return id, nil
}
