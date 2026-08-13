package workflow

import (
	"time"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/criteria"
)

type Definition struct {
	Criteria criteria.Criteria `json:"criteria"`
	Actions  []action.Action   `json:"actions"`
}

type WorkflowDefinition struct {
	ID string `json:"id"`

	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Active bool   `json:"active"`
	// Internal marks system-defined definitions that are inserted
	// automatically on network creation, as opposed to user-authored ones.
	Internal bool `json:"internal"`

	Definition Definition `json:"definition"`

	SchemaID  string `json:"schemaId"`
	NetworkID string `json:"networkId"`
	UserID    string `json:"userId"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type insertWorkflowDefinitionRequest struct {
	Name       string     `json:"name"`
	Active     bool       `json:"active"`
	Internal   bool       `json:"internal"`
	Definition Definition `json:"definition"`
	SchemaID   string     `json:"schemaId"`
	NetworkID  string     `json:"networkId"`
	UserID     string     `json:"-"`
}
