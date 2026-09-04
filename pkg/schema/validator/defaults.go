package validator

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/resolver"
)

// ApplyDefaults copies schema property defaults into data for keys that are
// absent. String defaults that contain "{{" are evaluated with the resolver
// against an empty trigger ({{ now }}, {{ uuid }}, {{ add 1 1 }}). Existing
// keys, including explicit null, are left unchanged. The input is not mutated.
func ApplyDefaults(definition json.RawMessage, data json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return data, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("invalid data: %w", err)
	}
	if obj == nil {
		obj = map[string]json.RawMessage{}
	}

	properties, err := Properties(definition)
	if err != nil {
		return nil, fmt.Errorf("invalid definition: %w", err)
	}

	applied := false
	for _, property := range properties {
		if len(property.Default) == 0 {
			continue
		}
		if _, exists := obj[property.Name]; exists {
			continue
		}
		value, err := resolveDefault(property.Default)
		if err != nil {
			return nil, fmt.Errorf("property %q: default: %w", property.Name, err)
		}
		obj[property.Name] = value
		applied = true
	}
	if !applied {
		return data, nil
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ValidateDefaultKeywords checks that templated default strings parse and
// execute against an empty trigger.
func ValidateDefaultKeywords(definition json.RawMessage) error {
	properties, err := Properties(definition)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	for _, property := range properties {
		if len(property.Default) == 0 {
			continue
		}
		if _, err := resolveDefault(property.Default); err != nil {
			return fmt.Errorf("property %q: default: %w", property.Name, err)
		}
	}
	return nil
}

func resolveDefault(raw json.RawMessage) (json.RawMessage, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	s, ok := v.(string)
	if !ok {
		return raw, nil
	}
	resolved, err := resolver.ResolveOne(s, resolver.Trigger{})
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(resolved)
	if err != nil {
		return nil, err
	}
	return out, nil
}
