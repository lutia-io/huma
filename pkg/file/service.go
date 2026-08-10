package file

import (
	"context"
	"errors"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/nats-io/nats.go/jetstream"
)

// Service is the file domain API. HTTP handlers and (later) pipeline nodes
// both call into it; SQL lives in the store and bytes live in object storage.
type Service struct {
	logger *logger.Logger
	store  store
	objs   jetstream.ObjectStore
}

func NewService(logger *logger.Logger, store store, objs jetstream.ObjectStore) *Service {
	return &Service{
		logger: logger,
		store:  store,
		objs:   objs,
	}
}

// Create validates metadata, reserves a row, then streams content into the
// object store keyed by file ID.
//
// When IdempotencyKey is set and a row already exists, content is not
// re-uploaded and the existing ID is returned.
func (s *Service) Create(ctx context.Context, params CreateParams) (string, error) {
	filename := strings.TrimSpace(params.Filename)
	if filename == "" {
		s.logger.WarnContext(ctx, "Empty filename")
		return "", apperror.NewBadRequestError("Filename is required", nil)
	}

	contentType := strings.TrimSpace(params.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
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

	if params.Content == nil {
		s.logger.WarnContext(ctx, "Empty content")
		return "", apperror.NewBadRequestError("Content is required", nil)
	}

	id, created, err := s.store.Insert(ctx, &File{
		Filename:           filename,
		ContentType:        contentType,
		SizeBytes:          0,
		OrganizationID:     organizationID,
		OrganizationUserID: organizationUserID,
		NetworkID:          networkID,
		IdempotencyKey:     params.IdempotencyKey,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to insert file metadata", "organization_id", organizationID, logger.KeyError, err)
		return "", err
	}

	// Idempotent replay of a completed upload: skip put.
	if !created {
		if _, err := s.objs.GetInfo(ctx, id); err == nil {
			s.logger.InfoContext(ctx, "Resolved file via idempotency key", logger.KeyID, id)
			return id, nil
		}
		// Metadata exists but object is missing (crash mid-upload); fall through
		// and store content under the existing ID.
		s.logger.WarnContext(ctx, "Repairing file missing object store content", logger.KeyID, id)
	}

	info, err := s.objs.Put(ctx, jetstream.ObjectMeta{
		Name:        id,
		Description: filename,
		Metadata: map[string]string{
			"filename":     filename,
			"content_type": contentType,
		},
	}, params.Content)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to put file object", logger.KeyID, id, logger.KeyError, err)
		if delErr := s.store.Delete(ctx, id); delErr != nil {
			s.logger.ErrorContext(ctx, "Failed to delete file metadata after object put failure", logger.KeyID, id, logger.KeyError, delErr)
		}
		return "", apperror.NewInternalError("Failed to store file content", err)
	}

	if err := s.store.UpdateSize(ctx, id, int64(info.Size)); err != nil {
		s.logger.ErrorContext(ctx, "Failed to update file size", logger.KeyID, id, logger.KeyError, err)
		return "", err
	}

	s.logger.InfoContext(ctx, "Successfully created file", logger.KeyID, id, "size_bytes", info.Size)
	return id, nil
}

// GetMeta returns file metadata, or found=false if missing/deleted.
func (s *Service) GetMeta(ctx context.Context, fileID string) (*File, bool, error) {
	return s.store.Get(ctx, fileID)
}

// OpenContent returns metadata and a reader for the object bytes.
// The caller must Close the reader.
func (s *Service) OpenContent(ctx context.Context, fileID string) (*Content, bool, error) {
	meta, found, err := s.store.Get(ctx, fileID)
	if err != nil || !found {
		return nil, found, err
	}

	result, err := s.objs.Get(ctx, fileID)
	if err != nil {
		if errors.Is(err, jetstream.ErrObjectNotFound) {
			s.logger.ErrorContext(ctx, "File metadata exists but object is missing", logger.KeyID, fileID)
			return nil, false, nil
		}
		s.logger.ErrorContext(ctx, "Failed to get file object", logger.KeyID, fileID, logger.KeyError, err)
		return nil, false, err
	}

	return &Content{File: meta, Reader: result}, true, nil
}

// SoftDelete marks the file deleted in Postgres and removes the object from
// the store. Returns found=false when the file is missing or already deleted.
func (s *Service) SoftDelete(ctx context.Context, fileID string) (bool, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return false, apperror.NewBadRequestError("File ID is required", nil)
	}

	found, err := s.store.SoftDelete(ctx, fileID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to soft-delete file metadata", logger.KeyID, fileID, logger.KeyError, err)
		return false, err
	}
	if !found {
		return false, nil
	}

	if err := s.objs.Delete(ctx, fileID); err != nil && !errors.Is(err, jetstream.ErrObjectNotFound) {
		s.logger.ErrorContext(ctx, "Failed to delete file object", logger.KeyID, fileID, logger.KeyError, err)
		return false, apperror.NewInternalError("Failed to delete file content", err)
	}

	s.logger.InfoContext(ctx, "Successfully deleted file", logger.KeyID, fileID)
	return true, nil
}
