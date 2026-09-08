package record

import "encoding/json"

const (
	StreamName    = "RECORDS"
	StreamSubject = "records.>"

	SubjectCreated = "records.created"
	SubjectUpdated = "records.updated"
)

type CreatedEvent struct {
	ID                 string          `json:"id"`
	Data               json.RawMessage `json:"data"`
	SchemaID           string          `json:"schema_id"`
	OrganizationID     string          `json:"organization_id"`
	OrganizationUserID string          `json:"organization_user_id"`
	NetworkID          string          `json:"network_id"`
}

type UpdatedEvent struct {
	ID                 string          `json:"id"`
	Before             json.RawMessage `json:"before"`
	After              json.RawMessage `json:"after"`
	EventID            string          `json:"event_id"`
	SchemaID           string          `json:"schema_id"`
	OrganizationID     string          `json:"organization_id"`
	OrganizationUserID string          `json:"organization_user_id"`
	NetworkID          string          `json:"network_id"`
}
