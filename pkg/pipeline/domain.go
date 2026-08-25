package pipeline

import (
	"encoding/json"
	"time"

	"github.com/lutia-io/huma/pkg/node"
)

type pipelineDefinition struct {
	ID string `json:"id"`

	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Active   bool   `json:"active"`
	Internal bool   `json:"internal"`

	Definition definition `json:"definition"`

	NetworkID string `json:"networkId"`
	UserID    string `json:"userId"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type insertPipelineDefinitionRequest struct {
	Name       string     `json:"name"`
	Active     bool       `json:"active"`
	Internal   bool       `json:"internal"`
	Definition definition `json:"definition"`
	NetworkID  string     `json:"networkId"`
	UserID     string     `json:"-"`
}

type patchPipelineDefinitionRequest struct {
	Name       *string     `json:"name"`
	Active     *bool       `json:"active"`
	Definition *definition `json:"definition"`
}

type insertPipelineRequest struct {
	PipelineDefinitionID string         `json:"pipelineDefinitionId"`
	Input                map[string]any `json:"input"`
	DedupeKey            string         `json:"dedupeKey,omitempty"`
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
	Network   string
	NetworkOp string
	Source    string
	SourceOp  string
	Stages    *int
	StagesOp  string
	Sort      string
	Order     string
	Page      int
	PageSize  int
}

type listResult struct {
	Items    []*pipelineDefinition `json:"items"`
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
	Items    []*Pipeline `json:"items"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

// SnapshotNode is a node definition frozen onto a pipeline run.
type SnapshotNode struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	Type       node.Type `json:"type"`
	Definition any       `json:"definition"`
}

type SnapshotDefinition struct {
	Nodes [][]SnapshotNode `json:"nodes"`
}

// Pipeline is one execution instance of a pipeline definition.
type Pipeline struct {
	ID                   string `json:"id"`
	PipelineDefinitionID string `json:"pipelineDefinitionId"`
	NetworkID            string `json:"networkId"`

	Input              map[string]any `json:"input"`
	OrganizationID     string         `json:"organizationId"`
	OrganizationUserID string         `json:"organizationUserId"`

	DedupeKey string `json:"-"`

	Definition SnapshotDefinition `json:"definition"`

	Status       string `json:"status"`
	CurrentLevel int    `json:"currentLevel"`
	Attempts     int    `json:"attempts"`
	MaxAttempts  int    `json:"maxAttempts"`
	Error        string `json:"error,omitempty"`

	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	UserID string `json:"-"`
}

// PipelineNode is one journaled attempt of a single node in a pipeline level.
type PipelineNode struct {
	ID               string          `json:"id"`
	PipelineID       string          `json:"pipelineId"`
	LevelIndex       int             `json:"levelIndex"`
	NodeIndex        int             `json:"nodeIndex"`
	Attempt          int             `json:"attempt"`
	NodeDefinitionID string          `json:"nodeDefinitionId"`
	NodeSlug         string          `json:"nodeSlug"`
	NodeType         string          `json:"nodeType"`
	Status           string          `json:"status"`
	Input            json.RawMessage `json:"input,omitempty"`
	Output           json.RawMessage `json:"output,omitempty"`
	Error            string          `json:"error,omitempty"`
	StartedAt        time.Time       `json:"startedAt"`
	CompletedAt      time.Time       `json:"completedAt"`
}

type EnqueueRequest struct {
	PipelineDefinitionID string
	PipelineSlug         string
	NetworkID            string
	OrganizationID       string
	OrganizationUserID   string
	Input                map[string]any
	DedupeKey            string
}
