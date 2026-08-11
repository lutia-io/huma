package validator

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateDefinition(t *testing.T) {
	tests := []struct {
		name    string
		def     string
		wantErr string
	}{
		{
			name: "valid",
			def:  `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
		},
		{
			name:    "empty",
			def:     ``,
			wantErr: "definition is required",
		},
		{
			name:    "invalid type",
			def:     `{"type":"nope"}`,
			wantErr: "invalid definition",
		},
		{
			name:    "external ref",
			def:     `{"$ref":"https://example.com/schema.json"}`,
			wantErr: "external schema references are not allowed",
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

func TestValidateData(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": { "type": "string" },
			"email": { "type": "string", "format": "email" }
		},
		"required": ["name"],
		"additionalProperties": false
	}`)

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{name: "valid", data: `{"name":"Ada","email":"ada@example.com"}`},
		{name: "missing required", data: `{}`, wantErr: true},
		{name: "wrong type", data: `{"name":1}`, wantErr: true},
		{name: "bad format", data: `{"name":"Ada","email":"nope"}`, wantErr: true},
		{name: "extra property", data: `{"name":"Ada","x":1}`, wantErr: true},
		{name: "empty data", data: ``, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateData(def, json.RawMessage(tt.data))
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateDataFileFormat(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"attachment": { "type": "string", "format": "file" }
		},
		"required": ["attachment"]
	}`)

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{name: "valid file id", data: `{"attachment":"550e8400-e29b-41d4-a716-446655440000"}`},
		{name: "not a uuid", data: `{"attachment":"not-a-file-id"}`, wantErr: true},
		{name: "empty string", data: `{"attachment":""}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateData(def, json.RawMessage(tt.data))
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
