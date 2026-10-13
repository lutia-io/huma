package workflow

import (
	"encoding/json"
	"time"

	"github.com/lutia-io/huma/pkg/criteria"
	"github.com/lutia-io/huma/pkg/user"
)

type Definition struct {
	Trigger  Trigger           `json:"trigger"`
	Criteria criteria.Criteria `json:"criteria"`
	Nodes    [][]Node          `json:"nodes"`
}

const (
	TriggerOnCreated  = "created"
	TriggerOnUpdated  = "updated"
	TriggerOnSchedule = "schedule"
)

// Trigger decides when a definition is eligible for intake. Empty On means
// manual/API enqueue only (no record or schedule intake).
type Trigger struct {
	On       []string `json:"on,omitempty"`
	Changed  []string `json:"changed,omitempty"`
	Cron     string   `json:"cron,omitempty"`
	Timezone string   `json:"timezone,omitempty"`
}

// Events returns trigger events; empty when the workflow is enqueue-only.
func (t Trigger) Events() []string {
	if len(t.On) == 0 {
		return nil
	}
	return t.On
}

// HasAutoIntake reports whether the definition can be enqueued by the evaluator.
func (t Trigger) HasAutoIntake() bool {
	for _, on := range t.Events() {
		switch on {
		case TriggerOnCreated, TriggerOnUpdated, TriggerOnSchedule:
			return true
		}
	}
	return false
}

// Includes reports whether event is one of the trigger's events.
func (t Trigger) Includes(event string) bool {
	for _, on := range t.Events() {
		if on == event {
			return true
		}
	}
	return false
}

type WorkflowDefinition struct {
	ID string `json:"id"`

	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
	// Internal marks system-defined definitions that are inserted
	// automatically on network creation, as opposed to user-authored ones.
	Internal bool `json:"internal"`

	Definition Definition `json:"definition"`

	SchemaID       string   `json:"schemaId,omitempty"`
	NetworkID      string   `json:"networkId"`
	OrganizationID *string  `json:"organizationId,omitempty"`
	UserID         string   `json:"userId"`
	CreatedBy      user.Ref `json:"createdBy"`
	UpdatedBy      user.Ref `json:"updatedBy"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (w WorkflowDefinition) MatchesOrganization(organizationID string) bool {
	if w.OrganizationID == nil {
		return true
	}
	return *w.OrganizationID == organizationID
}

type insertWorkflowDefinitionRequest struct {
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Active         bool       `json:"active"`
	Internal       bool       `json:"internal"`
	Definition     Definition `json:"definition"`
	SchemaID       string     `json:"schemaId"`
	NetworkID      string     `json:"networkId"`
	OrganizationID string     `json:"organizationId"`
	UserID         string     `json:"-"`
}

type patchWorkflowDefinitionRequest struct {
	Name        *string     `json:"name"`
	Description *string     `json:"description"`
	Active      *bool       `json:"active"`
	Definition  *Definition `json:"definition"`
	SchemaID    *string     `json:"schemaId"`
}

type insertWorkflowRequest struct {
	WorkflowDefinitionID string         `json:"workflowDefinitionId"`
	Input                map[string]any `json:"input"`
	DedupeKey            string         `json:"dedupeKey,omitempty"`
}

type EnqueueRequest struct {
	WorkflowDefinitionID string
	WorkflowSlug         string
	NetworkID            string
	OrganizationID       string
	OrganizationUserID   string
	Input                map[string]any
	DedupeKey            string
}

type listParams struct {
	UserID         string
	Query          string
	NetworkID      string
	OrganizationID string
	Scope          string
	Active         *bool
	Name           string
	NameOp         string
	Slug           string
	SlugOp         string
	Schema         string
	SchemaOp       string
	SchemaID       string
	Field          string
	Network        string
	NetworkOp      string
	Stages         *int
	StagesOp       string
	// Deprecated query alias: actions/actionsOp map to stages.
	Internal       *bool
	Sort           string
	Order          string
	Page           int
	PageSize       int
}

type listResult struct {
	Items    []*WorkflowDefinition `json:"items"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
}

type runListParams struct {
	UserID         string
	Query          string
	NetworkID      string
	OrganizationID string
	Name           string
	NameOp         string
	Status         string
	Network        string
	NetworkOp      string
	Organization   string
	OrganizationOp string
	Sort           string
	Order          string
	Page           int
	PageSize       int
}

type runListResult struct {
	Items    []*Workflow `json:"items"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

// Workflow is one execution instance of a workflow definition.
type Workflow struct {
	ID                   string `json:"id"`
	WorkflowDefinitionID string `json:"workflowDefinitionId"`
	NetworkID            string `json:"networkId"`

	RecordID           string         `json:"recordId,omitempty"`
	Data               map[string]any `json:"data,omitempty"`
	Input              map[string]any `json:"input"`
	OrganizationID     string         `json:"organizationId"`
	OrganizationUserID string         `json:"organizationUserId"`
	DedupeKey          string         `json:"-"`

	Definition RunDefinition `json:"definition"`

	Status       string `json:"status"`
	CurrentLevel int    `json:"currentLevel"`
	Attempts     int    `json:"attempts"`
	MaxAttempts  int    `json:"maxAttempts"`
	Error        string `json:"error,omitempty"`

	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// UserID is the owning network's user, used for authorization only.
	UserID string `json:"-"`
}

// WorkflowNode is one journaled attempt of a single node in a workflow.
type WorkflowNode struct {
	ID          string          `json:"id"`
	WorkflowID  string          `json:"workflowId"`
	LevelIndex  int             `json:"levelIndex"`
	NodeIndex   int             `json:"nodeIndex"`
	Attempt     int             `json:"attempt"`
	NodeSlug    string          `json:"nodeSlug"`
	NodeType    string          `json:"nodeType"`
	Status      string          `json:"status"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Error       string          `json:"error,omitempty"`
	StartedAt   time.Time       `json:"startedAt"`
	CompletedAt time.Time       `json:"completedAt"`
}
