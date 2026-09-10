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

// mergeLevelInput keeps the pipeline enqueue fields (salePrice, schema ids)
// available on later levels, then overlays previous-level outputs as "0", "1", …
func mergeLevelInput(base, outputs map[string]any) map[string]any {
	if len(outputs) == 0 {
		return base
	}
	out := make(map[string]any, len(base)+len(outputs))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range outputs {
		out[k] = v
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
