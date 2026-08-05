// Package resolver interpolates templated values in action context data using
// the trigger record data of a workflow run.
//
// String values may embed expressions between {{ and }}. Expressions are
// evaluated by expr-lang (https://expr-lang.org). The trigger record is
// exposed as context.record, and formulas are namespaced functions declared
// in formulas.go:
//
//	{"email": "{{ context.record.email }}", "createdAt": "{{ date.now() }}", "total": "{{ jsonata.eval(\"$sum(items.price)\") }}"}
//
// A string that consists of exactly one expression resolves to the
// expression's typed value: numbers stay numbers, booleans stay booleans, and
// objects and arrays pass through as-is. Strings mixing literal text and
// expressions ("hi {{ context.record.name }}") resolve via string
// interpolation.
package resolver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
)

// Resolve returns a copy of data with every templated string value replaced
// by its resolved value. Nested maps and slices are resolved recursively;
// non-string values and strings without expressions pass through unchanged.
func Resolve(data map[string]any, record map[string]any) (map[string]any, error) {
	env := newEnv(record)
	resolved, err := resolveValue(data, env)
	if err != nil {
		return nil, err
	}
	return resolved.(map[string]any), nil
}

func resolveValue(v any, env map[string]any) (any, error) {
	switch v := v.(type) {
	case string:
		return resolveString(v, env)
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, elem := range v {
			rv, err := resolveValue(elem, env)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", k, err)
			}
			out[k] = rv
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, elem := range v {
			rv, err := resolveValue(elem, env)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			out[i] = rv
		}
		return out, nil
	default:
		return v, nil
	}
}

func resolveString(s string, env map[string]any) (any, error) {
	segs, err := splitSegments(s)
	if err != nil {
		return nil, err
	}
	// A value that is exactly one expression keeps its typed result.
	if len(segs) == 1 && segs[0].isExpr {
		return eval(segs[0].text, env)
	}
	var b strings.Builder
	for _, seg := range segs {
		if !seg.isExpr {
			b.WriteString(seg.text)
			continue
		}
		v, err := eval(seg.text, env)
		if err != nil {
			return nil, err
		}
		str, err := stringify(v)
		if err != nil {
			return nil, fmt.Errorf("expression %q: %w", seg.text, err)
		}
		b.WriteString(str)
	}
	return b.String(), nil
}

// segment is a run of literal text or the inside of one {{ ... }} expression.
type segment struct {
	isExpr bool
	text   string
}

func splitSegments(s string) ([]segment, error) {
	var segs []segment
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			if s != "" || len(segs) == 0 {
				segs = append(segs, segment{text: s})
			}
			return segs, nil
		}
		if i > 0 {
			segs = append(segs, segment{text: s[:i]})
		}
		rest := s[i+2:]
		j := strings.Index(rest, "}}")
		if j < 0 {
			return nil, fmt.Errorf("unclosed {{ in %q", s)
		}
		segs = append(segs, segment{isExpr: true, text: strings.TrimSpace(rest[:j])})
		s = rest[j+2:]
	}
}

func eval(src string, env map[string]any) (any, error) {
	program, err := expr.Compile(src, expr.Env(env))
	if err != nil {
		return nil, fmt.Errorf("compiling expression %q: %w", src, err)
	}
	v, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("evaluating expression %q: %w", src, err)
	}
	return v, nil
}

// stringify renders an expression result for interpolation into a larger
// string. Non-string values use their JSON encoding.
func stringify(v any) (string, error) {
	if s, ok := v.(string); ok {
		return s, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
