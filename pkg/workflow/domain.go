package workflow

import (
	"encoding/json"
	"time"
)

type workflowDefinition struct {
	ID string `json:"id"`

	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Active   bool   `json:"active"`
	Internal bool   `json:"internal"`

	Definition json.RawMessage `json:"definition"`

	SchemaID  string `json:"schemaId"`
	NetworkID string `json:"networkId"`
	UserID    string `json:"userId"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type insertWorkflowDefinitionRequest struct {
	Name       string          `json:"name"`
	Active     bool            `json:"active"`
	Internal   bool            `json:"internal"`
	Definition json.RawMessage `json:"definition"`
	SchemaID   string          `json:"schemaId"`
	NetworkID  string          `json:"networkId"`
	UserID     string          `json:"userId"`
}
