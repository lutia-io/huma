package workflow

import (
	"context"
	"errors"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/slug"
)

type Service struct {
	logger *logger.Logger
	store  store
}

func NewService(logger *logger.Logger, store store) *Service {
	return &Service{
		logger: logger,
		store:  store,
	}
}

func (s *Service) Insert(ctx context.Context, req insertWorkflowDefinitionRequest) (string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		s.logger.WarnContext(ctx, "Empty name")
		return "", apperror.NewBadRequestError("Name is required", nil)
	}

	slug := slug.Slugify(req.Name)
	if slug == "" {
		s.logger.WarnContext(ctx, "Empty slug")
		return "", apperror.NewBadRequestError("Slug is required", nil)
	}

	networkID := strings.TrimSpace(req.NetworkID)
	if networkID == "" {
		s.logger.WarnContext(ctx, "Empty network ID")
		return "", apperror.NewBadRequestError("Network ID is required", nil)
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		s.logger.WarnContext(ctx, "Empty user ID")
		return "", apperror.NewBadRequestError("User ID is required", nil)
	}

	if len(req.Definition) == 0 {
		s.logger.WarnContext(ctx, "Empty definition")
		return "", apperror.NewBadRequestError("Definition is required", nil)
	}

	schemaID := strings.TrimSpace(req.SchemaID)
	if schemaID == "" {
		s.logger.WarnContext(ctx, "Empty schema ID")
		return "", apperror.NewBadRequestError("Schema ID is required", nil)
	}

	workflow := &workflowDefinition{
		Name:       name,
		Slug:       slug,
		Active:     req.Active,
		Internal:   req.Internal,
		Definition: req.Definition,
		SchemaID:   schemaID,
		NetworkID:  networkID,
		UserID:     userID,
	}

	id, err := s.store.Insert(ctx, workflow)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate workflow definition", logger.KeySlug, slug)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert workflow definition", logger.KeySlug, slug, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created workflow definition", logger.KeyID, id)
	return id, nil
}
