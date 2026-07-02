package schema

import (
	"encoding/json"
	"fmt"
	"strings"
)

type fieldValidator func(value any) error

var fieldValidators = map[string]fieldValidator{
	"text":    validateText,
	"number":  validateNumber,
	"boolean": validateBoolean,
}

func ParseDefinition(raw json.RawMessage) (Definition, error) {
	if len(raw) == 0 {
		return Definition{}, fmt.Errorf("definition is required")
	}

	var def Definition
	if err := json.Unmarshal(raw, &def); err != nil {
		return Definition{}, fmt.Errorf("invalid definition: %w", err)
	}

	if err := validateDefinition(def); err != nil {
		return Definition{}, err
	}

	return def, nil
}

func ValidateDefinition(raw json.RawMessage) error {
	_, err := ParseDefinition(raw)
	return err
}

func ValidateData(definition json.RawMessage, data json.RawMessage) error {
	def, err := ParseDefinition(definition)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return fmt.Errorf("data is required")
	}

	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("invalid data: %w", err)
	}

	allowed := make(map[string]Field, len(def.Fields))
	for _, field := range def.Fields {
		allowed[field.Name] = field
	}

	for name := range values {
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("unknown field %q", name)
		}
	}

	for _, field := range def.Fields {
		value, ok := values[field.Name]
		if !ok {
			if field.Required {
				return fmt.Errorf("field %q is required", field.Name)
			}
			continue
		}
		if value == nil {
			if field.Required {
				return fmt.Errorf("field %q is required", field.Name)
			}
			continue
		}
		if err := validateSingle(field.Type, value); err != nil {
			return fmt.Errorf("field %q: %w", field.Name, err)
		}
	}

	return nil
}

func validateDefinition(def Definition) error {
	if def.Fields == nil {
		return fmt.Errorf("fields is required")
	}

	seen := make(map[string]struct{}, len(def.Fields))
	for _, field := range def.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return fmt.Errorf("field name is required")
		}
		if field.Name != name {
			return fmt.Errorf("field name %q must not contain leading or trailing spaces", field.Name)
		}
		if _, ok := seen[field.Name]; ok {
			return fmt.Errorf("duplicate field name %q", field.Name)
		}
		seen[field.Name] = struct{}{}

		fieldType := strings.TrimSpace(field.Type)
		if fieldType == "" {
			return fmt.Errorf("field %q: type is required", field.Name)
		}
		if _, ok := fieldValidators[fieldType]; !ok {
			return fmt.Errorf("field %q: unknown type %q", field.Name, field.Type)
		}
		if field.Type != fieldType {
			return fmt.Errorf("field %q: type must not contain leading or trailing spaces", field.Name)
		}
	}

	return nil
}

func validateSingle(fieldType string, value any) error {
	validate, ok := fieldValidators[fieldType]
	if !ok {
		return fmt.Errorf("unknown type %q", fieldType)
	}
	return validate(value)
}

func validateText(value any) error {
	if _, ok := value.(string); !ok {
		return fmt.Errorf("must be text")
	}
	return nil
}

func validateNumber(value any) error {
	switch value.(type) {
	case float64, json.Number:
		return nil
	default:
		return fmt.Errorf("must be a number")
	}
}

func validateBoolean(value any) error {
	if _, ok := value.(bool); !ok {
		return fmt.Errorf("must be a boolean")
	}
	return nil
}
