package file

import (
	"io"
	"time"
)

// ObjectStoreBucket is the NATS object-store bucket for file blobs.
const ObjectStoreBucket = "FILES"

// File is metadata for a blob stored in the object store.
// The object name in the store matches ID.
type File struct {
	ID string `json:"id"`

	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`

	OrganizationID     string `json:"organizationId"`
	OrganizationUserID string `json:"organizationUserId"`
	NetworkID          string `json:"networkId"`
	IdempotencyKey     string `json:"-"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`

	// UserID is the owning network's user, used for authorization only.
	UserID string `json:"-"`
}

// CreateParams is the input to Service.Create. HTTP may leave IdempotencyKey
// empty; workflow/pipeline callers can set a deterministic key so crash
// replays do not duplicate uploads.
type CreateParams struct {
	Filename           string
	ContentType        string
	OrganizationID     string
	OrganizationUserID string
	NetworkID          string
	IdempotencyKey     string
	Content            io.Reader
}

// Content is an open object-store reader plus the file's metadata.
type Content struct {
	File   *File
	Reader io.ReadCloser
}
