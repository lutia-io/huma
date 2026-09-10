package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/lutia-io/huma/pkg/node"
)

func TestValidateDefinition_ok(t *testing.T) {
	err := validateDefinition(definition{
		Nodes: [][]Node{
			{
				{Name: "fetch", Type: node.TypeHTTP, Definition: node.HTTPContext{Method: "GET", URL: "https://example.com"}},
				{Name: "shape", Type: node.TypeMapper, Definition: node.MapperContext{Mapping: map[string]any{}}},
			},
			{
				{Name: "noop", Type: node.TypeNoop, Definition: node.NoopContext{Message: "ok"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateDefinition_empty(t *testing.T) {
	if err := validateDefinition(definition{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateDefinition_emptyLevel(t *testing.T) {
	err := validateDefinition(definition{
		Nodes: [][]Node{{}, {{Name: "noop", Type: node.TypeNoop, Definition: node.NoopContext{}}}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateDefinition_missingName(t *testing.T) {
	err := validateDefinition(definition{
		Nodes: [][]Node{{{Type: node.TypeNoop, Definition: node.NoopContext{}}}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNodeUnmarshalJSON(t *testing.T) {
	raw := []byte(`{
		"name": "Fetch rates",
		"type": "HTTP",
		"definition": {"method": "get", "url": "https://example.com"}
	}`)
	var n Node
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatal(err)
	}
	if n.Name != "Fetch rates" || n.Type != node.TypeHTTP {
		t.Fatalf("got name=%s type=%s", n.Name, n.Type)
	}
	ctx, ok := n.Definition.(node.HTTPContext)
	if !ok || ctx.Method != "GET" || ctx.URL != "https://example.com" {
		t.Fatalf("got %#v", n.Definition)
	}
}

func TestSnapshotDefinition_slugFromName(t *testing.T) {
	snap := snapshotDefinition(definition{
		Nodes: [][]Node{{{
			Name:       "Fetch Rates",
			Type:       node.TypeHTTP,
			Definition: node.HTTPContext{Method: "GET", URL: "https://example.com"},
		}}},
	})
	if len(snap.Nodes) != 1 || len(snap.Nodes[0]) != 1 {
		t.Fatalf("got %#v", snap.Nodes)
	}
	got := snap.Nodes[0][0]
	if got.Name != "Fetch Rates" || got.Slug != "fetch-rates" || got.Type != node.TypeHTTP {
		t.Fatalf("got %#v", got)
	}
}
