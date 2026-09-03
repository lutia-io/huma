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
	NetworkID          string `json:"networkId"`
	IdempotencyKey     string `json:"-"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`

	// UserID is the owning network's user, used for authorization only.
	UserID string `json:"-"`
}

// CreateParams is the input to Service.Create. HTTP always leaves
// IdempotencyKey empty; the workflow engine sets it to a deterministic action
// key so crash replays do not duplicate records.
type CreateParams struct {
	SchemaID           string
	OrganizationID     string
	OrganizationUserID string
	NetworkID          string
	Data               json.RawMessage
	IdempotencyKey     string
}

type insertRecordRequest struct {
	Data               json.RawMessage `json:"data"`
	SchemaID           string          `json:"schemaId"`
	OrganizationUserID string          `json:"organizationUserId"`
}

type patchRecordRequest struct {
	Data json.RawMessage `json:"data"`
}

type fieldFilter struct {
	Name         string
	Value        string
	Op           string
	Kind         string
	NumberValue  *float64
	BooleanValue *bool
	TitleKey     string
}

type schemaField struct {
	Name     string
	Kind     string
	SchemaID string
	TitleKey string
}

type listParams struct {
	UserID         string
	Query          string
	SchemaID       string
	NetworkID      string
	OrganizationID string
	Organization   string
	OrganizationOp string
	Fields         []fieldFilter
	SchemaFields   map[string]schemaField
	Sort           string
	Order          string
	Page           int
	PageSize       int
}

type RelatedRecord struct {
	ID       string `json:"id"`
	SchemaID string `json:"schemaId"`
	Title    string `json:"title"`
}

type listResult struct {
	Items    []*Record                `json:"items"`
	Related  map[string]RelatedRecord `json:"related"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
}
