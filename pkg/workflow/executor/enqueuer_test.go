package executor

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/lutia-io/huma/pkg/criteria"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/node"
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

func (f *fakeWorkflowStore) CompleteNode(context.Context, WorkflowNode) error { return nil }
func (f *fakeWorkflowStore) FailNode(context.Context, WorkflowNode) error     { return nil }
func (f *fakeWorkflowStore) ListTerminalNodes(context.Context, string, int) ([]TerminalNode, error) {
	return nil, nil
}
func (f *fakeWorkflowStore) AdvanceLevel(context.Context, string, int) error { return nil }
func (f *fakeWorkflowStore) Finish(context.Context, string, bool, string) error {
	return nil
}
func (f *fakeWorkflowStore) FailExhausted(context.Context) (int64, error) { return 0, nil }

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
			Nodes: [][]workflow.Node{{{
				Type: node.TypeNoop, Definition: node.NoopContext{},
			}}},
		},
	}
	if status != "" {
		def.Definition.Criteria = criteria.Criteria{Field: "status", Operator: criteria.OpEq, Value: status}
	}
	return def
}

func TestEvaluateCreated_matchesCreatedTrigger(t *testing.T) {
	def := sampleDef("def-1", workflow.Trigger{On: []string{workflow.TriggerOnCreated}}, "pending")
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
	if len(store.inserted) != 1 {
		t.Fatalf("inserted %d workflows, want 1", len(store.inserted))
	}
	if store.inserted[0].Input["recordId"] != "rec-1" {
		t.Fatalf("input = %+v", store.inserted[0].Input)
	}
}

func TestEvaluateCreated_skipsManualOnly(t *testing.T) {
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
	if len(store.inserted) != 0 {
		t.Fatalf("inserted %d workflows, want 0", len(store.inserted))
	}
}

func TestEvaluateSchedule(t *testing.T) {
	def := sampleDef("def-1", workflow.Trigger{On: []string{workflow.TriggerOnSchedule}, Cron: "* * * * *", Timezone: "UTC"}, "")
	workflows := &fakeWorkflowStore{}
	enqueuer := NewEnqueuer(logger.NewWithWriter(io.Discard), &fakeDefinitionStore{scheduled: []*workflow.WorkflowDefinition{def}}, workflows)
	store := workflows
	period := time.Now().UTC()
	err := enqueuer.EvaluateSchedule(context.Background(), def, []*record.Record{{
		ID:                 "rec-1",
		Data:               json.RawMessage(`{}`),
		OrganizationID:     "org-1",
		OrganizationUserID: "ou-1",
	}}, period)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("inserted %d workflows, want 1", len(store.inserted))
	}
}
