package executor

import (
	"time"

	"github.com/lutia-io/huma/pkg/pipeline"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type NodeStatus string

const (
	NodeStatusCompleted NodeStatus = "completed"
	NodeStatusFailed    NodeStatus = "failed"
)

type Pipeline struct {
	ID                   string
	PipelineDefinitionID string
	NetworkID            string
	Input                map[string]any
	OrganizationID       string
	OrganizationUserID   string
	DedupeKey            string
	Definition           pipeline.SnapshotDefinition
	Status               Status
	CurrentLevel         int
	Attempts             int
	MaxAttempts          int
	CreatedAt            time.Time
	CompletedAt          *time.Time
}

type PipelineNode struct {
	ID               string
	PipelineID       string
	LevelIndex       int
	NodeIndex        int
	Attempt          int
	NodeDefinitionID string
	NodeSlug         string
	NodeType         string
	Status           NodeStatus
	Input            []byte
	Output           []byte
	Error            string
	StartedAt        time.Time
	CompletedAt      time.Time
}

type TerminalNode struct {
	NodeIndex int
	Status    NodeStatus
	Output    []byte
}
