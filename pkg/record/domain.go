package record

import (
	"encoding/json"
	"time"
)

type Record struct {
	ID string `json:"id"`

	Data json.RawMessage `json:"data"`

	SchemaID           string `json:"schemaId"`
	OrganizationID     string `json:"organizationId"`
	OrganizationUserID string `json:"organizationUserId"`
	IdempotencyKey     string `json:"-"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// CreateParams is the input to Service.Create. HTTP always leaves
// IdempotencyKey empty; the workflow engine sets it to a deterministic action
// key so crash replays do not duplicate records.
type CreateParams struct {
	SchemaID           string
	OrganizationID     string
	OrganizationUserID string
	Data               json.RawMessage
	IdempotencyKey     string
}

type insertRecordRequest struct {
	Data               json.RawMessage `json:"data"`
	SchemaID           string          `json:"schemaId"`
	OrganizationID     string          `json:"organizationId"`
	OrganizationUserID string          `json:"organizationUserId"`
}
