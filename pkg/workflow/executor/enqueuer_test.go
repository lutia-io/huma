package executor

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/lutia-io/huma/pkg/criteria"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/record"
	"github.com/lutia-io/huma/pkg/workflow"
)

type fakeDefinitionStore struct {
	defs      []*workflow.WorkflowDefinition
	scheduled []*workflow.WorkflowDefinition
}

func (f *fakeDefinitionStore) ListActiveBySchemaID(context.Context, string) ([]*workflow.WorkflowDefinition, error) {
	return f.defs, nil
}

func (f *fakeDefinitionStore) ListActiveScheduled(context.Context) ([]*workflow.WorkflowDefinition, error) {
	return f.scheduled, nil
}

type fakeWorkflowStore struct {
	inserted []*Workflow
}

func (f *fakeWorkflowStore) InsertPending(_ context.Context, workflows []*Workflow) error {
	f.inserted = append(f.inserted, workflows...)
	return nil
}

func (f *fakeWorkflowStore) ClaimOne(context.Context, string) (*Workflow, error) {
	return nil, nil
}

func (f *fakeWorkflowStore) CompleteAction(context.Context, WorkflowAction) error {
	return nil
}

func (f *fakeWorkflowStore) FailAction(context.Context, WorkflowAction) error {
	return nil
}

func (f *fakeWorkflowStore) Finish(context.Context, string) (Status, error) {
	return StatusCompleted, nil
}

func (f *fakeWorkflowStore) FailExhausted(context.Context) (int64, error) {
	return 0, nil
}

func testEnqueuer(defs []*workflow.WorkflowDefinition) (*Enqueuer, *fakeWorkflowStore) {
	workflows := &fakeWorkflowStore{}
	return NewEnqueuer(logger.NewWithWriter(io.Discard), &fakeDefinitionStore{defs: defs}, workflows), workflows
}

func sampleDef(id string, trigger workflow.Trigger, status string) *workflow.WorkflowDefinition {
	def := &workflow.WorkflowDefinition{
		ID:        id,
		NetworkID: "net-1",
		Definition: workflow.Definition{
			Trigger: trigger,
			Actions: nil,
		},
	}
	if status != "" {
		def.Definition.Criteria = criteria.Criteria{Field: "status", Operator: criteria.OpEq, Value: status}
	}
	return def
}

func TestEvaluateCreated_defaultTrigger(t *testing.T) {
	def := sampleDef("def-1", workflow.Trigger{}, "pending")
	enqueuer, store := testEnqueuer([]*workflow.WorkflowDefinition{def})
	err := enqueuer.EvaluateCreated(context.Background(), record.CreatedEvent{
		ID:                 "rec-1",
		Data:               json.RawMessage(`{"status":"pending"}`),
		SchemaID:           "schema-1",
		OrganizationID:     "org-1",
		OrganizationUserID: "ou-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 1 || store.inserted[0].DedupeKey != "rec-1" {
		t.Fatalf("inserted = %+v", store.inserted)
	}
}

func TestEvaluateCreated_skipsUpdateOnly(t *testing.T) {
	def := sampleDef("def-1", workflow.Trigger{On: []string{workflow.TriggerOnUpdated}}, "pending")
	enqueuer, store := testEnqueuer([]*workflow.WorkflowDefinition{def})
	err := enqueuer.EvaluateCreated(context.Background(), record.CreatedEvent{
		ID:   "rec-1",
		Data: json.RawMessage(`{"status":"pending"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("inserted %d", len(store.inserted))
	}
}

func TestEvaluateUpdated_changedFields(t *testing.T) {
	def := sampleDef("def-1", workflow.Trigger{
		On:      []string{workflow.TriggerOnUpdated},
		Changed: []string{"status"},
	}, "shipped")
	enqueuer, store := testEnqueuer([]*workflow.WorkflowDefinition{def})
	event := record.UpdatedEvent{
		ID:                 "rec-1",
		Before:             json.RawMessage(`{"status":"pending","notes":"x"}`),
		After:              json.RawMessage(`{"status":"shipped","notes":"x"}`),
		EventID:            "evt-1",
		OrganizationID:     "org-1",
		OrganizationUserID: "ou-1",
	}
	if err := enqueuer.EvaluateUpdated(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 1 || store.inserted[0].DedupeKey != "rec-1:updated:evt-1" {
		t.Fatalf("inserted = %+v", store.inserted)
	}

	store.inserted = nil
	event.After = json.RawMessage(`{"status":"pending","notes":"y"}`)
	def.Definition.Criteria = criteria.Criteria{Field: "status", Operator: criteria.OpEq, Value: "pending"}
	if err := enqueuer.EvaluateUpdated(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 0 {
		t.Fatal("notes-only change with status filter should not enqueue")
	}
}

func TestEvaluateUpdated_emptyCriteria(t *testing.T) {
	def := sampleDef("def-1", workflow.Trigger{On: []string{workflow.TriggerOnUpdated}, Changed: []string{"status"}}, "")
	enqueuer, store := testEnqueuer([]*workflow.WorkflowDefinition{def})
	err := enqueuer.EvaluateUpdated(context.Background(), record.UpdatedEvent{
		ID:      "rec-1",
		Before:  json.RawMessage(`{"status":"pending"}`),
		After:   json.RawMessage(`{"status":"shipped"}`),
		EventID: "evt-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("inserted %d", len(store.inserted))
	}
}

func TestEvaluateSchedule_dedupeKey(t *testing.T) {
	def := sampleDef("def-1", workflow.Trigger{
		On:       []string{workflow.TriggerOnSchedule},
		Cron:     "0 9 * * *",
		Timezone: "UTC",
	}, "pending")
	enqueuer, store := testEnqueuer(nil)
	period := time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)
	err := enqueuer.EvaluateSchedule(context.Background(), def, []*record.Record{{
		ID:                 "rec-1",
		Data:               json.RawMessage(`{"status":"pending"}`),
		OrganizationID:     "org-1",
		OrganizationUserID: "ou-1",
	}}, period)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("inserted %d", len(store.inserted))
	}
	want := "rec-1:schedule:2026-09-08T09:00:00Z"
	if store.inserted[0].DedupeKey != want {
		t.Fatalf("dedupe = %s, want %s", store.inserted[0].DedupeKey, want)
	}
}
