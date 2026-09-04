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
// Schema authors use: {"type":"string","format":"file"}
const FileFormat = "file"

// ForeignFormat is the JSON Schema format name for a related record ID.
// Schema authors use: {"type":"string","format":"foreign","schemaId":"<uuid>"}
const ForeignFormat = "foreign"

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

func compile(definition json.RawMessage) (*jsonschema.Schema, error) {
	if len(bytes.TrimSpace(definition)) == 0 {
		return nil, fmt.Errorf("definition is required")
	}

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
		Name:     AddressFormat,
		Validate: validateAddressFormat,
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
	return ValidateDefaultKeywords(definition)
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
