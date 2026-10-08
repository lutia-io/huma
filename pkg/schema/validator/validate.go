package validator

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/lutia-io/huma/pkg/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaURL = "schema.json"

// FileFormat is the JSON Schema format name for a file ID reference.
// Schema authors use either a single ID:
//
//	{"type":"string","format":"file"}
//
// or an array of IDs:
//
//	{"type":"array","format":"file","items":{"type":"string","format":"file"}}
const FileFormat = "file"

// ForeignFormat is the JSON Schema format name for a related record ID.
// Schema authors use: {"type":"string","format":"foreign","schemaId":"<uuid>"}
const ForeignFormat = "foreign"

// UserFormat is the JSON Schema format name for an organization user ID.
// Schema authors use: {"type":"string","format":"user"}
const UserFormat = "user"

type deniedLoader struct{}

func (deniedLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema references are not allowed")
}

// validateFileFormat asserts the value is a UUID string (a file ID).
func validateFileFormat(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if !uuid.Valid(s) {
		return fmt.Errorf("must be a file id (uuid)")
	}
	return nil
}

// validateForeignFormat asserts the value is a UUID string (a record ID).
func validateForeignFormat(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if !uuid.Valid(s) {
		return fmt.Errorf("must be a record id (uuid)")
	}
	return nil
}

// validateUserFormat asserts the value is a UUID string (an organization user ID).
func validateUserFormat(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if !uuid.Valid(s) {
		return fmt.Errorf("must be an organization user id (uuid)")
	}
	return nil
}

func compile(definition json.RawMessage) (*jsonschema.Schema, error) {
	if len(bytes.TrimSpace(definition)) == 0 {
		return nil, fmt.Errorf("definition is required")
	}

	definition = asNumberTypes(definition)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(definition))
	if err != nil {
		return nil, fmt.Errorf("invalid definition: %w", err)
	}

	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	c.RegisterFormat(&jsonschema.Format{
		Name:     FileFormat,
		Validate: validateFileFormat,
	})
	c.RegisterFormat(&jsonschema.Format{
		Name:     ForeignFormat,
		Validate: validateForeignFormat,
	})
	c.RegisterFormat(&jsonschema.Format{
		Name:     UserFormat,
		Validate: validateUserFormat,
	})
	c.RegisterFormat(&jsonschema.Format{
		Name:     AddressFormat,
		Validate: validateAddressFormat,
	})
	c.RegisterFormat(&jsonschema.Format{
		Name:     PhoneFormat,
		Validate: validatePhoneFormat,
	})
	c.UseLoader(deniedLoader{})

	if err := c.AddResource(schemaURL, doc); err != nil {
		return nil, fmt.Errorf("invalid definition: %w", err)
	}
	sch, err := c.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("invalid definition: %w", err)
	}
	return sch, nil
}

// ValidateDefinition validates a JSON Schema definition.
func ValidateDefinition(definition json.RawMessage) error {
	if _, err := compile(definition); err != nil {
		return err
	}
	if err := ValidateForeignKeywords(definition); err != nil {
		return err
	}
	if err := ValidateAddressKeywords(definition); err != nil {
		return err
	}
	if err := ValidateFileKeywords(definition); err != nil {
		return err
	}
	if err := ValidateUserKeywords(definition); err != nil {
		return err
	}
	if err := ValidatePhoneKeywords(definition); err != nil {
		return err
	}
	if err := ValidateDefaultKeywords(definition); err != nil {
		return err
	}
	return ValidateFieldNames(definition)
}

// PrepareUpdate validates next against definition for a record rewrite.
// When additional properties are forbidden, a key that is no longer in the
// schema is still accepted if that key already exists on previous. The
// returned document is next unchanged, so removed columns stay stored until a
// writer omits them. A new unknown key is rejected.
func PrepareUpdate(definition, previous, next json.RawMessage) (json.RawMessage, error) {
	if !forbidsAdditionalProperties(definition) {
		if err := ValidateData(definition, next); err != nil {
			return nil, err
		}
		return next, nil
	}

	properties, err := Properties(definition)
	if err != nil {
		return nil, fmt.Errorf("invalid definition: %w", err)
	}
	known := make(map[string]struct{}, len(properties))
	for _, property := range properties {
		known[property.Name] = struct{}{}
	}

	var prev map[string]json.RawMessage
	if len(bytes.TrimSpace(previous)) > 0 && string(previous) != "null" {
		if err := json.Unmarshal(previous, &prev); err != nil {
			return nil, fmt.Errorf("invalid data: %w", err)
		}
	}
	var nextObj map[string]json.RawMessage
	if err := json.Unmarshal(next, &nextObj); err != nil {
		return nil, fmt.Errorf("invalid data: %w", err)
	}

	stripped := make(map[string]json.RawMessage, len(nextObj))
	for key, value := range nextObj {
		if _, ok := known[key]; ok {
			stripped[key] = value
			continue
		}
		if _, ok := prev[key]; ok {
			continue
		}
		return nil, fmt.Errorf("additional properties not allowed: %s", key)
	}

	raw, err := json.Marshal(stripped)
	if err != nil {
		return nil, err
	}
	if err := ValidateData(definition, raw); err != nil {
		return nil, err
	}
	return next, nil
}

func forbidsAdditionalProperties(definition json.RawMessage) bool {
	var doc struct {
		Additional json.RawMessage `json:"additionalProperties"`
	}
	if err := json.Unmarshal(definition, &doc); err != nil {
		return false
	}
	if len(bytes.TrimSpace(doc.Additional)) == 0 {
		return false
	}
	var allowed bool
	if err := json.Unmarshal(doc.Additional, &allowed); err != nil {
		return false
	}
	return !allowed
}

// asNumberTypes rewrites JSON Schema "integer" to "number". Schemas have one
// numeric type, and older whole-number columns stay valid.
func asNumberTypes(definition json.RawMessage) json.RawMessage {
	var doc any
	if err := json.Unmarshal(definition, &doc); err != nil {
		return definition
	}
	if !rewriteIntegerTypes(doc) {
		return definition
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return definition
	}
	return out
}

func rewriteIntegerTypes(v any) bool {
	changed := false
	switch n := v.(type) {
	case map[string]any:
		switch t := n["type"].(type) {
		case string:
			if t == "integer" {
				n["type"] = "number"
				changed = true
			}
		case []any:
			for i, item := range t {
				if s, ok := item.(string); ok && s == "integer" {
					t[i] = "number"
					changed = true
				}
			}
		}
		for _, child := range n {
			if rewriteIntegerTypes(child) {
				changed = true
			}
		}
	case []any:
		for _, child := range n {
			if rewriteIntegerTypes(child) {
				changed = true
			}
		}
	}
	return changed
}

// ValidateData validates data against a JSON Schema definition.
func ValidateData(definition json.RawMessage, data json.RawMessage) error {
	sch, err := compile(definition)
	if err != nil {
		return err
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("data is required")
	}

	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid data: %w", err)
	}
	return sch.Validate(inst)
}
