package executor

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/workflow"
)

type recordingStore struct {
	completed map[int]struct{}
	executed  []int
}

func (s *recordingStore) InsertPending(context.Context, []*Workflow) error { return nil }
func (s *recordingStore) ClaimOne(context.Context, string) (*Workflow, error) {
	return nil, nil
}
func (s *recordingStore) FailExhausted(context.Context) (int64, error) { return 0, nil }
func (s *recordingStore) CompletedActionIndexes(context.Context, string) (map[int]struct{}, error) {
	return s.completed, nil
}
func (s *recordingStore) CompleteAction(_ context.Context, entry WorkflowAction) error {
	s.executed = append(s.executed, entry.ActionIndex)
	return nil
}
func (s *recordingStore) FailAction(_ context.Context, entry WorkflowAction) error {
	s.executed = append(s.executed, entry.ActionIndex)
	return nil
}
func (s *recordingStore) Finish(context.Context, string) (Status, error) {
	return StatusCompleted, nil
}

type recordingHandler struct {
	typ action.Type
}

func (h *recordingHandler) Type() action.Type { return h.typ }
func (h *recordingHandler) Execute(context.Context, ExecutionContext, action.Action) (json.RawMessage, error) {
	return []byte(`{}`), nil
}

func testWorker(store WorkflowStore) *worker {
	return &worker{
		id: "test",
		service: NewService(
			logger.NewWithWriter(io.Discard),
			store,
			NewRegistry(&recordingHandler{typ: action.TypeUpdateRecord}),
		),
	}
}

func twoUpdateActions() []action.Action {
	act := action.Action{
		Type:    action.TypeUpdateRecord,
		Context: action.UpdateRecordContext{RecordID: "rec-1", Data: map[string]any{}},
	}
	return []action.Action{act, act}
}

func TestExecute_skipsCompletedActionIndexes(t *testing.T) {
	store := &recordingStore{completed: map[int]struct{}{0: {}}}
	w := testWorker(store)
	w.execute(context.Background(), &Workflow{
		ID:         "wf-1",
		Definition: workflow.Definition{Actions: twoUpdateActions()},
	})
	if len(store.executed) != 1 || store.executed[0] != 1 {
		t.Fatalf("executed = %v, want [1]", store.executed)
	}
}

func TestExecute_runsAllWhenNoneCompleted(t *testing.T) {
	store := &recordingStore{completed: map[int]struct{}{}}
	w := testWorker(store)
	w.execute(context.Background(), &Workflow{
		ID:         "wf-1",
		Definition: workflow.Definition{Actions: twoUpdateActions()},
	})
	if len(store.executed) != 2 || store.executed[0] != 0 || store.executed[1] != 1 {
		t.Fatalf("executed = %v, want [0 1]", store.executed)
	}
}
