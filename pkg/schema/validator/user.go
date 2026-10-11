package validator

import (
	"encoding/json"
	"fmt"
	"strings"
)

// UserField is a property that stores one organization user ID, or a list of them.
type UserField struct {
	Name     string
	Multiple bool
}

// UserFields returns properties that store an organization user ID.
// A list of users is an array whose items use format user. Format user on the
// array itself is not a user field.
func UserFields(definition json.RawMessage) ([]UserField, error) {
	properties, err := Properties(definition)
	if err != nil {
		return nil, err
	}
	fields := make([]UserField, 0)
	for _, property := range properties {
		switch {
		case isUser(property):
			fields = append(fields, UserField{Name: property.Name})
		case isUserList(property):
			fields = append(fields, UserField{Name: property.Name, Multiple: true})
		}
	}
	return fields, nil
}

// ValidateUserKeywords checks format:user properties are a single user ID string,
// and that format:user on array items is a list of user ID strings.
func ValidateUserKeywords(definition json.RawMessage) error {
	properties, err := Properties(definition)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	for _, property := range properties {
		if isUser(property) {
			if property.Type != "" && property.Type != "string" {
				return fmt.Errorf("property %q: format user requires type string", property.Name)
			}
		}
		if isUserItem(property) {
			if property.Type != "array" {
				return fmt.Errorf("property %q: format user items require type array", property.Name)
			}
			if property.ItemsType != "" && property.ItemsType != "string" {
				return fmt.Errorf("property %q: format user items require type string", property.Name)
			}
		}
	}
	return nil
}

func isUser(property Property) bool {
	return strings.EqualFold(property.Format, UserFormat)
}

func isUserItem(property Property) bool {
	return strings.EqualFold(property.ItemsFormat, UserFormat)
}

func isUserList(property Property) bool {
	return property.Type == "array" && isUserItem(property) && !isUser(property)
}
