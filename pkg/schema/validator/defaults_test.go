package validator

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lutia-io/huma/pkg/uuid"
)

func TestApplyDefaults(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"status": { "type": "string", "default": "draft" },
			"count": { "type": "integer", "default": 1 },
			"createdAt": { "type": "string", "format": "date-time", "default": "{{ now }}" },
			"externalId": { "type": "string", "default": "{{ uuid }}" }
		},
		"required": ["status", "createdAt", "externalId"]
	}`)

	got, err := ApplyDefaults(def, json.RawMessage(`{"name":"Ada"}`))
	if err != nil {
		t.Fatalf("ApplyDefaults: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["name"] != "Ada" {
		t.Errorf("name = %v", obj["name"])
	}
	if obj["status"] != "draft" {
		t.Errorf("status = %v", obj["status"])
	}
	if obj["count"] != float64(1) {
		t.Errorf("count = %#v", obj["count"])
	}
	createdAt, _ := obj["createdAt"].(string)
	if _, err := time.Parse(time.RFC3339, createdAt); err != nil {
		t.Errorf("createdAt = %v: %v", obj["createdAt"], err)
	}
	externalID, _ := obj["externalId"].(string)
	if !uuid.Valid(externalID) {
		t.Errorf("externalId = %v", obj["externalId"])
	}
}

func TestApplyDefaultsDoesNotOverwrite(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"status": { "type": "string", "default": "draft" }
		}
	}`)

	got, err := ApplyDefaults(def, json.RawMessage(`{"status":"active"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"status":"active"}` {
		t.Fatalf("got %s", got)
	}

	got, err = ApplyDefaults(def, json.RawMessage(`{"status":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"status":null}` {
		t.Fatalf("got %s", got)
	}
}

func TestApplyDefaultsThenValidate(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"status": { "type": "string", "enum": ["draft", "active"], "default": "draft" },
			"createdAt": { "type": "string", "format": "date-time", "default": "{{ now }}" },
			"externalId": { "type": "string", "format": "uuid", "default": "{{ uuid }}" }
		},
		"required": ["status", "createdAt", "externalId"],
		"additionalProperties": false
	}`)
	filled, err := ApplyDefaults(def, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateData(def, filled); err != nil {
		t.Fatalf("ValidateData: %v", err)
	}
	var obj map[string]string
	if err := json.Unmarshal(filled, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["status"] != "draft" {
		t.Errorf("status = %q", obj["status"])
	}
	if _, err := time.Parse(time.RFC3339, obj["createdAt"]); err != nil {
		t.Errorf("createdAt: %v", err)
	}
	if !uuid.Valid(obj["externalId"]) {
		t.Errorf("externalId %q is not a uuid", obj["externalId"])
	}
}

func TestValidateDefinition_defaultTemplate(t *testing.T) {
	tests := []struct {
		name    string
		def     string
		wantErr string
	}{
		{
			name: "static default",
			def:  `{"type":"object","properties":{"status":{"type":"string","default":"draft"}}}`,
		},
		{
			name: "now default",
			def:  `{"type":"object","properties":{"createdAt":{"type":"string","default":"{{ now }}"}}}`,
		},
		{
			name: "uuid default",
			def:  `{"type":"object","properties":{"externalId":{"type":"string","default":"{{ uuid }}"}}}`,
		},
		{
			name:    "unknown function",
			def:     `{"type":"object","properties":{"x":{"type":"string","default":"{{ frobnicate }}"}}}`,
			wantErr: "default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDefinition(json.RawMessage(tt.def))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want contain %q", err, tt.wantErr)
			}
		})
	}
}
