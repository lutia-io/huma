package workflow

import (
	"testing"

	"github.com/lutia-io/huma/pkg/node"
)

func validNodes() [][]Node {
	return [][]Node{{
		{
			Type:       node.TypeNoop,
			Definition: node.NoopContext{Message: "ok"},
		},
	}}
}

func TestValidateDefinition(t *testing.T) {
	def := Definition{
		Trigger: Trigger{On: []string{TriggerOnCreated}},
		Nodes:   validNodes(),
	}
	if err := validateDefinition(def); err != nil {
		t.Fatalf("validateDefinition() error = %v", err)
	}
}

func TestValidateDefinition_requiresLevels(t *testing.T) {
	def := Definition{Trigger: Trigger{On: []string{TriggerOnCreated}}}
	if err := validateDefinition(def); err == nil {
		t.Fatal("expected error for empty nodes")
	}
}
