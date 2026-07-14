package validator

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaURL = "schema.json"

type deniedLoader struct{}

func (deniedLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema references are not allowed")
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
	_, err := compile(definition)
	return err
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
