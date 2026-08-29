package record

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/schema"
	"github.com/lutia-io/huma/pkg/schema/validator"
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
	if err := s.validateForeignRefs(ctx, schemaID, params.NetworkID, params.OrganizationID, params.Data); err != nil {
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

	if err := s.resolveListParams(ctx, &params); err != nil {
		return nil, err
	}

	result, err := s.store.List(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list records", logger.KeyUserID, p.ID, logger.KeyError, err)
		return nil, err
	}
	related, err := s.relatedMap(ctx, result.Items)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to load related records", logger.KeyError, err)
		return nil, err
	}
	result.Related = related
	return result, nil
}

func (s *Service) resolveListParams(ctx context.Context, params *listParams) error {
	if params.SchemaID != "" {
		definition, err := s.schemaService.Definition(ctx, params.SchemaID)
		if err != nil {
			return err
		}
		fields, err := parseSchemaFields(definition)
		if err != nil {
			return apperror.NewBadRequestError("Invalid schema definition", err)
		}
		params.SchemaFields = fields
		if err := s.resolveForeignTitleKeys(ctx, params.SchemaFields); err != nil {
			return err
		}
	}

	if _, reserved := reservedSortColumns[params.Sort]; !reserved && params.SchemaFields != nil {
		if _, ok := params.SchemaFields[params.Sort]; !ok {
			return apperror.NewBadRequestError("Invalid sort field", nil)
		}
	}

	for i := range params.Fields {
		field := &params.Fields[i]
		if params.SchemaFields != nil {
			meta, ok := params.SchemaFields[field.Name]
			if !ok {
				return apperror.NewBadRequestError("Invalid filter field", nil)
			}
			field.Kind = meta.Kind
			field.TitleKey = meta.TitleKey
		} else if field.Op == opGte || field.Op == opLte {
			field.Kind = fieldKindNumber
		} else {
			field.Kind = fieldKindString
		}

		op, err := normalizeFieldOp(field.Kind, field.Op)
		if err != nil {
			return err
		}
		field.Op = op

		if field.Op == opEmpty {
			continue
		}

		switch field.Kind {
		case fieldKindNumber:
			n, convErr := strconv.ParseFloat(field.Value, 64)
			if convErr != nil {
				return apperror.NewBadRequestError("Invalid number filter", convErr)
			}
			field.NumberValue = &n
		case fieldKindBoolean:
			v, boolErr := parseBooleanFilter(field.Value)
			if boolErr != nil {
				return boolErr
			}
			field.BooleanValue = &v
		}
	}

	return nil
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

func (s *Service) GetVisibleWithRelated(ctx context.Context, p principal.Principal, id string) (*Record, map[string]RelatedRecord, error) {
	rec, err := s.GetVisible(ctx, p, id)
	if err != nil {
		return nil, nil, err
	}
	related, err := s.relatedMap(ctx, []*Record{rec})
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to load related records", logger.KeyID, id, logger.KeyError, err)
		return nil, nil, err
	}
	return rec, related, nil
}

// PatchData validates data against the record's schema and replaces it.
func (s *Service) PatchData(ctx context.Context, rec *Record, data json.RawMessage) error {
	if len(data) == 0 || string(data) == "null" {
		s.logger.WarnContext(ctx, "Empty data")
		return apperror.NewBadRequestError("Data is required", nil)
	}
	if err := s.schemaService.ValidateRecordData(ctx, rec.SchemaID, data); err != nil {
		return err
	}
	if err := s.validateForeignRefs(ctx, rec.SchemaID, rec.NetworkID, rec.OrganizationID, data); err != nil {
		return err
	}
	found, err := s.store.UpdateData(ctx, rec.ID, data)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to update record", logger.KeyID, rec.ID, logger.KeyError, err)
		return err
	}
	if !found {
		return apperror.NewNotFoundError("Record not found", nil)
	}
	s.logger.InfoContext(ctx, "Successfully updated record", logger.KeyID, rec.ID)
	return nil
}

// UpdateData replaces the record's data. Callers pass the full merged
// document, so the write is set-to-value and naturally idempotent.
func (s *Service) UpdateData(ctx context.Context, recordID string, data json.RawMessage) (bool, error) {
	return s.store.UpdateData(ctx, recordID, data)
}

func (s *Service) resolveForeignTitleKeys(ctx context.Context, fields map[string]schemaField) error {
	for name, meta := range fields {
		if meta.Kind != fieldKindForeign || meta.SchemaID == "" {
			continue
		}
		snap, err := s.schemaService.SnapshotByID(ctx, meta.SchemaID)
		if err != nil {
			var appErr *apperror.Error
			if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
				continue
			}
			return err
		}
		meta.TitleKey = validator.TitleKey(snap.Definition)
		fields[name] = meta
	}
	return nil
}

func (s *Service) validateForeignRefs(ctx context.Context, schemaID, networkID, organizationID string, data json.RawMessage) error {
	definition, err := s.schemaService.Definition(ctx, schemaID)
	if err != nil {
		return err
	}
	fields, err := validator.ForeignFields(definition)
	if err != nil {
		return apperror.NewBadRequestError("Invalid schema definition", err)
	}
	if len(fields) == 0 {
		return nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return apperror.NewBadRequestError("Invalid data", err)
	}

	ids := make([]string, 0)
	seen := make(map[string]struct{})
	type ref struct {
		name     string
		schemaID string
		recordID string
	}
	refs := make([]ref, 0)
	for _, field := range fields {
		raw, ok := obj[field.Name]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		refs = append(refs, ref{name: field.Name, schemaID: field.SchemaID, recordID: value})
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}
	if len(refs) == 0 {
		return nil
	}

	records, err := s.store.GetByIDs(ctx, ids)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to load foreign records", logger.KeyError, err)
		return err
	}
	byID := make(map[string]*Record, len(records))
	for _, rec := range records {
		byID[rec.ID] = rec
	}

	for _, field := range refs {
		target, ok := byID[field.recordID]
		if !ok {
			s.logger.WarnContext(ctx, "Unknown foreign record", "property", field.name, logger.KeyID, field.recordID)
			return apperror.NewBadRequestError(fmt.Sprintf("%s must reference an existing record", field.name), nil)
		}
		if target.NetworkID != networkID || target.OrganizationID != organizationID {
			s.logger.WarnContext(ctx, "Foreign record out of scope", "property", field.name, logger.KeyID, field.recordID)
			return apperror.NewBadRequestError(fmt.Sprintf("%s must reference a record in this organization", field.name), nil)
		}
		if target.SchemaID != field.schemaID {
			s.logger.WarnContext(ctx, "Foreign record schema mismatch", "property", field.name, logger.KeyID, field.recordID)
			return apperror.NewBadRequestError(fmt.Sprintf("%s must reference a record of the related schema", field.name), nil)
		}
	}
	return nil
}

func (s *Service) relatedMap(ctx context.Context, records []*Record) (map[string]RelatedRecord, error) {
	related := make(map[string]RelatedRecord)
	if len(records) == 0 {
		return related, nil
	}

	defCache := make(map[string]json.RawMessage)
	definition := func(schemaID string) (json.RawMessage, error) {
		if def, ok := defCache[schemaID]; ok {
			return def, nil
		}
		def, err := s.schemaService.Definition(ctx, schemaID)
		if err != nil {
			return nil, err
		}
		defCache[schemaID] = def
		return def, nil
	}

	ids := make([]string, 0)
	seen := make(map[string]struct{})
	for _, rec := range records {
		def, err := definition(rec.SchemaID)
		if err != nil {
			var appErr *apperror.Error
			if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
				continue
			}
			return nil, err
		}
		fields, err := validator.ForeignFields(def)
		if err != nil {
			return nil, err
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(rec.Data, &obj); err != nil {
			continue
		}
		for _, field := range fields {
			raw, ok := obj[field.Name]
			if !ok {
				continue
			}
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				continue
			}
			value = strings.TrimSpace(value)
			if value == "" || !uuid.Valid(value) {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			ids = append(ids, value)
		}
	}
	if len(ids) == 0 {
		return related, nil
	}

	targets, err := s.store.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	snapCache := make(map[string]*schema.Snapshot)
	snapshot := func(schemaID string) *schema.Snapshot {
		if snap, ok := snapCache[schemaID]; ok {
			return snap
		}
		snap, err := s.schemaService.SnapshotByID(ctx, schemaID)
		if err != nil {
			return nil
		}
		snapCache[schemaID] = snap
		return snap
	}

	for _, target := range targets {
		snap := snapshot(target.SchemaID)
		fallback := "Record"
		var definition json.RawMessage
		if snap != nil {
			fallback = snap.Name
			definition = snap.Definition
		}
		related[target.ID] = RelatedRecord{
			ID:       target.ID,
			SchemaID: target.SchemaID,
			Title:    validator.DisplayTitle(target.Data, definition, fallback),
		}
	}
	return related, nil
}
