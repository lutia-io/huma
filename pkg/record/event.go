package record

import "encoding/json"

const (
	StreamName    = "RECORDS"
	StreamSubject = "records.>"

	SubjectCreated = "records.created"
)

type CreatedEvent struct {
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data"`
}
