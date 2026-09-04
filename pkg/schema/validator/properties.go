package validator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lutia-io/huma/pkg/uuid"
)

// Property is one JSON Schema object property, in document order.
type Property struct {
	Name     string
	Type     string
	Format   string
	SchemaID string
	Enum     []string
	Default  json.RawMessage
}

// ForeignField is a property with format "foreign".
type ForeignField struct {
	Name     string
	SchemaID string
}

// Properties returns top-level object properties in document order.
func Properties(definition json.RawMessage) ([]Property, error) {
	if len(bytes.TrimSpace(definition)) == 0 {
		return nil, nil
	}

	var doc struct {
		Properties json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(definition, &doc); err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(doc.Properties)) == 0 {
		return nil, nil
	}

	dec := json.NewDecoder(bytes.NewReader(doc.Properties))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, nil
	}

	properties := make([]Property, 0)
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, _ := keyTok.(string)
		var spec struct {
			Type     json.RawMessage `json:"type"`
			Format   string          `json:"format"`
			SchemaID string          `json:"schemaId"`
			Enum     []string        `json:"enum"`
			Default  json.RawMessage `json:"default"`
		}
		if err := dec.Decode(&spec); err != nil {
			return nil, err
		}
		properties = append(properties, Property{
			Name:     name,
			Type:     jsonTypeName(spec.Type),
			Format:   spec.Format,
			SchemaID: strings.TrimSpace(spec.SchemaID),
			Enum:     spec.Enum,
			Default:  spec.Default,
		})
	}
	return properties, nil
}

// ForeignFields returns properties that reference another schema.
func ForeignFields(definition json.RawMessage) ([]ForeignField, error) {
	properties, err := Properties(definition)
	if err != nil {
		return nil, err
	}
	fields := make([]ForeignField, 0)
	for _, property := range properties {
		if !isForeign(property) {
			continue
		}
		fields = append(fields, ForeignField{
			Name:     property.Name,
			SchemaID: property.SchemaID,
		})
	}
	return fields, nil
}

// ValidateForeignKeywords checks format:foreign properties have a UUID schemaId.
func ValidateForeignKeywords(definition json.RawMessage) error {
	properties, err := Properties(definition)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	for _, property := range properties {
		if !isForeign(property) {
			continue
		}
		if property.Type != "" && property.Type != "string" {
			return fmt.Errorf("property %q: format foreign requires type string", property.Name)
		}
		if property.SchemaID == "" {
			return fmt.Errorf("property %q: schemaId is required for format foreign", property.Name)
		}
		if !uuid.Valid(property.SchemaID) {
			return fmt.Errorf("property %q: schemaId must be a uuid", property.Name)
		}
	}
	return nil
}

// TitleKey is the first string property suitable as a record display title.
func TitleKey(definition json.RawMessage) string {
	properties, err := Properties(definition)
	if err != nil {
		return ""
	}
	for _, property := range properties {
		if isTitleProperty(property) {
			return property.Name
		}
	}
	return ""
}

// DisplayTitle returns the first title-suitable string in data, or fallback.
func DisplayTitle(data json.RawMessage, definition json.RawMessage, fallback string) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return fallback
	}
	properties, err := Properties(definition)
	if err != nil {
		return fallback
	}
	for _, property := range properties {
		if !isTitleProperty(property) {
			continue
		}
		raw, ok := obj[property.Name]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	if fallback != "" {
		return fallback
	}
	return "Record"
}

func isForeign(property Property) bool {
	return strings.EqualFold(property.Format, ForeignFormat)
}

func isTitleProperty(property Property) bool {
	if property.Type != "" && property.Type != "string" {
		return false
	}
	if strings.EqualFold(property.Format, FileFormat) || isForeign(property) || isAddress(property) {
		return false
	}
	return len(property.Enum) == 0
}

func jsonTypeName(typeJSON json.RawMessage) string {
	if len(typeJSON) == 0 {
		return "string"
	}
	var asString string
	if err := json.Unmarshal(typeJSON, &asString); err == nil {
		return asString
	}
	var asList []string
	if err := json.Unmarshal(typeJSON, &asList); err == nil && len(asList) > 0 {
		return asList[0]
	}
	return "string"
}
