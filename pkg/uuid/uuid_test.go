package uuid

import (
	"errors"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	id, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !Valid(id) {
		t.Fatalf("New returned invalid uuid %q", id)
	}
	if id[14] != '4' {
		t.Fatalf("version nibble: got %c, want 4", id[14])
	}
	switch id[19] {
	case '8', '9', 'a', 'b':
	default:
		t.Fatalf("variant nibble: got %c, want 8/9/a/b", id[19])
	}
}

func TestNewUnique(t *testing.T) {
	a, err := New()
	if err != nil {
		t.Fatal(err)
	}
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("expected distinct uuids, both %q", a)
	}
}

func TestMustNew(t *testing.T) {
	id := MustNew()
	if !Valid(id) {
		t.Fatalf("MustNew returned invalid uuid %q", id)
	}
}

func TestValid(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"canonical lower", "550e8400-e29b-41d4-a716-446655440000", true},
		{"canonical upper", "550E8400-E29B-41D4-A716-446655440000", true},
		{"empty", "", false},
		{"no hyphens", "550e8400e29b41d4a716446655440000", false},
		{"too short", "550e8400-e29b-41d4-a716", false},
		{"bad char", "550e8400-e29b-41d4-a716-44665544000g", false},
		{"not a uuid", "not-a-file-id", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Valid(tt.in); got != tt.want {
				t.Fatalf("Valid(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	const id = "550e8400-e29b-41d4-a716-446655440000"
	got, err := Parse(id)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got != id {
		t.Fatalf("got %q, want %q", got, id)
	}

	_, err = Parse("nope")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
}

func TestFormatLowercase(t *testing.T) {
	id, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if id != strings.ToLower(id) {
		t.Fatalf("expected lowercase hex, got %q", id)
	}
}
