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

type listParams struct {
	UserID         string
	Query          string
	NetworkID      string
	OrganizationID string
	Filename       string
	FilenameOp     string
	ContentType    string
	SizeBytes      *int64
	SizeBytesOp    string
	Organization   string
	OrganizationOp string
	Sort           string
	Order          string
	Page           int
	PageSize       int
}

type listResult struct {
	Items    []*File `json:"items"`
	Total    int     `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
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
