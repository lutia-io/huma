package workflow

import (
	"context"
	"io"
	"testing"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/uuid"
)

type retryStore struct {
	workflow *Workflow
	live     *WorkflowDefinition
	retried  *RunDefinition
}

func (s *retryStore) Insert(context.Context, *WorkflowDefinition) (string, error) {
	panic("unused")
}
func (s *retryStore) Update(context.Context, *WorkflowDefinition) error { panic("unused") }
func (s *retryStore) List(context.Context, listParams) (*listResult, error) {
	panic("unused")
}
func (s *retryStore) ListActiveBySchemaID(context.Context, string) ([]*WorkflowDefinition, error) {
	panic("unused")
}
func (s *retryStore) ListActiveScheduled(context.Context) ([]*WorkflowDefinition, error) {
	panic("unused")
}
func (s *retryStore) ListWorkflows(context.Context, runListParams) (*runListResult, error) {
	panic("unused")
}
func (s *retryStore) GetWorkflowNodeByID(context.Context, string) (*WorkflowNode, error) {
	panic("unused")
}
func (s *retryStore) ListWorkflowNodesByWorkflowID(context.Context, string) ([]*WorkflowNode, error) {
	panic("unused")
}
func (s *retryStore) InsertPending(context.Context, *Workflow) (string, error) { panic("unused") }
func (s *retryStore) GetBySlug(context.Context, string, string, string) (*WorkflowDefinition, error) {
	panic("unused")
}

func (s *retryStore) GetByID(_ context.Context, id string) (*WorkflowDefinition, error) {
	if s.live == nil || s.live.ID != id {
		return nil, apperror.NewNotFoundError("Workflow definition not found", nil)
	}
	return s.live, nil
}

func (s *retryStore) GetWorkflowByID(_ context.Context, id string) (*Workflow, error) {
	if s.workflow == nil || s.workflow.ID != id {
		return nil, apperror.NewNotFoundError("Workflow not found", nil)
	}
	return s.workflow, nil
}

func (s *retryStore) RetryFailed(_ context.Context, id string, definition RunDefinition) error {
	if s.workflow == nil || s.workflow.ID != id {
		return apperror.NewNotFoundError("Workflow not found", nil)
	}
	s.retried = &definition
	return nil
}

func retryService(store *retryStore) *Service {
	return NewService(logger.NewWithWriter(io.Discard), store, nil)
}

func TestRetry_usesLiveDefinition(t *testing.T) {
	id := uuid.MustNew()
	defID := uuid.MustNew()
	snap := SnapshotRunDefinition(Definition{
		Trigger: Trigger{On: []string{TriggerOnCreated}},
		Nodes: [][]Node{{{
			Name: "a", Type: node.TypeNoop, Definition: node.NoopContext{Message: "old"},
		}}},
	})
	live := SnapshotRunDefinition(Definition{
		Trigger: Trigger{On: []string{TriggerOnCreated}},
		Nodes: [][]Node{{{
			Name: "a", Type: node.TypeNoop, Definition: node.NoopContext{Message: "new"},
		}}},
	})
	store := &retryStore{
		workflow: &Workflow{
			ID:                   id,
			WorkflowDefinitionID: defID,
			UserID:               "user-1",
			Status:               "failed",
			Definition:           snap,
		},
		live: &WorkflowDefinition{ID: defID, Definition: Definition{
			Trigger: Trigger{On: []string{TriggerOnCreated}},
			Nodes: [][]Node{{{
				Name: "a", Type: node.TypeNoop, Definition: node.NoopContext{Message: "new"},
			}}},
		}},
	}

	err := retryService(store).Retry(context.Background(), principal.Principal{
		Type: principal.TypeUser,
		ID:   "user-1",
	}, id)
	if err != nil {
		t.Fatal(err)
	}
	if store.retried == nil {
		t.Fatal("RetryFailed was not called")
	}
	msg := store.retried.Nodes[0][0].Definition.(node.NoopContext).Message
	if msg != "new" {
		t.Fatalf("retry used snapshot, want live definition: %q", msg)
	}
	_ = live
}

func TestRetry_keepsSnapshotWhenDefinitionMissing(t *testing.T) {
	id := uuid.MustNew()
	snap := SnapshotRunDefinition(Definition{Trigger: Trigger{On: []string{TriggerOnCreated}}, Nodes: validNodes()})
	store := &retryStore{
		workflow: &Workflow{
			ID:                   id,
			WorkflowDefinitionID: uuid.MustNew(),
			UserID:               "user-1",
			Status:               "failed",
			Definition:           snap,
		},
	}

	err := retryService(store).Retry(context.Background(), principal.Principal{
		Type: principal.TypeUser,
		ID:   "user-1",
	}, id)
	if err != nil {
		t.Fatal(err)
	}
	if store.retried == nil || len(store.retried.Nodes) != 1 {
		t.Fatalf("retried = %+v", store.retried)
	}
}
