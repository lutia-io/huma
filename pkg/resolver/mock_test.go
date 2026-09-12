package resolver

import (
	"net/mail"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestResolveMockFunctions(t *testing.T) {
	got, err := Resolve(map[string]any{
		"text":     "{{ mockText }}",
		"number":   "{{ mockNumber }}",
		"integer":  "{{ mockInteger }}",
		"boolean":  "{{ mockBoolean }}",
		"date":     "{{ mockDate }}",
		"datetime": "{{ mockDateTime }}",
		"email":    "{{ mockEmail }}",
		"url":      "{{ mockURL }}",
		"phone":    "{{ mockPhone }}",
		"choice":   `{{ mockChoice "draft" "active" }}`,
		"note":     "hi {{ mockText }}",
	}, Trigger{})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	text, _ := got["text"].(string)
	if text == "" || text[0] < 'A' || text[0] > 'Z' {
		t.Errorf("mockText = %#v, want a title-cased phrase", got["text"])
	}

	switch got["number"].(type) {
	case float64:
	default:
		t.Errorf("mockNumber = %#v, want float64", got["number"])
	}

	switch got["integer"].(type) {
	case int64:
	default:
		t.Errorf("mockInteger = %#v, want int64", got["integer"])
	}

	switch got["boolean"].(type) {
	case bool:
	default:
		t.Errorf("mockBoolean = %#v, want bool", got["boolean"])
	}

	date, _ := got["date"].(string)
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		t.Errorf("mockDate = %#v: %v", got["date"], err)
	}

	datetime, _ := got["datetime"].(string)
	if _, err := time.Parse(time.RFC3339, datetime); err != nil {
		t.Errorf("mockDateTime = %#v: %v", got["datetime"], err)
	}

	email, _ := got["email"].(string)
	if _, err := mail.ParseAddress(email); err != nil {
		t.Errorf("mockEmail = %#v: %v", got["email"], err)
	}

	rawURL, _ := got["url"].(string)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		t.Errorf("mockURL = %#v", got["url"])
	}

	phone, _ := got["phone"].(string)
	if !strings.HasPrefix(phone, "+1") || len(phone) != 12 {
		t.Errorf("mockPhone = %#v", got["phone"])
	}

	choice, _ := got["choice"].(string)
	if choice != "draft" && choice != "active" {
		t.Errorf("mockChoice = %#v, want draft or active", got["choice"])
	}

	note, _ := got["note"].(string)
	if note == "hi {{ mockText }}" || note == "" {
		t.Errorf("mixed mockText = %#v", got["note"])
	}
}

func TestResolveMockChoiceTyped(t *testing.T) {
	got, err := Resolve(map[string]any{
		"n": "{{ mockChoice 1 2 }}",
	}, Trigger{})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	switch got["n"].(type) {
	case int64:
	default:
		t.Errorf("mockChoice number = %#v, want int64", got["n"])
	}
}

func TestResolveMockErrors(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{name: "mockText with argument", data: map[string]any{"x": "{{ mockText 1 }}"}},
		{name: "mockChoice with no arguments", data: map[string]any{"x": "{{ mockChoice }}"}},
		{name: "mockInteger with argument", data: map[string]any{"x": "{{ mockInteger 3 }}"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(tt.data, Trigger{})
			if err == nil {
				t.Fatal("Resolve() expected error, got nil")
			}
		})
	}
}
