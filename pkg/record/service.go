package record

import (
	"context"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/schema"
	"github.com/lutia-io/huma/pkg/workflow"
)

type service struct {
	logger          *logger.Logger
	store           store
	schemaService   *schema.Service
	workflowService *workflow.Service
}

func NewService(logger *logger.Logger, store store, schemaService *schema.Service, workflowService *workflow.Service) *service {
	return &service{
		logger:          logger,
		store:           store,
		schemaService:   schemaService,
		workflowService: workflowService,
	}
}

func (s *service) Insert(ctx context.Context, req insertRecordRequest) (string, error) {
	if len(req.Data) == 0 {
		s.logger.WarnContext(ctx, "Empty data")
		return "", apperror.NewBadRequestError("Data is required", nil)
	}

	schemaID := strings.TrimSpace(req.SchemaID)
	if schemaID == "" {
		s.logger.WarnContext(ctx, "Empty schema ID")
		return "", apperror.NewBadRequestError("Schema ID is required", nil)
	}

	organizationID := strings.TrimSpace(req.OrganizationID)
	if organizationID == "" {
		s.logger.WarnContext(ctx, "Empty organization ID")
		return "", apperror.NewBadRequestError("Organization ID is required", nil)
	}

	organizationUserID := strings.TrimSpace(req.OrganizationUserID)
	if organizationUserID == "" {
		s.logger.WarnContext(ctx, "Empty organization user ID")
		return "", apperror.NewBadRequestError("Organization user ID is required", nil)
	}

	if err := s.schemaService.ValidateRecordData(ctx, schemaID, req.Data); err != nil {
		return "", err
	}

	record := &record{
		Data:               req.Data,
		SchemaID:           schemaID,
		OrganizationID:     organizationID,
		OrganizationUserID: organizationUserID,
	}

	id, err := s.store.Insert(ctx, record)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to insert record", "schema_id", schemaID, "organization_id", organizationID, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created record", logger.KeyID, id)

	if err := s.workflowService.ExecuteForRecord(ctx, schemaID, id, record.Data); err != nil {
		s.logger.ErrorContext(ctx, "Failed to execute workflows after record insert", logger.KeyID, id, "schema_id", schemaID, logger.KeyError, err)
	}

	return id, nil
}
