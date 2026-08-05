package resolver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/recolabs/gnata"
)

// nowFunc returns the current time; a package variable so tests can pin the
// clock.
var nowFunc = time.Now

// jsonataCache holds compiled JSONata expressions keyed by source string.
// gnata.Expression is goroutine-safe and intended for reuse.
var jsonataCache sync.Map // map[string]*gnata.Expression

// newEnv builds the expression environment for one Resolve call: the context
// data plus the namespaced formula library. Formulas are grouped by
// namespace, so expressions call them as date.now(), math.add(1, 2),
// jsonata.eval("..."), etc.
func newEnv(record map[string]any) map[string]any {
	if record == nil {
		record = map[string]any{}
	}
	return map[string]any{
		"context": map[string]any{
			"record": record,
		},
		"date": map[string]any{
			// now returns the current UTC time in RFC 3339 format,
			// e.g. "2026-08-04T21:45:00Z".
			"now": func() string {
				return nowFunc().UTC().Format(time.RFC3339)
			},
		},
		"math": map[string]any{
			"add": func(a, b any) (float64, error) {
				x, err := toFloat(a)
				if err != nil {
					return 0, err
				}
				y, err := toFloat(b)
				if err != nil {
					return 0, err
				}
				return x + y, nil
			},
		},
		"jsonata": map[string]any{
			// eval runs a JSONata expression against the trigger record.
			// Example: jsonata.eval("$sum(items.price)")
			"eval": func(expression string) (any, error) {
				return evalJSONata(expression, record)
			},
		},
	}
}

func evalJSONata(expression string, data map[string]any) (any, error) {
	compiled, err := compileJSONata(expression)
	if err != nil {
		return nil, err
	}
	result, err := compiled.Eval(context.Background(), data)
	if err != nil {
		return nil, fmt.Errorf("evaluating jsonata %q: %w", expression, err)
	}
	return gnata.NormalizeValue(result), nil
}

func compileJSONata(expression string) (*gnata.Expression, error) {
	if v, ok := jsonataCache.Load(expression); ok {
		return v.(*gnata.Expression), nil
	}
	compiled, err := gnata.Compile(expression)
	if err != nil {
		return nil, fmt.Errorf("compiling jsonata %q: %w", expression, err)
	}
	actual, _ := jsonataCache.LoadOrStore(expression, compiled)
	return actual.(*gnata.Expression), nil
}

// toFloat coerces the two numeric representations that reach formulas: JSON
// numbers decode as float64, while integer literals in expressions are int.
func toFloat(v any) (float64, error) {
	switch v := v.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("expected a number, got %T", v)
	}
}
