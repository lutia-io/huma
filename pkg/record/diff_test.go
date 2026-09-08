package record

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestChangedFields(t *testing.T) {
	before := map[string]any{"status": "pending", "notes": "a", "weight": float64(1)}
	after := map[string]any{"status": "shipped", "notes": "a", "weight": float64(1)}
	got := ChangedFields(before, after)
	want := []string{"status"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ChangedFields() = %v, want %v", got, want)
	}
}

func TestChangedFields_nestedCountsAsTopLevel(t *testing.T) {
	before := map[string]any{"address": map[string]any{"city": "A"}}
	after := map[string]any{"address": map[string]any{"city": "B"}}
	got := ChangedFields(before, after)
	if len(got) != 1 || got[0] != "address" {
		t.Fatalf("ChangedFields() = %v", got)
	}
}

func TestChangedFields_addedAndRemoved(t *testing.T) {
	before := map[string]any{"keep": "x", "gone": "y"}
	after := map[string]any{"keep": "x", "new": "z"}
	got := ChangedFields(before, after)
	want := []string{"gone", "new"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ChangedFields() = %v, want %v", got, want)
	}
}

func TestEqualDocuments(t *testing.T) {
	if !EqualDocuments(json.RawMessage(`{"a":1}`), json.RawMessage(`{"a": 1}`)) {
		t.Fatal("equal objects should match")
	}
	if EqualDocuments(json.RawMessage(`{"a":1}`), json.RawMessage(`{"a":2}`)) {
		t.Fatal("different objects should not match")
	}
}
