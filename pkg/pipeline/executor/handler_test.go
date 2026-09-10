package executor

import (
	"context"
	"testing"

	"github.com/lutia-io/huma/pkg/pipeline"
)

func TestRegistryUnknownType(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), ExecutionContext{}, pipeline.SnapshotNode{Type: "SQL"})
	if err == nil {
		t.Fatal("expected error for unimplemented type")
	}
}
