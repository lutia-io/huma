package organization

import (
	"context"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/slug"
	"github.com/lutia-io/huma/pkg/uuid"
)

type service struct {
	logger *logger.Logger
	store  store
	hasher hasher.Hasher
}

func newService(logger *logger.Logger, store store, hasher hasher.Hasher) *service {
	return &service{
		logger: logger,
		store:  store,
		hasher: hasher,
	}
}

func (s *service) Insert(ctx context.Context, req insertOrganizationRequest) (string, error) {
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

	organization := &organization{
		Name:      name,
		Slug:      slug,
		NetworkID: networkID,
		UserID:    userID,
	}

	id, err := s.store.Insert(ctx, organization)
	if err != nil {
		if apperror.IsConflict(err) {
			s.logger.WarnContext(ctx, "Rejected duplicate organization", logger.KeySlug, slug)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert organization", logger.KeySlug, slug, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created organization", logger.KeySlug, slug)
	return id, nil
}

func (s *service) Patch(ctx context.Context, existing *organization, req patchOrganizationRequest) error {
	if req.Name == nil {
		return apperror.NewBadRequestError("No fields to update", nil)
	}

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

	if err := s.store.Update(ctx, existing); err != nil {
		if apperror.IsConflict(err) {
			s.logger.WarnContext(ctx, "Rejected duplicate organization", logger.KeySlug, slug)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to update organization", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated organization", logger.KeyID, existing.ID)
	return nil
}

func (s *service) Delete(ctx context.Context, existing *organization) error {
	if err := s.store.Delete(ctx, existing.ID); err != nil {
		s.logger.ErrorContext(ctx, "Failed to delete organization", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully deleted organization", logger.KeyID, existing.ID)
	return nil
}

func (s *service) List(ctx context.Context, p principal.Principal, params listParams) (*listResult, error) {
	switch p.Type {
	case principal.TypeUser:
		params.UserID = p.ID
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" || p.OrganizationID == "" {
			return nil, apperror.NewForbiddenError("Organization user token missing network or organization", nil)
		}
		params.UserID = ""
		params.NetworkID = p.NetworkID
		params.OrganizationID = p.OrganizationID
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	result, err := s.store.List(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list organizations", logger.KeyUserID, p.ID, logger.KeyError, err)
		return nil, err
	}
	return result, nil
}

func (s *service) Get(ctx context.Context, p principal.Principal, id string) (*organization, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid organization ID", nil)
	}

	o, err := s.store.GetByID(ctx, id)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get organization", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	switch p.Type {
	case principal.TypeUser:
		if o.UserID != p.ID {
			return nil, apperror.NewNotFoundError("Organization not found", nil)
		}
	case principal.TypeOrganizationUser:
		if p.OrganizationID != o.ID {
			return nil, apperror.NewNotFoundError("Organization not found", nil)
		}
		if p.NetworkID != "" && p.NetworkID != o.NetworkID {
			return nil, apperror.NewNotFoundError("Organization not found", nil)
		}
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	return o, nil
}
