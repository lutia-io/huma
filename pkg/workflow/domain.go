package workflow

import (
	"encoding/json"
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

// Workflow is one execution instance of a workflow definition.
type Workflow struct {
	ID                   string `json:"id"`
	WorkflowDefinitionID string `json:"workflowDefinitionId"`
	NetworkID            string `json:"networkId"`

	RecordID           string         `json:"recordId"`
	Data               map[string]any `json:"data"`
	OrganizationID     string         `json:"organizationId"`
	OrganizationUserID string         `json:"organizationUserId"`

	Definition Definition `json:"definition"`

	Status        string `json:"status"`
	CurrentAction int    `json:"currentAction"`
	Attempts      int    `json:"attempts"`
	MaxAttempts   int    `json:"maxAttempts"`
	Error         string `json:"error,omitempty"`

	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// UserID is the owning network's user, used for authorization only.
	UserID string `json:"-"`
}

// WorkflowAction is one journaled attempt of a single action in a workflow.
type WorkflowAction struct {
	ID          string          `json:"id"`
	WorkflowID  string          `json:"workflowId"`
	ActionIndex int             `json:"actionIndex"`
	Attempt     int             `json:"attempt"`
	ActionType  string          `json:"actionType"`
	Status      string          `json:"status"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	StartedAt   time.Time       `json:"startedAt"`
	CompletedAt time.Time       `json:"completedAt"`
}
