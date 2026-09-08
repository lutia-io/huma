package record

import (
	"encoding/json"
	"reflect"
	"sort"
)

// ChangedFields returns the sorted union of top-level keys whose values differ
// between before and after. Nested edits count as a change to that property.
func ChangedFields(before, after map[string]any) []string {
	keys := make(map[string]struct{}, len(before)+len(after))
	for k := range before {
		keys[k] = struct{}{}
	}
	for k := range after {
		keys[k] = struct{}{}
	}
	changed := make([]string, 0, len(keys))
	for k := range keys {
		if !reflect.DeepEqual(before[k], after[k]) {
			changed = append(changed, k)
		}
	}
	sort.Strings(changed)
	return changed
}

// EqualDocuments reports whether two JSON objects have the same top-level
// content. Non-objects are compared as raw JSON.
func EqualDocuments(before, after json.RawMessage) bool {
	left, leftOK := unmarshalObject(before)
	right, rightOK := unmarshalObject(after)
	if leftOK && rightOK {
		return reflect.DeepEqual(left, right)
	}
	return string(before) == string(after)
}

func unmarshalObject(data json.RawMessage) (map[string]any, bool) {
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, false
	}
	return object, true
}
