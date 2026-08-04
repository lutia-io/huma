package criteria

import "testing"

func TestMatchShipmentExample(t *testing.T) {
	// weight > 200 OR (numOfContainers > 500 AND numOfPeople < 5)
	c := Criteria{
		Logic: LogicOr,
		Conditions: []Criteria{
			{Field: "weight", Operator: OpGt, Value: 200},
			{
				Logic: LogicAnd,
				Conditions: []Criteria{
					{Field: "numOfContainers", Operator: OpGt, Value: 500},
					{Field: "numOfPeople", Operator: OpLt, Value: 5},
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

	if !Match(Criteria{Field: "status", Operator: OpEq, Value: "open"}, data) {
		t.Fatal("eq failed")
	}
	if Match(Criteria{Field: "status", Operator: OpNeq, Value: "open"}, data) {
		t.Fatal("neq should fail")
	}
	if !Match(Criteria{Field: "count", Operator: OpGte, Value: 3}, data) {
		t.Fatal("gte failed")
	}
	if !Match(Criteria{Field: "count", Operator: OpIn, Value: []any{1, 3, 5}}, data) {
		t.Fatal("in failed")
	}
	if !Match(Criteria{Field: "nested.flag", Operator: OpEq, Value: true}, data) {
		t.Fatal("nested eq failed")
	}
	if Match(Criteria{Logic: LogicNot, Conditions: []Criteria{{Field: "status", Operator: OpEq, Value: "open"}}}, data) {
		t.Fatal("not should fail when child matches")
	}
}
