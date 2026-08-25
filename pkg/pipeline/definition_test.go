package pipeline

import (
	"testing"

	"github.com/lutia-io/huma/pkg/uuid"
)

func TestValidateDefinitionShape_ok(t *testing.T) {
	err := validateDefinitionShape(definition{
		Nodes: [][]nodeRef{
			{{ID: uuid.MustNew()}, {ID: uuid.MustNew()}},
			{{ID: uuid.MustNew()}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateDefinitionShape_empty(t *testing.T) {
	if err := validateDefinitionShape(definition{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateDefinitionShape_emptyLevel(t *testing.T) {
	err := validateDefinitionShape(definition{
		Nodes: [][]nodeRef{{}, {{ID: uuid.MustNew()}}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateDefinitionShape_invalidID(t *testing.T) {
	err := validateDefinitionShape(definition{
		Nodes: [][]nodeRef{{{ID: "not-a-uuid"}}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCollectNodeIDs(t *testing.T) {
	a, b, c := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	ids := collectNodeIDs(definition{
		Nodes: [][]nodeRef{{{ID: a}, {ID: b}}, {{ID: c}}},
	})
	if len(ids) != 3 || ids[0] != a || ids[2] != c {
		t.Fatalf("got %v", ids)
	}
}
