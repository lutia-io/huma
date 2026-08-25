package executor

import (
	"context"
	"testing"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
)

func TestRegistryUnknownType(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), ExecutionContext{}, pipeline.SnapshotNode{Type: node.TypeMapper})
	if err == nil {
		t.Fatal("expected error for unimplemented type")
	}
}
