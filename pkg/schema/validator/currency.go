package validator

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CurrencyFormat is the JSON Schema format name for a decimal money amount.
// Schema authors use: {"type":"number","format":"currency"}
// The value is a JSON number (dollars and cents, not a string).
const CurrencyFormat = "currency"

// validateCurrencyFormat accepts any value. A currency amount is a JSON
// number, and the type keyword already requires that.
func validateCurrencyFormat(any) error {
	return nil
}

// ValidateCurrencyKeywords checks format:currency properties have type number.
func ValidateCurrencyKeywords(definition json.RawMessage) error {
	properties, err := Properties(definition)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	for _, property := range properties {
		if !isCurrency(property) {
			continue
		}
		if property.Type != "" && property.Type != "number" && property.Type != "integer" {
			return fmt.Errorf("property %q: format currency requires type number", property.Name)
		}
	}
	return nil
}

func isCurrency(property Property) bool {
	return strings.EqualFold(property.Format, CurrencyFormat)
}
