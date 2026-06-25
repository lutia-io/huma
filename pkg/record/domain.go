package record

import (
	"encoding/json"
	"time"
)

type record struct {
	ID string `json:"id"`

	Data json.RawMessage `json:"data"`

	SchemaID       string `json:"schemaId"`
	NetworkID      string `json:"networkId"`
	OrganizationID string `json:"organizationId"`
	UserID         string `json:"userId"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type insertRecordRequest struct {
	Data json.RawMessage `json:"data"`

	SchemaID       string `json:"schemaId"`
	NetworkID      string `json:"networkId"`
	OrganizationID string `json:"organizationId"`
	UserID         string `json:"userId"`
}
