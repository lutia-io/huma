package validator

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AddressFormat is the JSON Schema format name for a postal address object.
// Schema authors use: {"type":"object","format":"address"}
const AddressFormat = "address"

func validateAddressFormat(v any) error {
	if v == nil {
		return nil
	}
	if _, ok := v.(map[string]any); ok {
		return nil
	}
	return fmt.Errorf("must be an address object")
}

// ValidateAddressKeywords checks format:address properties have type object.
func ValidateAddressKeywords(definition json.RawMessage) error {
	properties, err := Properties(definition)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	for _, property := range properties {
		if !isAddress(property) {
			continue
		}
		if property.Type != "" && property.Type != "object" {
			return fmt.Errorf("property %q: format address requires type object", property.Name)
		}
	}
	return nil
}

func isAddress(property Property) bool {
	return strings.EqualFold(property.Format, AddressFormat)
}
