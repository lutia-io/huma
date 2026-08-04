// Package resolver interpolates templated values in action context data using
// the trigger record data of a workflow run.
package resolver

// Resolve returns the action data with template expressions substituted from
// the trigger data.
//
// Stub: currently returns data unchanged. The full implementation will use
// text/template so action definitions can reference trigger fields, e.g.
// {"email": "{{ .trigger.email }}"}.
func Resolve(data map[string]any, trigger map[string]any) (map[string]any, error) {
	return data, nil
}
