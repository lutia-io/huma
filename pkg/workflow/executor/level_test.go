package executor

import (
	"encoding/json"
	"testing"
)

func TestMergeLevelInput_overlaysPreviousLevelOnly(t *testing.T) {
	base := map[string]any{"schemaId": "s1", "email": "a@b.c"}
	level0 := IndexedOutput(map[int]json.RawMessage{
		0: json.RawMessage(`{"items":[{"id":"1"}]}`),
		1: json.RawMessage(`{"status":200}`),
	})
	after0 := MergeLevelInput(base, level0)
	if after0["schemaId"] != "s1" || after0["0"] == nil || after0["1"] == nil {
		t.Fatalf("after level 0: %#v", after0)
	}

	// Next level only sees base enqueue fields + that level's outputs.
	// Earlier level outputs are not carried automatically.
	level1 := IndexedOutput(map[int]json.RawMessage{
		0: json.RawMessage(`{"name":"carried"}`),
	})
	after1 := MergeLevelInput(base, level1)
	if _, ok := after1["1"]; ok {
		t.Fatalf("level 0 node 1 should not appear after level 1: %#v", after1)
	}
	if after1["0"].(map[string]any)["name"] != "carried" {
		t.Fatalf("after level 1: %#v", after1)
	}
}

// Pass-through MAPPER pattern: to keep level-0 data available on level 2,
// a MAPPER on level 1 must copy the needed fields into its own output.
func TestPassThroughMapperPattern(t *testing.T) {
	base := map[string]any{"recordId": "r1"}
	level0 := IndexedOutput(map[int]json.RawMessage{
		0: json.RawMessage(`{"items":[{"id":"a"}],"total":1}`),
	})
	input1 := MergeLevelInput(base, level0)

	// Level 1 MAPPER copies prior list + enqueue field into one object.
	passThrough := map[string]any{
		"items":    input1["0"].(map[string]any)["items"],
		"recordId": input1["recordId"],
	}
	raw, err := json.Marshal(passThrough)
	if err != nil {
		t.Fatal(err)
	}
	level1 := IndexedOutput(map[int]json.RawMessage{0: raw})
	input2 := MergeLevelInput(base, level1)

	got := input2["0"].(map[string]any)
	if got["recordId"] != "r1" {
		t.Fatalf("pass-through lost recordId: %#v", got)
	}
	items, ok := got["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("pass-through lost items: %#v", got)
	}
}
