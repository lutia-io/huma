package validator

import (
	"encoding/json"
	"fmt"
	"strings"
)

// UserField is a property with format "user".
type UserField struct {
	Name string
}

// UserFields returns properties that store an organization user ID.
func UserFields(definition json.RawMessage) ([]UserField, error) {
	properties, err := Properties(definition)
	if err != nil {
		return nil, err
	}
	fields := make([]UserField, 0)
	for _, property := range properties {
		if !isUser(property) {
			continue
		}
		fields = append(fields, UserField{Name: property.Name})
	}
	return fields, nil
}

// ValidateUserKeywords checks format:user properties are a single user ID string.
func ValidateUserKeywords(definition json.RawMessage) error {
	properties, err := Properties(definition)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	for _, property := range properties {
		if !isUser(property) {
			continue
		}
		if property.Type != "" && property.Type != "string" {
			return fmt.Errorf("property %q: format user requires type string", property.Name)
		}
	}
	return nil
}

func isUser(property Property) bool {
	return strings.EqualFold(property.Format, UserFormat)
}
