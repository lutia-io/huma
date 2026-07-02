package schema

import (
	"context"
	"encoding/json"
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

func (s *Service) ValidateRecordData(ctx context.Context, schemaID string, data json.RawMessage) error {
	sch, err := s.store.GetByID(ctx, schemaID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to load schema", "schema_id", schemaID, logger.KeyError, err)
		return err
	}
	if !sch.Active {
		s.logger.WarnContext(ctx, "Inactive schema", "schema_id", schemaID)
		return apperror.NewBadRequestError("schema.service.ValidateRecord", "Schema is not active", nil)
	}
	if err := ValidateData(sch.Definition, data); err != nil {
		s.logger.WarnContext(ctx, "Invalid record data", "schema_id", schemaID, logger.KeyError, err)
		return apperror.NewBadRequestError("schema.service.ValidateRecord", err.Error(), err)
	}
	return nil
}

func (s *Service) Insert(ctx context.Context, req insertSchemaRequest) (string, error) {
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

	if len(req.Definition) == 0 {
		s.logger.WarnContext(ctx, "Empty definition")
		return "", apperror.NewBadRequestError("schema.service.Insert", "Definition is required", nil)
	}
	if err := ValidateDefinition(req.Definition); err != nil {
		s.logger.WarnContext(ctx, "Invalid definition", logger.KeyError, err)
		return "", apperror.NewBadRequestError("schema.service.Insert", err.Error(), err)
	}

	schema := &schema{
		Name:       name,
		Slug:       slug,
		Active:     req.Active,
		Internal:   req.Internal,
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
