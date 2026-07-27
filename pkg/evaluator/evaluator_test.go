package evaluator

import (
	"testing"

	"github.com/lutia-io/huma/pkg/criteria"
)

func TestMatchShipmentExample(t *testing.T) {
	// weight > 200 OR (numOfContainers > 500 AND numOfPeople < 5)
	c := criteria.Criteria{
		Logic: criteria.LogicOr,
		Conditions: []criteria.Criteria{
			{Field: "weight", Operator: criteria.OpGt, Value: 200},
			{
				Logic: criteria.LogicAnd,
				Conditions: []criteria.Criteria{
					{Field: "numOfContainers", Operator: criteria.OpGt, Value: 500},
					{Field: "numOfPeople", Operator: criteria.OpLt, Value: 5},
				},
			},
		},
	}

	tests := []struct {
		name string
		data map[string]any
		want bool
	}{
		{
			name: "weight alone matches",
			data: map[string]any{"weight": float64(250), "numOfContainers": float64(1), "numOfPeople": float64(10)},
			want: true,
		},
		{
			name: "containers and people match",
			data: map[string]any{"weight": float64(100), "numOfContainers": float64(600), "numOfPeople": float64(2)},
			want: true,
		},
		{
			name: "neither branch matches",
			data: map[string]any{"weight": float64(100), "numOfContainers": float64(100), "numOfPeople": float64(10)},
			want: false,
		},
		{
			name: "missing field fails leaf",
			data: map[string]any{"numOfContainers": float64(600), "numOfPeople": float64(2)},
			want: true, // second branch still matches
		},
		{
			name: "missing field on both branches",
			data: map[string]any{"weight": float64(100)},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Match(c, tt.data); got != tt.want {
				t.Fatalf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchOperators(t *testing.T) {
	data := map[string]any{
		"status": "open",
		"count":  float64(3),
		"nested": map[string]any{"flag": true},
	}

	if !Match(criteria.Criteria{Field: "status", Operator: criteria.OpEq, Value: "open"}, data) {
		t.Fatal("eq failed")
	}
	if Match(criteria.Criteria{Field: "status", Operator: criteria.OpNeq, Value: "open"}, data) {
		t.Fatal("neq should fail")
	}
	if !Match(criteria.Criteria{Field: "count", Operator: criteria.OpGte, Value: 3}, data) {
		t.Fatal("gte failed")
	}
	if !Match(criteria.Criteria{Field: "count", Operator: criteria.OpIn, Value: []any{1, 3, 5}}, data) {
		t.Fatal("in failed")
	}
	if !Match(criteria.Criteria{Field: "nested.flag", Operator: criteria.OpEq, Value: true}, data) {
		t.Fatal("nested eq failed")
	}
	if Match(criteria.Criteria{Logic: criteria.LogicNot, Conditions: []criteria.Criteria{{Field: "status", Operator: criteria.OpEq, Value: "open"}}}, data) {
		t.Fatal("not should fail when child matches")
	}
}
