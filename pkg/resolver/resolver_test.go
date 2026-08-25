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
	}

	tests := []struct {
		name string
		data map[string]any
		want map[string]any
	}{
		{
			name: "passthrough without templates",
			data: map[string]any{"literal": "hello", "n": float64(7), "b": false, "nil": nil},
			want: map[string]any{"literal": "hello", "n": float64(7), "b": false, "nil": nil},
		},
		{
			name: "single field path keeps string type",
			data: map[string]any{"email": "{{ .Record.email }}"},
			want: map[string]any{"email": "jane@example.com"},
		},
		{
			name: "single field path keeps number type",
			data: map[string]any{"age": "{{ .Record.age }}"},
			want: map[string]any{"age": float64(42)},
		},
		{
			name: "single field path keeps bool type",
			data: map[string]any{"vip": "{{ .Record.vip }}"},
			want: map[string]any{"vip": true},
		},
		{
			name: "single field path keeps object and array types",
			data: map[string]any{"addr": "{{ .Record.address }}", "tags": "{{ .Record.tags }}"},
			want: map[string]any{"addr": map[string]any{"city": "Berlin"}, "tags": []any{"a", "b"}},
		},
		{
			name: "nested path lookup",
			data: map[string]any{"city": "{{ .Record.address.city }}"},
			want: map[string]any{"city": "Berlin"},
		},
		{
			name: "mixed text interpolates to string",
			data: map[string]any{"greeting": "Hello {{ .Record.email }}, age {{ .Record.age }}"},
			want: map[string]any{"greeting": "Hello jane@example.com, age 42"},
		},
		{
			name: "recurses into nested maps and slices",
			data: map[string]any{
				"outer": map[string]any{"email": "{{ .Record.email }}"},
				"list":  []any{"{{ .Record.vip }}", "plain"},
			},
			want: map[string]any{
				"outer": map[string]any{"email": "jane@example.com"},
				"list":  []any{true, "plain"},
			},
		},
		{
			name: "default with or for missing field",
			data: map[string]any{"name": `{{ or .Record.nickname "friend" }}`},
			want: map[string]any{"name": "friend"},
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
		"at":   "{{ now }}",
		"note": "created at {{ now }}",
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
			name: "unknown top-level field",
			data: map[string]any{"x": "{{ .Contxt.Record.email }}"},
		},
		{
			name: "unknown function",
			data: map[string]any{"x": "{{ frobnicate }}"},
		},
		{
			name: "malformed template",
			data: map[string]any{"x": "{{ .Record. }}"},
		},
		{
			name: "unclosed template",
			data: map[string]any{"x": "{{ .Record.email"},
		},
		{
			name: "error inside nested value",
			data: map[string]any{"outer": map[string]any{"x": "{{ frobnicate }}"}},
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
// the or function provides defaults.
func TestResolveMissingRecordField(t *testing.T) {
	got, err := Resolve(map[string]any{"x": "{{ .Record.missing }}"}, map[string]any{})
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

func TestResolveInputIndex(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want map[string]any
	}{
		{
			name: "dotted numeric index",
			data: map[string]any{"org": "{{ .Input.1.body.name }}"},
			want: map[string]any{"org": "Beta"},
		},
		{
			name: "mixed text interpolates numeric index",
			data: map[string]any{"url": "https://example.com/{{ .Input.1.body.name }}"},
			want: map[string]any{"url": "https://example.com/Beta"},
		},
		{
			name: "level 0 named field still works",
			data: map[string]any{"orgId": "{{ .Input.orgId }}"},
			want: map[string]any{"orgId": "acme"},
		},
	}

	named := map[string]any{"orgId": "acme"}
	indexed := map[string]any{
		"0": map[string]any{"status": float64(200), "body": map[string]any{"users": []any{map[string]any{"id": "u1"}}}},
		"1": map[string]any{"status": float64(200), "body": map[string]any{"name": "Beta"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := indexed
			if tt.name == "level 0 named field still works" {
				in = named
			}
			got, err := ResolveInput(tt.data, in)
			if err != nil {
				t.Fatalf("ResolveInput() error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ResolveInput() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
