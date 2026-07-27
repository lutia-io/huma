package evaluator

import (
	"reflect"
	"strings"

	"github.com/lutia-io/huma/pkg/criteria"
)

// Match evaluates the criteria tree against record data.
// Missing fields fail leaf comparisons. Malformed nodes fail.
func Match(c criteria.Criteria, data map[string]any) bool {
	if c.Logic != "" {
		return matchGroup(c, data)
	}
	if c.Field == "" || c.Operator == "" {
		return false
	}
	value, ok := lookupField(data, c.Field)
	if !ok {
		return false
	}
	return compare(value, c.Operator, c.Value)
}

func matchGroup(c criteria.Criteria, data map[string]any) bool {
	switch c.Logic {
	case criteria.LogicAnd:
		if len(c.Conditions) == 0 {
			return false
		}
		for _, child := range c.Conditions {
			if !Match(child, data) {
				return false
			}
		}
		return true
	case criteria.LogicOr:
		if len(c.Conditions) == 0 {
			return false
		}
		for _, child := range c.Conditions {
			if Match(child, data) {
				return true
			}
		}
		return false
	case criteria.LogicNot:
		if len(c.Conditions) != 1 {
			return false
		}
		return !Match(c.Conditions[0], data)
	default:
		return false
	}
}

func lookupField(data map[string]any, field string) (any, bool) {
	if data == nil {
		return nil, false
	}
	if !strings.Contains(field, ".") {
		v, ok := data[field]
		return v, ok
	}
	var cur any = data
	for part := range strings.SplitSeq(field, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func compare(left any, op criteria.CompareOp, right any) bool {
	switch op {
	case criteria.OpEq:
		return equalValues(left, right)
	case criteria.OpNeq:
		return !equalValues(left, right)
	case criteria.OpIn:
		return containsValue(right, left)
	case criteria.OpGt, criteria.OpGte, criteria.OpLt, criteria.OpLte:
		cmp, ok := compareOrdered(left, right)
		if !ok {
			return false
		}
		switch op {
		case criteria.OpGt:
			return cmp > 0
		case criteria.OpGte:
			return cmp >= 0
		case criteria.OpLt:
			return cmp < 0
		case criteria.OpLte:
			return cmp <= 0
		}
	}
	return false
}

func containsValue(haystack any, needle any) bool {
	rv := reflect.ValueOf(haystack)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return false
	}
	for i := 0; i < rv.Len(); i++ {
		if equalValues(rv.Index(i).Interface(), needle) {
			return true
		}
	}
	return false
}

func equalValues(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	if af, aok := asFloat(a); aok {
		if bf, bok := asFloat(b); bok {
			return af == bf
		}
	}
	return reflect.DeepEqual(a, b)
}

func compareOrdered(a, b any) (int, bool) {
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if aok && bok {
		switch {
		case af < bf:
			return -1, true
		case af > bf:
			return 1, true
		default:
			return 0, true
		}
	}

	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		return strings.Compare(as, bs), true
	}
	return 0, false
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}
