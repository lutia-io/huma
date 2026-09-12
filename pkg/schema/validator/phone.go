package validator

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PhoneFormat is the JSON Schema format name for a telephone number.
// Schema authors use: {"type":"string","format":"phone"}
const PhoneFormat = "phone"

func validatePhoneFormat(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if !validPhone(s) {
		return fmt.Errorf("must be a phone number")
	}
	return nil
}

// ValidatePhoneKeywords checks format:phone properties have type string.
func ValidatePhoneKeywords(definition json.RawMessage) error {
	properties, err := Properties(definition)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	for _, property := range properties {
		if !isPhone(property) {
			continue
		}
		if property.Type != "" && property.Type != "string" {
			return fmt.Errorf("property %q: format phone requires type string", property.Name)
		}
	}
	return nil
}

func isPhone(property Property) bool {
	return strings.EqualFold(property.Format, PhoneFormat)
}

func validPhone(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	var b strings.Builder
	for i, r := range s {
		switch {
		case r == '+':
			if i != 0 || b.Len() != 0 {
				return false
			}
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
			continue
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			return false
		}
	}
	digits := strings.TrimPrefix(b.String(), "+")
	n := len(digits)
	if n < 7 || n > 15 {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
