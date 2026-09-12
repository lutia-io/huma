package validator

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateFileKeywords checks format:file properties are a file ID string or an array of file IDs.
func ValidateFileKeywords(definition json.RawMessage) error {
	properties, err := Properties(definition)
	if err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	for _, property := range properties {
		if !isFile(property) {
			continue
		}
		if property.Type != "" && property.Type != "string" && property.Type != "array" {
			return fmt.Errorf("property %q: format file requires type string or array", property.Name)
		}
	}
	return nil
}

func isFile(property Property) bool {
	return strings.EqualFold(property.Format, FileFormat)
}
