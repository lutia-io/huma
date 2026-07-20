package node

import (
	"encoding/json"
	"time"
)

type nodeDefinition struct {
	ID string `json:"id"`

	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Active   bool   `json:"active"`
	Internal bool   `json:"internal"`
	Type     Type   `json:"type"`

	Definition json.RawMessage `json:"definition"`

	NetworkID string `json:"networkId"`
	UserID    string `json:"userId"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type insertNodeDefinitionRequest struct {
	Name       string          `json:"name"`
	Active     bool            `json:"active"`
	Internal   bool            `json:"internal"`
	Type       Type            `json:"type"`
	Definition json.RawMessage `json:"definition"`
	NetworkID  string          `json:"networkId"`
	UserID     string          `json:"userId"`
}
