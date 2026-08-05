package resolver

import (
	"reflect"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	record := map[string]any{
		"email": "jane@example.com",
		"age":   float64(42),
		"vip":   true,
		"tags":  []any{"a", "b"},
		"address": map[string]any{
			"city": "Berlin",
		},
		"items": []any{
			map[string]any{"price": float64(10)},
			map[string]any{"price": float64(5.5)},
		},
	}

	tests := []struct {
		name string
		data map[string]any
		want map[string]any
	}{
		{
			name: "passthrough without expressions",
			data: map[string]any{"literal": "hello", "n": float64(7), "b": false, "nil": nil},
			want: map[string]any{"literal": "hello", "n": float64(7), "b": false, "nil": nil},
		},
		{
			name: "single expression keeps string type",
			data: map[string]any{"email": "{{ context.record.email }}"},
			want: map[string]any{"email": "jane@example.com"},
		},
		{
			name: "single expression keeps number type",
			data: map[string]any{"age": "{{ context.record.age }}"},
			want: map[string]any{"age": float64(42)},
		},
		{
			name: "single expression keeps bool type",
			data: map[string]any{"vip": "{{ context.record.vip }}"},
			want: map[string]any{"vip": true},
		},
		{
			name: "single expression keeps object and array types",
			data: map[string]any{"addr": "{{ context.record.address }}", "tags": "{{ context.record.tags }}"},
			want: map[string]any{"addr": map[string]any{"city": "Berlin"}, "tags": []any{"a", "b"}},
		},
		{
			name: "nested path lookup",
			data: map[string]any{"city": "{{ context.record.address.city }}"},
			want: map[string]any{"city": "Berlin"},
		},
		{
			name: "mixed text interpolates to string",
			data: map[string]any{"greeting": "Hello {{ context.record.email }}, age {{ context.record.age }}"},
			want: map[string]any{"greeting": "Hello jane@example.com, age 42"},
		},
		{
			name: "recurses into nested maps and slices",
			data: map[string]any{
				"outer": map[string]any{"email": "{{ context.record.email }}"},
				"list":  []any{"{{ context.record.vip }}", "plain"},
			},
			want: map[string]any{
				"outer": map[string]any{"email": "jane@example.com"},
				"list":  []any{true, "plain"},
			},
		},
		{
			name: "math formula with arguments",
			data: map[string]any{"sum": "{{ math.add(context.record.age, 8) }}"},
			want: map[string]any{"sum": float64(50)},
		},
		{
			name: "default operator for missing field",
			data: map[string]any{"name": "{{ context.record.nickname ?? \"friend\" }}"},
			want: map[string]any{"name": "friend"},
		},
		{
			name: "jsonata path lookup",
			data: map[string]any{"city": `{{ jsonata.eval("address.city") }}`},
			want: map[string]any{"city": "Berlin"},
		},
		{
			name: "jsonata aggregate",
			data: map[string]any{"total": `{{ jsonata.eval("$sum(items.price)") }}`},
			want: map[string]any{"total": float64(15.5)},
		},
		{
			name: "jsonata object construction",
			data: map[string]any{"payload": `{{ jsonata.eval("{ \"email\": email, \"city\": address.city }") }}`},
			want: map[string]any{"payload": map[string]any{"email": "jane@example.com", "city": "Berlin"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.data, record)
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Resolve() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestResolveDateNow(t *testing.T) {
	fixed := time.Date(2026, 8, 4, 12, 30, 0, 0, time.UTC)
	orig := nowFunc
	nowFunc = func() time.Time { return fixed }
	defer func() { nowFunc = orig }()

	got, err := Resolve(map[string]any{
		"at":   "{{ date.now() }}",
		"note": "created at {{ date.now() }}",
	}, nil)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["at"] != "2026-08-04T12:30:00Z" {
		t.Errorf("at = %v, want 2026-08-04T12:30:00Z", got["at"])
	}
	if got["note"] != "created at 2026-08-04T12:30:00Z" {
		t.Errorf("note = %v", got["note"])
	}
}

func TestResolveErrors(t *testing.T) {
	record := map[string]any{"email": "jane@example.com"}

	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "unknown top-level identifier",
			data: map[string]any{"x": "{{ contxt.record.email }}"},
		},
		{
			name: "unknown formula",
			data: map[string]any{"x": "{{ date.frobnicate() }}"},
		},
		{
			name: "malformed expression",
			data: map[string]any{"x": "{{ context.record. }}"},
		},
		{
			name: "unclosed expression",
			data: map[string]any{"x": "{{ context.record.email"},
		},
		{
			name: "error inside nested value",
			data: map[string]any{"outer": map[string]any{"x": "{{ nope }}"}},
		},
		{
			name: "invalid jsonata expression",
			data: map[string]any{"x": `{{ jsonata.eval("$sum(") }}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Resolve(tt.data, record); err == nil {
				t.Fatal("Resolve() expected error, got nil")
			}
		})
	}
}

// Missing record fields resolve to nil (JSON null) rather than erroring;
// schema validation downstream rejects them where they are not allowed, and
// the ?? operator provides defaults.
func TestResolveMissingRecordField(t *testing.T) {
	got, err := Resolve(map[string]any{"x": "{{ context.record.missing }}"}, map[string]any{})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["x"] != nil {
		t.Errorf("x = %#v, want nil", got["x"])
	}
}

func TestResolveNilRecord(t *testing.T) {
	got, err := Resolve(map[string]any{"literal": "hello"}, nil)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["literal"] != "hello" {
		t.Errorf("literal = %v, want hello", got["literal"])
	}
}
