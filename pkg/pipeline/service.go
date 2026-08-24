package pipeline

import (
	"context"
	"errors"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/slug"
	"github.com/lutia-io/huma/pkg/uuid"
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

func (s *Service) Insert(ctx context.Context, req insertPipelineDefinitionRequest) (string, error) {
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

	pipeline := &pipelineDefinition{
		Name:       name,
		Slug:       slug,
		Active:     req.Active,
		Internal:   req.Internal,
		Definition: req.Definition,
		NetworkID:  networkID,
		UserID:     userID,
	}

	id, err := s.store.Insert(ctx, pipeline)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate pipeline definition", logger.KeySlug, slug)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert pipeline definition", logger.KeySlug, slug, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created pipeline definition", logger.KeyID, id)
	return id, nil
}

func (s *Service) List(ctx context.Context, p principal.Principal, params listParams) (*listResult, error) {
	switch p.Type {
	case principal.TypeUser:
		params.UserID = p.ID
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" {
			return nil, apperror.NewUnauthorizedError("Organization user token missing network", nil)
		}
		params.UserID = ""
		params.NetworkID = p.NetworkID
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	result, err := s.store.List(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list pipeline definitions", logger.KeyUserID, p.ID, logger.KeyError, err)
		return nil, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, p principal.Principal, id string) (*pipelineDefinition, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid pipeline definition ID", nil)
	}

	pipeline, err := s.store.GetByID(ctx, id)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get pipeline definition", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	switch p.Type {
	case principal.TypeUser:
		if pipeline.UserID != p.ID {
			return nil, apperror.NewNotFoundError("Pipeline definition not found", nil)
		}
	case principal.TypeOrganizationUser:
		if pipeline.NetworkID != p.NetworkID {
			return nil, apperror.NewNotFoundError("Pipeline definition not found", nil)
		}
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	return pipeline, nil
}
