package network

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

func (s *service) Insert(ctx context.Context, req insertNetworkRequest) (string, error) {
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

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		s.logger.WarnContext(ctx, "Empty user ID")
		return "", apperror.NewBadRequestError("User ID is required", nil)
	}

	network := &network{
		Name:   name,
		Slug:   slug,
		UserID: userID,
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

func (s *service) Patch(ctx context.Context, existing *network, req patchNetworkRequest) error {
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
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantConflict {
			s.logger.WarnContext(ctx, "Rejected duplicate network", logger.KeySlug, slug)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to update network", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated network", logger.KeyID, existing.ID)
	return nil
}

func (s *service) Delete(ctx context.Context, existing *network) error {
	if err := s.store.Delete(ctx, existing.ID); err != nil {
		s.logger.ErrorContext(ctx, "Failed to delete network", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully deleted network", logger.KeyID, existing.ID)
	return nil
}

func (s *service) List(ctx context.Context, p principal.Principal) ([]*network, error) {
	switch p.Type {
	case principal.TypeUser:
		networks, err := s.store.ListByUserID(ctx, p.ID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to list networks", logger.KeyUserID, p.ID, logger.KeyError, err)
			return nil, err
		}
		return networks, nil
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" {
			return nil, apperror.NewUnauthorizedError("Organization user token missing network", nil)
		}
		n, err := s.Get(ctx, p, p.NetworkID)
		if err != nil {
			return nil, err
		}
		return []*network{n}, nil
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}
}

func (s *service) Get(ctx context.Context, p principal.Principal, id string) (*network, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid network ID", nil)
	}

	n, err := s.store.GetByID(ctx, id)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get network", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	switch p.Type {
	case principal.TypeUser:
		if n.UserID != p.ID {
			return nil, apperror.NewNotFoundError("Network not found", nil)
		}
	case principal.TypeOrganizationUser:
		if p.NetworkID != n.ID {
			return nil, apperror.NewNotFoundError("Network not found", nil)
		}
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	return n, nil
}
