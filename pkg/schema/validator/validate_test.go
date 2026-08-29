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

func TestValidateDefinition_foreignFormat(t *testing.T) {
	validID := "550e8400-e29b-41d4-a716-446655440000"
	tests := []struct {
		name    string
		def     string
		wantErr string
	}{
		{
			name: "valid",
			def:  `{"type":"object","properties":{"investorId":{"type":"string","format":"foreign","schemaId":"` + validID + `"}}}`,
		},
		{
			name:    "missing schemaId",
			def:     `{"type":"object","properties":{"investorId":{"type":"string","format":"foreign"}}}`,
			wantErr: "schemaId is required",
		},
		{
			name:    "invalid schemaId",
			def:     `{"type":"object","properties":{"investorId":{"type":"string","format":"foreign","schemaId":"not-a-uuid"}}}`,
			wantErr: "schemaId must be a uuid",
		},
		{
			name:    "non-string type",
			def:     `{"type":"object","properties":{"investorId":{"type":"integer","format":"foreign","schemaId":"` + validID + `"}}}`,
			wantErr: "format foreign requires type string",
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

func TestValidateDataForeignFormat(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"investorId": { "type": "string", "format": "foreign", "schemaId": "550e8400-e29b-41d4-a716-446655440000" }
		},
		"required": ["investorId"]
	}`)

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{name: "valid record id", data: `{"investorId":"550e8400-e29b-41d4-a716-446655440000"}`},
		{name: "not a uuid", data: `{"investorId":"not-a-record-id"}`, wantErr: true},
		{name: "empty string", data: `{"investorId":""}`, wantErr: true},
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

func TestForeignFields(t *testing.T) {
	def := json.RawMessage(`{
		"properties": {
			"legalName": { "type": "string" },
			"investorId": { "type": "string", "format": "foreign", "schemaId": "550e8400-e29b-41d4-a716-446655440000" },
			"proofFileId": { "type": "string", "format": "file" }
		}
	}`)
	fields, err := ForeignFields(def)
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].Name != "investorId" {
		t.Fatalf("got %#v", fields)
	}
	if fields[0].SchemaID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("schemaId=%s", fields[0].SchemaID)
	}
}

func TestDisplayTitle(t *testing.T) {
	def := json.RawMessage(`{
		"properties": {
			"status": { "type": "string", "enum": ["active"] },
			"investorId": { "type": "string", "format": "foreign", "schemaId": "550e8400-e29b-41d4-a716-446655440000" },
			"proofFileId": { "type": "string", "format": "file" },
			"legalName": { "type": "string" },
			"email": { "type": "string", "format": "email" }
		}
	}`)
	title := DisplayTitle(json.RawMessage(`{"status":"active","legalName":"Acme LP","email":"a@x.com"}`), def, "Investor")
	if title != "Acme LP" {
		t.Fatalf("title=%q", title)
	}
	if got := TitleKey(def); got != "legalName" {
		t.Fatalf("title key=%q", got)
	}
	if got := DisplayTitle(json.RawMessage(`{}`), def, "Investor"); got != "Investor" {
		t.Fatalf("fallback=%q", got)
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
