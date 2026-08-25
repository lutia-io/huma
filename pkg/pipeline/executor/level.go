package executor

import (
	"encoding/json"
	"strconv"
)

func indexedOutput(outputs map[int]json.RawMessage) map[string]any {
	out := make(map[string]any, len(outputs))
	for i, raw := range outputs {
		var v any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &v); err != nil {
				v = json.RawMessage(raw)
			}
		}
		out[strconv.Itoa(i)] = v
	}
	return out
}

func nodeInputJSON(input map[string]any) []byte {
	if input == nil {
		return []byte("{}")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil
	}
	return raw
}
