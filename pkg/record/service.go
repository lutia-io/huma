package record

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/schema"
	"github.com/lutia-io/huma/pkg/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// Service is the record domain API. HTTP handlers and the workflow engine
// both call into it; SQL lives only in the store.
type Service struct {
	logger        *logger.Logger
	store         store
	js            jetstream.JetStream
	schemaService *schema.Service
}

func NewService(logger *logger.Logger, store store, js jetstream.JetStream, schemaService *schema.Service) *Service {
	return &Service{
		logger:        logger,
		store:         store,
		js:            js,
		schemaService: schemaService,
	}
}

// Create validates and inserts a record, then publishes records.created.
// When IdempotencyKey is set, a replay resolves to the existing record and
// the event is published with that key as the JetStream message ID.
func (s *Service) Create(ctx context.Context, params CreateParams) (string, error) {
	if len(params.Data) == 0 {
		s.logger.WarnContext(ctx, "Empty data")
		return "", apperror.NewBadRequestError("Data is required", nil)
	}

	schemaID := strings.TrimSpace(params.SchemaID)
	if schemaID == "" {
		s.logger.WarnContext(ctx, "Empty schema ID")
		return "", apperror.NewBadRequestError("Schema ID is required", nil)
	}

	organizationID := strings.TrimSpace(params.OrganizationID)
	if organizationID == "" {
		s.logger.WarnContext(ctx, "Empty organization ID")
		return "", apperror.NewBadRequestError("Organization ID is required", nil)
	}

	organizationUserID := strings.TrimSpace(params.OrganizationUserID)
	if organizationUserID == "" {
		s.logger.WarnContext(ctx, "Empty organization user ID")
		return "", apperror.NewBadRequestError("Organization user ID is required", nil)
	}

	networkID := strings.TrimSpace(params.NetworkID)
	if networkID == "" {
		s.logger.WarnContext(ctx, "Empty network ID")
		return "", apperror.NewBadRequestError("Network ID is required", nil)
	}

	if err := s.schemaService.ValidateRecordData(ctx, schemaID, params.Data); err != nil {
		return "", err
	}

	rec := &Record{
		Data:               params.Data,
		SchemaID:           schemaID,
		OrganizationID:     organizationID,
		OrganizationUserID: organizationUserID,
		NetworkID:          networkID,
		IdempotencyKey:     params.IdempotencyKey,
	}

	id, err := s.store.Insert(ctx, rec)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to insert record", "schema_id", schemaID, "organization_id", organizationID, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created record", logger.KeyID, id)

	payload, err := json.Marshal(CreatedEvent{
		ID:                 id,
		Data:               rec.Data,
		SchemaID:           rec.SchemaID,
		OrganizationID:     rec.OrganizationID,
		OrganizationUserID: rec.OrganizationUserID,
		NetworkID:          rec.NetworkID,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to marshal record created event", logger.KeyError, err)
		return id, nil
	}

	msgID := id
	if params.IdempotencyKey != "" {
		msgID = params.IdempotencyKey
	}
	if _, err := s.js.Publish(ctx, SubjectCreated, payload, jetstream.WithMsgID(msgID)); err != nil {
		s.logger.ErrorContext(ctx, "Failed to publish record created event", logger.KeyID, id, logger.KeyError, err)
	}

	return id, nil
}

// Get returns the record, or found=false if it does not exist or is deleted.
func (s *Service) Get(ctx context.Context, recordID string) (*Record, bool, error) {
	return s.store.Get(ctx, recordID)
}

func (s *Service) List(ctx context.Context, p principal.Principal) ([]*Record, error) {
	switch p.Type {
	case principal.TypeUser:
		records, err := s.store.ListByUserID(ctx, p.ID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to list records", logger.KeyUserID, p.ID, logger.KeyError, err)
			return nil, err
		}
		return records, nil
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" || p.OrganizationID == "" {
			return nil, apperror.NewUnauthorizedError("Organization user token missing network or organization", nil)
		}
		records, err := s.store.ListByOrganization(ctx, p.NetworkID, p.OrganizationID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to list records", logger.KeyError, err)
			return nil, err
		}
		return records, nil
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}
}

func (s *Service) GetVisible(ctx context.Context, p principal.Principal, id string) (*Record, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid record ID", nil)
	}

	rec, err := s.store.GetByID(ctx, id)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get record", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	switch p.Type {
	case principal.TypeUser:
		if rec.UserID != p.ID {
			return nil, apperror.NewNotFoundError("Record not found", nil)
		}
	case principal.TypeOrganizationUser:
		if rec.NetworkID != p.NetworkID || rec.OrganizationID != p.OrganizationID {
			return nil, apperror.NewNotFoundError("Record not found", nil)
		}
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	return rec, nil
}

// UpdateData replaces the record's data. Callers pass the full merged
// document, so the write is set-to-value and naturally idempotent.
func (s *Service) UpdateData(ctx context.Context, recordID string, data json.RawMessage) (bool, error) {
	return s.store.UpdateData(ctx, recordID, data)
}
