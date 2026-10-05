package validator

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// ValidateFieldNames rejects a definition whose top-level properties share a
// display name. Comparison trims and ignores case. A property with no title
// uses a spaced version of its key. Nested properties are not compared.
func ValidateFieldNames(definition json.RawMessage) error {
	properties, err := Properties(definition)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	seen := make(map[string]struct{}, len(properties))
	for _, property := range properties {
		display := fieldDisplayName(property)
		key := strings.ToLower(strings.TrimSpace(display))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("a field named %s already exists", display)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func fieldDisplayName(property Property) string {
	title := strings.TrimSpace(property.Title)
	if title != "" {
		return title
	}
	return humanizeFieldKey(property.Name)
}

func humanizeFieldKey(name string) string {
	if strings.HasSuffix(name, "Id") {
		name = strings.TrimSuffix(name, "Id") + " ID"
	}
	var b strings.Builder
	var prev rune
	for i, r := range name {
		if i > 0 && prev >= 'a' && prev <= 'z' && r >= 'A' && r <= 'Z' {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		prev = r
	}
	result := b.String()
	if result == "" {
		return ""
	}
	runes := []rune(result)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
