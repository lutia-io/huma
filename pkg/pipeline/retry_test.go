package pipeline

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
	pipeline *Pipeline
	live     *pipelineDefinition
	retried  *SnapshotDefinition
}

func (s *retryStore) Insert(context.Context, *pipelineDefinition) (string, error) {
	panic("unused")
}
func (s *retryStore) Update(context.Context, *pipelineDefinition) error { panic("unused") }
func (s *retryStore) GetBySlug(context.Context, string, string, string) (*pipelineDefinition, error) {
	panic("unused")
}
func (s *retryStore) List(context.Context, listParams) (*listResult, error) {
	panic("unused")
}
func (s *retryStore) InsertPending(context.Context, *Pipeline) (string, error) {
	panic("unused")
}
func (s *retryStore) ListPipelines(context.Context, runListParams) (*runListResult, error) {
	panic("unused")
}
func (s *retryStore) GetPipelineNodeByID(context.Context, string) (*PipelineNode, error) {
	panic("unused")
}
func (s *retryStore) ListPipelineNodesByPipelineID(context.Context, string) ([]*PipelineNode, error) {
	panic("unused")
}

func (s *retryStore) GetByID(_ context.Context, id string) (*pipelineDefinition, error) {
	if s.live == nil || s.live.ID != id {
		return nil, apperror.NewNotFoundError("Pipeline definition not found", nil)
	}
	return s.live, nil
}

func (s *retryStore) GetPipelineByID(_ context.Context, id string) (*Pipeline, error) {
	if s.pipeline == nil || s.pipeline.ID != id {
		return nil, apperror.NewNotFoundError("Pipeline not found", nil)
	}
	return s.pipeline, nil
}

func (s *retryStore) RetryFailed(_ context.Context, id string, definition SnapshotDefinition) error {
	if s.pipeline == nil || s.pipeline.ID != id {
		return apperror.NewNotFoundError("Pipeline not found", nil)
	}
	s.retried = &definition
	return nil
}

func retryService(store *retryStore) *Service {
	return NewService(logger.NewWithWriter(io.Discard), store)
}

func mapperNode(name, field string) Node {
	return Node{
		Name: name,
		Type: node.TypeMapper,
		Definition: node.MapperContext{
			Mapping: map[string]any{field: 1},
		},
	}
}

func TestRetry_usesLiveDefinition(t *testing.T) {
	id := uuid.MustNew()
	defID := uuid.MustNew()
	store := &retryStore{
		pipeline: &Pipeline{
			ID:                   id,
			PipelineDefinitionID: defID,
			UserID:               "user-1",
			Status:               "failed",
			Definition: snapshotDefinition(definition{
				Nodes: [][]Node{{mapperNode("shape", "oldField")}},
			}),
		},
		live: &pipelineDefinition{
			ID: defID,
			Definition: definition{
				Nodes: [][]Node{{mapperNode("shape", "newField")}},
			},
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
	ctx := store.retried.Nodes[0][0].Definition.(node.MapperContext)
	if _, ok := ctx.Mapping["newField"]; !ok {
		t.Fatalf("retry used snapshot, want live definition: %+v", ctx.Mapping)
	}
}

func TestRetry_keepsSnapshotWhenDefinitionMissing(t *testing.T) {
	id := uuid.MustNew()
	store := &retryStore{
		pipeline: &Pipeline{
			ID:                   id,
			PipelineDefinitionID: uuid.MustNew(),
			UserID:               "user-1",
			Status:               "failed",
			Definition: snapshotDefinition(definition{
				Nodes: [][]Node{{mapperNode("shape", "oldField")}},
			}),
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
	ctx := store.retried.Nodes[0][0].Definition.(node.MapperContext)
	if _, ok := ctx.Mapping["oldField"]; !ok {
		t.Fatalf("retry dropped snapshot: %+v", ctx.Mapping)
	}
}

func TestRetry_rejectsNonFailed(t *testing.T) {
	id := uuid.MustNew()
	store := &retryStore{
		pipeline: &Pipeline{
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
		t.Fatal("should not retry a completed pipeline")
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
