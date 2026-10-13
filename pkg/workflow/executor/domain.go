package executor

import (
	"time"

	"github.com/lutia-io/huma/pkg/workflow"
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

type Workflow struct {
	ID                   string
	WorkflowDefinitionID string
	NetworkID            string
	RecordID             string
	Data                 map[string]any
	Input                map[string]any
	OrganizationID       string
	OrganizationUserID   string
	DedupeKey            string
	Definition           workflow.RunDefinition
	Status               Status
	CurrentLevel         int
	Attempts             int
	MaxAttempts          int
	CreatedAt            time.Time
	CompletedAt          *time.Time
}

type WorkflowNode struct {
	ID          string
	WorkflowID  string
	LevelIndex  int
	NodeIndex   int
	Attempt     int
	NodeType    string
	Status      NodeStatus
	Input       []byte
	Output      []byte
	Payload     []byte
	Error       string
	StartedAt   time.Time
	CompletedAt time.Time
}

type TerminalNode struct {
	NodeIndex int
	Status    NodeStatus
	Output    []byte
}
