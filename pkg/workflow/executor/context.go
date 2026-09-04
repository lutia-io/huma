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
	// TriggerData is the snapshotted JSONB document of the trigger record.
	// Handlers pass it to resolver.Resolve as Trigger.Data, so templates
	// address fields as {{ .Record.data.<field> }} and the row UUID as
	// {{ .Record.id }}. UPDATE_RECORD also loads the target row and exposes
	// it as {{ .Context.data.<field> }} / {{ .Context.id }}.
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
