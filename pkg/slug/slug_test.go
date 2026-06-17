package slug

import "testing"

func TestMake(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"simple", "hello", "hello"},
		{"spaces", "hello world", "hello-world"},
		{"uppercase", "Hello World", "hello-world"},
		{"digits", "Route 66", "route-66"},
		{"leading and trailing spaces", "  hello world  ", "hello-world"},
		{"collapse separators", "hello   world", "hello-world"},
		{"mixed punctuation", "Hello, World!", "hello-world"},
		{"leading punctuation", "!!!hello", "hello"},
		{"trailing punctuation", "hello!!!", "hello"},
		{"only punctuation", "!!!", ""},
		{"only spaces", "     ", ""},
		{"underscores and slashes", "a_b/c", "a-b-c"},
		{"existing hyphens", "already-a-slug", "already-a-slug"},
		{"collapse hyphens and spaces", "a - b", "a-b"},
		{"accented characters", "Café au lait", "cafe-au-lait"},
		{"german eszett", "Straße", "strasse"},
		{"ampersand", "Rock & Roll", "rock-and-roll"},
		{"ligatures", "Æther œuvre", "aether-oeuvre"},
		{"thorn and eth", "Þor ð", "thor-d"},
		{"many diacritics", "àáâãäåèéêëìíîïòóôõöùúûüýñçž", "aaaaaaeeeeiiiiooooouuuuyncz"},
		{"polish and czech", "Łódź Žižek", "lodz-zizek"},
		{"non-latin dropped", "日本語", ""},
		{"non-latin with ascii", "hello 日本語 world", "hello-world"},
		{"newlines and tabs", "hello\n\tworld", "hello-world"},
		{"numbers only", "12345", "12345"},
		{"emoji", "fun 🎉 times", "fun-times"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slugify(tt.in); got != tt.want {
				t.Fatalf("Make(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestMake_OnlyValidBytes guarantees the output alphabet is restricted to
// a-z, 0-9, and '-' for a deliberately hostile input.
func TestMake_OnlyValidBytes(t *testing.T) {
	got := Slugify("A1!@#$ b2 Café — 日本 _-_ Z")
	for _, r := range got {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-'
		if !ok {
			t.Fatalf("Make produced invalid rune %q in %q", r, got)
		}
	}
	if got == "" {
		t.Fatal("expected non-empty slug")
	}
}
