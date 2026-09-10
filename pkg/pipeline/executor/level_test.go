package executor

import (
	"encoding/json"
	"testing"
)

func TestIndexedOutput(t *testing.T) {
	got := indexedOutput(map[int]json.RawMessage{
		0: []byte(`{"status":200,"body":{"users":[{"id":"u1"}]}}`),
		1: []byte(`{"status":200,"body":{"name":"Acme"}}`),
	})
	one, ok := got["1"].(map[string]any)
	if !ok {
		t.Fatalf("got %#v", got["1"])
	}
	body, _ := one["body"].(map[string]any)
	if body["name"] != "Acme" {
		t.Fatalf("body.name = %#v", body["name"])
	}
	if _, ok := got["0"]; !ok {
		t.Fatal("missing index 0")
	}
}

func TestMergeLevelInput(t *testing.T) {
	got := mergeLevelInput(
		map[string]any{"salePrice": float64(100), "propertyId": "p1"},
		map[string]any{"0": map[string]any{"records": []any{}}},
	)
	if got["salePrice"] != float64(100) || got["propertyId"] != "p1" {
		t.Fatalf("lost enqueue input: %#v", got)
	}
	if _, ok := got["0"].(map[string]any); !ok {
		t.Fatalf("missing previous output: %#v", got)
	}
}
