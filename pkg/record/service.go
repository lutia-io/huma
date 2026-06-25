package record

import (
	"context"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
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

func (s *service) Insert(ctx context.Context, req insertRecordRequest) (string, error) {
	if len(req.Data) == 0 {
		s.logger.WarnContext(ctx, "Empty data")
		return "", apperror.NewBadRequestError("record.service.Insert", "Data is required", nil)
	}

	schemaID := strings.TrimSpace(req.SchemaID)
	if schemaID == "" {
		s.logger.WarnContext(ctx, "Empty schema ID")
		return "", apperror.NewBadRequestError("record.service.Insert", "Schema ID is required", nil)
	}

	networkID := strings.TrimSpace(req.NetworkID)
	if networkID == "" {
		s.logger.WarnContext(ctx, "Empty network ID")
		return "", apperror.NewBadRequestError("record.service.Insert", "Network ID is required", nil)
	}

	organizationID := strings.TrimSpace(req.OrganizationID)
	if organizationID == "" {
		s.logger.WarnContext(ctx, "Empty organization ID")
		return "", apperror.NewBadRequestError("record.service.Insert", "Organization ID is required", nil)
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		s.logger.WarnContext(ctx, "Empty user ID")
		return "", apperror.NewBadRequestError("record.service.Insert", "User ID is required", nil)
	}

	record := &record{
		Data:           req.Data,
		SchemaID:       schemaID,
		NetworkID:      networkID,
		OrganizationID: organizationID,
		UserID:         userID,
	}

	id, err := s.store.Insert(ctx, record)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to insert record", "schema_id", schemaID, "organization_id", organizationID, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created record", logger.KeyID, id)
	return id, nil
}
