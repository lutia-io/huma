package executor

import (
	"time"

	"github.com/lutia-io/huma/pkg/workflow"
)

// Status is the lifecycle state of a workflow row in public.workflows.
type Status string

const (
	// StatusPending means the workflow is waiting to be claimed by a worker.
	StatusPending Status = "pending"
	// StatusRunning means a worker currently holds the lease (or held it and
	// crashed — expired leases are reclaimable while still "running").
	StatusRunning Status = "running"
	// StatusCompleted means every action either succeeded or was skipped past
	// after a journaled failure, and none remain incomplete.
	StatusCompleted Status = "completed"
	// StatusFailed means at least one action never completed successfully, or
	// the workflow exhausted its claim attempts.
	StatusFailed Status = "failed"
)

// ActionStatus is the terminal state of a single action attempt in the
// workflow_actions journal. Attempts are append-only; a later completed
// attempt of the same action_index supersedes an earlier failed one.
type ActionStatus string

const (
	ActionStatusCompleted ActionStatus = "completed"
	ActionStatusFailed    ActionStatus = "failed"
)

// Workflow is one execution instance of a workflow definition, triggered by a
// single record event. The definition and record data are snapshots taken at
// enqueue time: retries always execute the actions the workflow started with,
// templated against the data that triggered it.
type Workflow struct {
	ID                   string
	WorkflowDefinitionID string
	NetworkID            string

	// RecordID identifies the trigger record; Data is its content at trigger
	// time, and the organization identity flows from it onto records created
	// by actions.
	RecordID           string
	Data               map[string]any
	OrganizationID     string
	OrganizationUserID string

	// DedupeKey makes intake idempotent under event redelivery. Today it is
	// the trigger record ID; unique with WorkflowDefinitionID.
	DedupeKey string

	// Definition is the frozen criteria+actions list for this execution.
	Definition workflow.Definition

	Status Status
	// CurrentAction is the resume cursor into Definition.Actions (0-based).
	// Workers start from this index after a reclaim.
	CurrentAction int
	// Attempts counts how many times this workflow has been claimed. Claims
	// stop once Attempts reaches MaxAttempts.
	Attempts    int
	MaxAttempts int

	CreatedAt   time.Time
	CompletedAt *time.Time
}

// WorkflowAction is one journal entry: a single attempt of a single action.
// Rows are never updated; failed attempts stay for audit, and a successful
// retry of the same ActionIndex is a new row.
type WorkflowAction struct {
	ID          string
	WorkflowID  string
	ActionIndex int
	Attempt     int
	ActionType  string
	Status      ActionStatus
	// Input is the action as executed (JSON); Output is the handler result.
	Input       []byte
	Output      []byte
	Error       string
	StartedAt   time.Time
	CompletedAt time.Time
}
