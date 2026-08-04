package executor

// ExecutionContext carries everything an action handler needs about the
// workflow it is executing within. Handlers receive it by value and must not
// retain it.
type ExecutionContext struct {
	// WorkflowID is the workflow instance ID (public.workflows.id).
	WorkflowID           string
	WorkflowDefinitionID string
	NetworkID            string

	// Trigger record context. Organization identity comes from the record
	// that triggered the workflow, so records created by actions belong to
	// the same organization as the trigger.
	TriggerRecordID string
	// TriggerData is the snapshotted record document used for template
	// resolution (resolver.Resolve) inside handlers.
	TriggerData        map[string]any
	OrganizationID     string
	OrganizationUserID string

	// ActionIndex is the position of the current action in the definition.
	ActionIndex int

	// IdempotencyKey is deterministic across retries of the same action
	// (workflowID:actionIndex). Handlers must pass it into their side
	// effects so replayed attempts do not duplicate work.
	IdempotencyKey string
}
