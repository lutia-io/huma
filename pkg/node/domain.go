package node

import (
	"encoding/json"
	"time"
)

type NodeDefinition struct {
	ID string `json:"id"`

	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Active   bool   `json:"active"`
	Internal bool   `json:"internal"`
	Type     Type   `json:"type"`

	Definition any `json:"definition"`

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
	UserID     string          `json:"-"`
}

type patchNodeDefinitionRequest struct {
	Name       *string          `json:"name"`
	Active     *bool            `json:"active"`
	Type       *Type            `json:"type"`
	Definition *json.RawMessage `json:"definition"`
}

type listParams struct {
	UserID    string
	Query     string
	NetworkID string
	Active    *bool
	Name      string
	NameOp    string
	Slug      string
	SlugOp    string
	Type      string
	TypeOp    string
	Network   string
	NetworkOp string
	Sort      string
	Order     string
	Page      int
	PageSize  int
}

type listResult struct {
	Items    []*NodeDefinition `json:"items"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}
