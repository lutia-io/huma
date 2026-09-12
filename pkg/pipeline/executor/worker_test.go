package executor

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
)

type recordingStore struct {
	mu        sync.Mutex
	terminals []TerminalNode
	executed  []int
	failed    *bool
}

func (s *recordingStore) ClaimOne(context.Context, string) (*Pipeline, error) {
	return nil, nil
}
func (s *recordingStore) FailExhausted(context.Context) (int64, error) { return 0, nil }
func (s *recordingStore) ListTerminalNodes(context.Context, string, int) ([]TerminalNode, error) {
	return s.terminals, nil
}
func (s *recordingStore) AdvanceLevel(context.Context, string, int) error { return nil }
func (s *recordingStore) CompleteNode(_ context.Context, entry PipelineNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executed = append(s.executed, entry.NodeIndex)
	return nil
}
func (s *recordingStore) FailNode(_ context.Context, entry PipelineNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executed = append(s.executed, entry.NodeIndex)
	return nil
}
func (s *recordingStore) Finish(_ context.Context, _ string, failed bool, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = &failed
	return nil
}

func (s *recordingStore) executedSet() map[int]struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int]struct{}, len(s.executed))
	for _, i := range s.executed {
		out[i] = struct{}{}
	}
	return out
}

type recordingHandler struct{}

func (h *recordingHandler) Type() node.Type { return node.TypeNoop }
func (h *recordingHandler) Execute(context.Context, ExecutionContext, pipeline.SnapshotNode) (Result, error) {
	return MarshalOutput(map[string]any{"ok": true})
}

func testWorker(store PipelineStore) *worker {
	return &worker{
		id: "test",
		service: NewService(
			logger.NewWithWriter(io.Discard),
			store,
			NewRegistry(&recordingHandler{}),
		),
	}
}

func twoNoopNodes() pipeline.SnapshotDefinition {
	n := pipeline.SnapshotNode{
		Name:       "noop",
		Slug:       "noop",
		Type:       node.TypeNoop,
		Definition: node.NoopContext{Message: "ok"},
	}
	return pipeline.SnapshotDefinition{Nodes: [][]pipeline.SnapshotNode{{n, n}}}
}

func TestExecute_skipsCompletedNodes(t *testing.T) {
	store := &recordingStore{terminals: []TerminalNode{{
		NodeIndex: 0,
		Status:    NodeStatusCompleted,
		Output:    []byte(`{"ok":true}`),
	}}}
	w := testWorker(store)
	w.execute(context.Background(), &Pipeline{
		ID:         "p-1",
		Definition: twoNoopNodes(),
	})
	got := store.executedSet()
	if _, ok := got[0]; ok {
		t.Fatalf("executed = %v, did not want completed node 0", got)
	}
	if _, ok := got[1]; !ok {
		t.Fatalf("executed = %v, want node 1", got)
	}
}

func TestExecute_rerunsFailedNodes(t *testing.T) {
	store := &recordingStore{terminals: []TerminalNode{{
		NodeIndex: 0,
		Status:    NodeStatusFailed,
	}}}
	w := testWorker(store)
	w.execute(context.Background(), &Pipeline{
		ID:         "p-1",
		Definition: twoNoopNodes(),
	})
	got := store.executedSet()
	if _, ok := got[0]; !ok {
		t.Fatalf("executed = %v, want failed node 0 rerun", got)
	}
	if _, ok := got[1]; !ok {
		t.Fatalf("executed = %v, want node 1", got)
	}
	if store.failed == nil || *store.failed {
		t.Fatalf("finished failed=%v, want completed", store.failed)
	}
}

func TestExecute_runsAllWhenNoneCompleted(t *testing.T) {
	store := &recordingStore{}
	w := testWorker(store)
	w.execute(context.Background(), &Pipeline{
		ID:         "p-1",
		Definition: twoNoopNodes(),
	})
	got := store.executedSet()
	if len(got) != 2 {
		t.Fatalf("executed = %v, want [0 1]", got)
	}
}
