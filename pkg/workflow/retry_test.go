package workflow

import (
	"context"
	"io"
	"testing"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/uuid"
)

type retryStore struct {
	workflow *Workflow
	live     *WorkflowDefinition
	retried  *Definition
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
func (s *retryStore) SchemaVisibleToOrganization(context.Context, string, string, string) (bool, error) {
	panic("unused")
}
func (s *retryStore) ListWorkflows(context.Context, runListParams) (*runListResult, error) {
	panic("unused")
}
func (s *retryStore) GetWorkflowActionByID(context.Context, string) (*WorkflowAction, error) {
	panic("unused")
}
func (s *retryStore) ListWorkflowActionsByWorkflowID(context.Context, string) ([]*WorkflowAction, error) {
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

func (s *retryStore) RetryFailed(_ context.Context, id string, definition Definition) error {
	if s.workflow == nil || s.workflow.ID != id {
		return apperror.NewNotFoundError("Workflow not found", nil)
	}
	s.retried = &definition
	return nil
}

func retryService(store *retryStore) *Service {
	return NewService(logger.NewWithWriter(io.Discard), store)
}

func TestRetry_usesLiveDefinition(t *testing.T) {
	id := uuid.MustNew()
	defID := uuid.MustNew()
	store := &retryStore{
		workflow: &Workflow{
			ID:                   id,
			WorkflowDefinitionID: defID,
			UserID:               "user-1",
			Status:               "failed",
			Definition: Definition{Actions: []action.Action{{
				Type: action.TypeUpdateRecord,
				Context: action.UpdateRecordContext{
					RecordID: "rec-1",
					Data:     map[string]any{"contributedCents": 1},
				},
			}}},
		},
		live: &WorkflowDefinition{
			ID: defID,
			Definition: Definition{Actions: []action.Action{{
				Type: action.TypeUpdateRecord,
				Context: action.UpdateRecordContext{
					RecordID: "rec-1",
					Data:     map[string]any{"contributedAmountCents": 1},
				},
			}}},
		},
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
	ctx := store.retried.Actions[0].Context.(action.UpdateRecordContext)
	if _, ok := ctx.Data["contributedAmountCents"]; !ok {
		t.Fatalf("retry used snapshot, want live definition: %+v", ctx.Data)
	}
}

func TestRetry_keepsSnapshotWhenDefinitionMissing(t *testing.T) {
	id := uuid.MustNew()
	store := &retryStore{
		workflow: &Workflow{
			ID:                   id,
			WorkflowDefinitionID: uuid.MustNew(),
			UserID:               "user-1",
			Status:               "failed",
			Definition:           Definition{Actions: validActions()},
		},
	}

	err := retryService(store).Retry(context.Background(), principal.Principal{
		Type: principal.TypeUser,
		ID:   "user-1",
	}, id)
	if err != nil {
		t.Fatal(err)
	}
	if store.retried == nil || len(store.retried.Actions) != 1 {
		t.Fatalf("retried = %+v", store.retried)
	}
}

func TestRetry_rejectsNonFailed(t *testing.T) {
	id := uuid.MustNew()
	store := &retryStore{
		workflow: &Workflow{
			ID:     id,
			UserID: "user-1",
			Status: "completed",
		},
	}

	err := retryService(store).Retry(context.Background(), principal.Principal{
		Type: principal.TypeUser,
		ID:   "user-1",
	}, id)
	if !apperror.IsBadRequest(err) {
		t.Fatalf("got %v", err)
	}
	if store.retried != nil {
		t.Fatal("should not retry a completed workflow")
	}
}

func TestRetry_notFound(t *testing.T) {
	err := retryService(&retryStore{}).Retry(context.Background(), principal.Principal{
		Type: principal.TypeUser,
		ID:   "user-1",
	}, uuid.MustNew())
	if !apperror.IsNotFound(err) {
		t.Fatalf("got %v", err)
	}
}
