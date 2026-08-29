package schema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/schema/validator"
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

// Definition returns the JSON Schema document for a schema ID.
func (s *Service) Definition(ctx context.Context, schemaID string) (json.RawMessage, error) {
	if !uuid.Valid(schemaID) {
		return nil, apperror.NewBadRequestError("Invalid schema ID", nil)
	}
	sch, err := s.store.GetByID(ctx, schemaID)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to load schema", "schema_id", schemaID, logger.KeyError, err)
		return nil, err
	}
	return sch.Definition, nil
}

func (s *Service) ValidateRecordData(ctx context.Context, schemaID string, data json.RawMessage) error {
	sch, err := s.store.GetByID(ctx, schemaID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to load schema", "schema_id", schemaID, logger.KeyError, err)
		return err
	}
	if !sch.Active {
		s.logger.WarnContext(ctx, "Inactive schema", "schema_id", schemaID)
		return apperror.NewBadRequestError("Schema is not active", nil)
	}
	if err := validator.ValidateData(sch.Definition, data); err != nil {
		s.logger.WarnContext(ctx, "Invalid record data", "schema_id", schemaID, logger.KeyError, err)
		return apperror.NewBadRequestError(err.Error(), err)
	}
	return nil
}

func optionalID(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func visibleToOrganization(sch *schema, networkID, organizationID string) bool {
	if sch.NetworkID != networkID {
		return false
	}
	if sch.OrganizationID == nil {
		return true
	}
	return *sch.OrganizationID == organizationID
}

func (s *Service) Insert(ctx context.Context, req insertSchemaRequest) (string, error) {
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

	organizationID := optionalID(req.OrganizationID)
	if organizationID != nil && !uuid.Valid(*organizationID) {
		s.logger.WarnContext(ctx, "Invalid organization ID")
		return "", apperror.NewBadRequestError("Invalid organization ID", nil)
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
	if err := validator.ValidateDefinition(req.Definition); err != nil {
		s.logger.WarnContext(ctx, "Invalid definition", logger.KeyError, err)
		return "", apperror.NewBadRequestError(err.Error(), err)
	}
	if err := s.validateForeignTargets(ctx, req.Definition, networkID, organizationID); err != nil {
		return "", err
	}

	schema := &schema{
		Name:           name,
		Slug:           slug,
		Active:         req.Active,
		Internal:       req.Internal,
		Definition:     req.Definition,
		NetworkID:      networkID,
		OrganizationID: organizationID,
		UserID:         userID,
	}

	id, err := s.store.Insert(ctx, schema)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && (appErr.Variant == apperror.ErrorVariantConflict || appErr.Variant == apperror.ErrorVariantBadRequest) {
			s.logger.WarnContext(ctx, "Rejected schema insert", logger.KeySlug, slug, logger.KeyError, err)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert schema", logger.KeySlug, slug, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created schema", logger.KeyID, id)
	return id, nil
}

func (s *Service) Patch(ctx context.Context, existing *schema, req patchSchemaRequest) error {
	if existing.Internal {
		return apperror.NewBadRequestError("Internal schemas cannot be updated", nil)
	}

	if req.Name == nil && req.Active == nil && len(req.Definition) == 0 {
		return apperror.NewBadRequestError("No fields to update", nil)
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			s.logger.WarnContext(ctx, "Empty name")
			return apperror.NewBadRequestError("Name is required", nil)
		}
		slug := slug.Slugify(name)
		if slug == "" {
			s.logger.WarnContext(ctx, "Empty slug")
			return apperror.NewBadRequestError("Slug is required", nil)
		}
		existing.Name = name
		existing.Slug = slug
	}

	if req.Active != nil {
		existing.Active = *req.Active
	}

	if len(req.Definition) > 0 {
		if string(req.Definition) == "null" {
			s.logger.WarnContext(ctx, "Empty definition")
			return apperror.NewBadRequestError("Definition is required", nil)
		}
		if err := validator.ValidateDefinition(req.Definition); err != nil {
			s.logger.WarnContext(ctx, "Invalid definition", logger.KeyError, err)
			return apperror.NewBadRequestError(err.Error(), err)
		}
		if err := s.validateForeignTargets(ctx, req.Definition, existing.NetworkID, existing.OrganizationID); err != nil {
			return err
		}
		existing.Definition = req.Definition
	}

	if err := s.store.Update(ctx, existing); err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && (appErr.Variant == apperror.ErrorVariantConflict || appErr.Variant == apperror.ErrorVariantBadRequest) {
			s.logger.WarnContext(ctx, "Rejected schema update", logger.KeyID, existing.ID, logger.KeyError, err)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to update schema", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated schema", logger.KeyID, existing.ID)
	return nil
}

func (s *Service) List(ctx context.Context, p principal.Principal, params listParams) (*listResult, error) {
	switch p.Type {
	case principal.TypeUser:
		params.UserID = p.ID
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" || p.OrganizationID == "" {
			return nil, apperror.NewUnauthorizedError("Organization user token missing network or organization", nil)
		}
		params.UserID = ""
		params.NetworkID = p.NetworkID
		params.OrganizationID = p.OrganizationID
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	result, err := s.store.List(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list schemas", logger.KeyUserID, p.ID, logger.KeyError, err)
		return nil, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, p principal.Principal, id string) (*schema, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid schema ID", nil)
	}

	sch, err := s.store.GetByID(ctx, id)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get schema", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	switch p.Type {
	case principal.TypeUser:
		if sch.UserID != p.ID {
			return nil, apperror.NewNotFoundError("Schema not found", nil)
		}
	case principal.TypeOrganizationUser:
		if !visibleToOrganization(sch, p.NetworkID, p.OrganizationID) {
			return nil, apperror.NewNotFoundError("Schema not found", nil)
		}
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	return sch, nil
}

// Snapshot is the subset of a schema needed to resolve foreign fields.
type Snapshot struct {
	ID             string
	Name           string
	Definition     json.RawMessage
	NetworkID      string
	OrganizationID *string
}

// SnapshotByID returns the schema, or a not-found error.
func (s *Service) SnapshotByID(ctx context.Context, schemaID string) (*Snapshot, error) {
	if !uuid.Valid(schemaID) {
		return nil, apperror.NewBadRequestError("Invalid schema ID", nil)
	}
	sch, err := s.store.GetByID(ctx, schemaID)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to load schema", "schema_id", schemaID, logger.KeyError, err)
		return nil, err
	}
	return &Snapshot{
		ID:             sch.ID,
		Name:           sch.Name,
		Definition:     sch.Definition,
		NetworkID:      sch.NetworkID,
		OrganizationID: sch.OrganizationID,
	}, nil
}

func (s *Service) validateForeignTargets(ctx context.Context, definition json.RawMessage, networkID string, organizationID *string) error {
	fields, err := validator.ForeignFields(definition)
	if err != nil {
		return apperror.NewBadRequestError(err.Error(), err)
	}
	for _, field := range fields {
		target, err := s.store.GetByID(ctx, field.SchemaID)
		if err != nil {
			var appErr *apperror.Error
			if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
				s.logger.WarnContext(ctx, "Unknown foreign schema", "schema_id", field.SchemaID, "property", field.Name)
				return apperror.NewBadRequestError(fmt.Sprintf("property %q: related schema not found", field.Name), err)
			}
			s.logger.ErrorContext(ctx, "Failed to load foreign schema", "schema_id", field.SchemaID, logger.KeyError, err)
			return err
		}
		if !visibleAsForeignTarget(target, networkID, organizationID) {
			s.logger.WarnContext(ctx, "Foreign schema not visible", "schema_id", field.SchemaID, "property", field.Name)
			return apperror.NewBadRequestError(fmt.Sprintf("property %q: related schema not found", field.Name), nil)
		}
	}
	return nil
}

func visibleAsForeignTarget(target *schema, networkID string, organizationID *string) bool {
	if target.NetworkID != networkID {
		return false
	}
	if target.OrganizationID == nil {
		return true
	}
	if organizationID == nil {
		return false
	}
	return *target.OrganizationID == *organizationID
}
