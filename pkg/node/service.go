package node

import (
	"context"
	"encoding/json"
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

func (s *Service) Insert(ctx context.Context, req insertNodeDefinitionRequest) (string, error) {
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

	if req.Type == "" {
		s.logger.WarnContext(ctx, "Empty type")
		return "", apperror.NewBadRequestError("Type is required", nil)
	}

	def, err := ParseDefinition(req.Type, req.Definition)
	if err != nil {
		s.logger.WarnContext(ctx, "Invalid definition", "type", req.Type, logger.KeyError, err)
		return "", apperror.NewBadRequestError("Invalid definition", err)
	}

	n := &NodeDefinition{
		Name:       name,
		Slug:       slug,
		Active:     req.Active,
		Internal:   req.Internal,
		Type:       req.Type,
		Definition: def,
		NetworkID:  networkID,
		UserID:     userID,
	}

	id, err := s.store.Insert(ctx, n)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate node definition", logger.KeySlug, slug)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert node definition", logger.KeySlug, slug, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created node definition", logger.KeyID, id)
	return id, nil
}

func (s *Service) Patch(ctx context.Context, existing *NodeDefinition, req patchNodeDefinitionRequest) error {
	if existing.Internal {
		return apperror.NewBadRequestError("Internal node definitions cannot be updated", nil)
	}

	if req.Name == nil && req.Active == nil && req.Type == nil && req.Definition == nil {
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

	nextType := existing.Type
	if req.Type != nil {
		if *req.Type == "" {
			return apperror.NewBadRequestError("Type is required", nil)
		}
		nextType = *req.Type
	}

	if req.Definition != nil || req.Type != nil {
		var raw json.RawMessage
		if req.Definition != nil {
			raw = *req.Definition
		} else {
			encoded, err := json.Marshal(existing.Definition)
			if err != nil {
				return err
			}
			raw = encoded
		}
		def, err := ParseDefinition(nextType, raw)
		if err != nil {
			s.logger.WarnContext(ctx, "Invalid definition", "type", nextType, logger.KeyError, err)
			return apperror.NewBadRequestError("Invalid definition", err)
		}
		existing.Type = nextType
		existing.Definition = def
	}

	if err := s.store.Update(ctx, existing); err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && (appErr.Variant == apperror.ErrorVariantConflict || appErr.Variant == apperror.ErrorVariantNotFound) {
			s.logger.WarnContext(ctx, "Rejected node definition update", logger.KeyID, existing.ID, logger.KeyError, err)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to update node definition", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated node definition", logger.KeyID, existing.ID)
	return nil
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
		s.logger.ErrorContext(ctx, "Failed to list node definitions", logger.KeyUserID, p.ID, logger.KeyError, err)
		return nil, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, p principal.Principal, id string) (*NodeDefinition, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid node definition ID", nil)
	}

	n, err := s.store.GetByID(ctx, id)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get node definition", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	switch p.Type {
	case principal.TypeUser:
		if n.UserID != p.ID {
			return nil, apperror.NewNotFoundError("Node definition not found", nil)
		}
	case principal.TypeOrganizationUser:
		if n.NetworkID != p.NetworkID {
			return nil, apperror.NewNotFoundError("Node definition not found", nil)
		}
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	return n, nil
}

// ResolveActive returns the given node definitions if every ID exists, is
// active, and belongs to networkID. Results are not ordered.
func (s *Service) ResolveActive(ctx context.Context, networkID string, ids []string) (map[string]*NodeDefinition, error) {
	found, err := s.store.GetByIDs(ctx, ids)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to load node definitions", logger.KeyError, err)
		return nil, err
	}
	byID := make(map[string]*NodeDefinition, len(found))
	for _, n := range found {
		byID[n.ID] = n
	}
	for _, id := range ids {
		n, ok := byID[id]
		if !ok {
			return nil, apperror.NewBadRequestError("Node definition not found: "+id, nil)
		}
		if n.NetworkID != networkID {
			return nil, apperror.NewBadRequestError("Node definition not found: "+id, nil)
		}
		if !n.Active {
			return nil, apperror.NewBadRequestError("Node definition is not active: "+id, nil)
		}
	}
	return byID, nil
}
