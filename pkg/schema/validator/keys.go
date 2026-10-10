package validator

import (
	"encoding/json"
	"fmt"
)

// RejectRenamedFieldKeys reports an error when next both adds and removes
// top-level property keys relative to previous. That combination is how a
// rename replaces a key in one save. Title and type edits on the same keys
// are allowed. Adding keys, or removing keys, but not both, is allowed.
func RejectRenamedFieldKeys(previous, next json.RawMessage) error {
	before, err := Properties(previous)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	after, err := Properties(next)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}

	oldKeys := make(map[string]struct{}, len(before))
	for _, property := range before {
		oldKeys[property.Name] = struct{}{}
	}

	added := 0
	for _, property := range after {
		if _, ok := oldKeys[property.Name]; ok {
			delete(oldKeys, property.Name)
			continue
		}
		added++
	}
	if added > 0 && len(oldKeys) > 0 {
		return fmt.Errorf("field keys cannot be renamed")
	}
	return nil
}
