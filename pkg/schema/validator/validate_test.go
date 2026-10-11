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

func TestValidateDataTimeFormat(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"opensAt": { "type": "string", "format": "time" }
		},
		"additionalProperties": false
	}`)

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{name: "utc", data: `{"opensAt":"14:30:00Z"}`},
		{name: "offset", data: `{"opensAt":"09:15:00-07:00"}`},
		{name: "missing seconds", data: `{"opensAt":"14:30"}`, wantErr: true},
		{name: "missing offset", data: `{"opensAt":"14:30:00"}`, wantErr: true},
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

func TestPropertiesDocumentOrder(t *testing.T) {
	def := json.RawMessage(`{
		"properties": {
			"legalName": { "type": "string" },
			"email": { "type": "string" },
			"investorId": { "type": "string", "format": "foreign", "schemaId": "550e8400-e29b-41d4-a716-446655440000" }
		}
	}`)
	properties, err := Properties(def)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(properties))
	for i, property := range properties {
		got[i] = property.Name
	}
	want := []string{"legalName", "email", "investorId"}
	if len(got) != len(want) {
		t.Fatalf("properties=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("properties=%v want %v", got, want)
		}
	}
}

func TestDisplayTitle(t *testing.T) {
	def := json.RawMessage(`{
		"properties": {
			"status": { "type": "string", "enum": ["active"] },
			"investorId": { "type": "string", "format": "foreign", "schemaId": "550e8400-e29b-41d4-a716-446655440000" },
			"proofFileId": { "type": "string", "format": "file" },
			"owner": { "type": "string", "format": "user" },
			"legalName": { "type": "string" },
			"email": { "type": "string", "format": "email" }
		}
	}`)
	title := DisplayTitle(json.RawMessage(`{"status":"active","owner":"550e8400-e29b-41d4-a716-446655440000","legalName":"Acme LP","email":"a@x.com"}`), def, "Investor")
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

func TestValidateDefinition_addressFormat(t *testing.T) {
	tests := []struct {
		name    string
		def     string
		wantErr string
	}{
		{
			name: "valid",
			def:  `{"type":"object","properties":{"mailingAddress":{"type":"object","format":"address"}}}`,
		},
		{
			name:    "non-object type",
			def:     `{"type":"object","properties":{"mailingAddress":{"type":"string","format":"address"}}}`,
			wantErr: "format address requires type object",
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

func TestValidateDataAddressFormat(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"mailingAddress": {
				"type": "object",
				"format": "address",
				"additionalProperties": false,
				"properties": {
					"line1": { "type": "string" },
					"line2": { "type": "string" },
					"city": { "type": "string" },
					"region": { "type": "string" },
					"postalCode": { "type": "string" },
					"country": { "type": "string" }
				},
				"required": ["line1", "city", "country"]
			}
		},
		"required": ["mailingAddress"]
	}`)

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name: "valid address",
			data: `{"mailingAddress":{"line1":"1 Market St","city":"San Francisco","region":"CA","postalCode":"94105","country":"US"}}`,
		},
		{
			name:    "missing required line1",
			data:    `{"mailingAddress":{"city":"San Francisco","country":"US"}}`,
			wantErr: true,
		},
		{
			name:    "string instead of object",
			data:    `{"mailingAddress":"1 Market St"}`,
			wantErr: true,
		},
		{
			name:    "unknown address field",
			data:    `{"mailingAddress":{"line1":"1 Market St","city":"San Francisco","country":"US","lat":1}}`,
			wantErr: true,
		},
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

func TestTitleKeySkipsAddress(t *testing.T) {
	def := json.RawMessage(`{
		"properties": {
			"mailingAddress": { "type": "object", "format": "address" },
			"legalName": { "type": "string" }
		}
	}`)
	if got := TitleKey(def); got != "legalName" {
		t.Fatalf("title key=%q", got)
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

func TestValidateDefinition_fileFormat(t *testing.T) {
	tests := []struct {
		name    string
		def     string
		wantErr string
	}{
		{
			name: "string",
			def:  `{"type":"object","properties":{"attachment":{"type":"string","format":"file"}}}`,
		},
		{
			name: "array",
			def:  `{"type":"object","properties":{"attachments":{"type":"array","format":"file","items":{"type":"string","format":"file"}}}}`,
		},
		{
			name:    "non-string non-array type",
			def:     `{"type":"object","properties":{"attachment":{"type":"integer","format":"file"}}}`,
			wantErr: "format file requires type string or array",
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

func TestValidateDataFileFormatArray(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"attachments": {
				"type": "array",
				"format": "file",
				"items": { "type": "string", "format": "file" }
			}
		},
		"required": ["attachments"]
	}`)

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{name: "valid file ids", data: `{"attachments":["550e8400-e29b-41d4-a716-446655440000","6ba7b810-9dad-11d1-80b4-00c04fd430c8"]}`},
		{name: "empty array", data: `{"attachments":[]}`},
		{name: "not a uuid", data: `{"attachments":["not-a-file-id"]}`, wantErr: true},
		{name: "string instead of array", data: `{"attachments":"550e8400-e29b-41d4-a716-446655440000"}`, wantErr: true},
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

func TestValidateDataUserFormat(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"owner": { "type": "string", "format": "user" }
		},
		"required": ["owner"]
	}`)

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{name: "valid organization user id", data: `{"owner":"550e8400-e29b-41d4-a716-446655440000"}`},
		{name: "not a uuid", data: `{"owner":"not-a-user-id"}`, wantErr: true},
		{name: "empty string", data: `{"owner":""}`, wantErr: true},
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

func TestValidateDataUserList(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"participants": { "type": "array", "items": { "type": "string", "format": "user" } }
		}
	}`)

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{name: "user ids", data: `{"participants":["550e8400-e29b-41d4-a716-446655440000","6ba7b810-9dad-11d1-80b4-00c04fd430c8"]}`},
		{name: "empty array", data: `{"participants":[]}`},
		{name: "not a uuid", data: `{"participants":["not-a-user-id"]}`, wantErr: true},
		{name: "string instead of array", data: `{"participants":"550e8400-e29b-41d4-a716-446655440000"}`, wantErr: true},
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

func TestValidateDefinition_userFormat(t *testing.T) {
	tests := []struct {
		name    string
		def     string
		wantErr string
	}{
		{
			name: "string",
			def:  `{"type":"object","properties":{"owner":{"type":"string","format":"user"}}}`,
		},
		{
			name:    "array",
			def:     `{"type":"object","properties":{"owners":{"type":"array","format":"user","items":{"type":"string","format":"user"}}}}`,
			wantErr: "format user requires type string",
		},
		{
			name: "list of users",
			def:  `{"type":"object","properties":{"participants":{"type":"array","items":{"type":"string","format":"user"}}}}`,
		},
		{
			name:    "user items on a string",
			def:     `{"type":"object","properties":{"owner":{"type":"string","items":{"type":"string","format":"user"}}}}`,
			wantErr: "format user items require type array",
		},
		{
			name:    "user items are not strings",
			def:     `{"type":"object","properties":{"participants":{"type":"array","items":{"type":"number","format":"user"}}}}`,
			wantErr: "format user items require type string",
		},
		{
			name:    "non-string type",
			def:     `{"type":"object","properties":{"owner":{"type":"integer","format":"user"}}}`,
			wantErr: "format user requires type string",
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

func TestValidateDataPhoneFormat(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"phone": { "type": "string", "format": "phone" }
		},
		"required": ["phone"]
	}`)

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{name: "e164", data: `{"phone":"+14155552671"}`},
		{name: "formatted", data: `{"phone":"(415) 555-2671"}`},
		{name: "dashed", data: `{"phone":"415-555-2671"}`},
		{name: "international", data: `{"phone":"+44 20 7946 0958"}`},
		{name: "empty", data: `{"phone":""}`, wantErr: true},
		{name: "too short", data: `{"phone":"123"}`, wantErr: true},
		{name: "letters", data: `{"phone":"call-me"}`, wantErr: true},
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

func TestValidateDefinition_phoneFormat(t *testing.T) {
	tests := []struct {
		name    string
		def     string
		wantErr string
	}{
		{
			name: "string",
			def:  `{"type":"object","properties":{"phone":{"type":"string","format":"phone"}}}`,
		},
		{
			name:    "non-string type",
			def:     `{"type":"object","properties":{"phone":{"type":"integer","format":"phone"}}}`,
			wantErr: "format phone requires type string",
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

func TestValidateDataCurrencyFormat(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"properties": {
			"amount": { "type": "number", "format": "currency" }
		},
		"required": ["amount"]
	}`)

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{name: "dollars and cents", data: `{"amount":12.5}`},
		{name: "whole dollars", data: `{"amount":10}`},
		{name: "zero", data: `{"amount":0}`},
		{name: "negative", data: `{"amount":-4.25}`},
		{name: "string", data: `{"amount":"12.50"}`, wantErr: true},
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

func TestValidateDefinition_currencyFormat(t *testing.T) {
	tests := []struct {
		name    string
		def     string
		wantErr string
	}{
		{
			name: "number",
			def:  `{"type":"object","properties":{"amount":{"type":"number","format":"currency"}}}`,
		},
		{
			name: "integer rewritten to number",
			def:  `{"type":"object","properties":{"amount":{"type":"integer","format":"currency"}}}`,
		},
		{
			name:    "string type",
			def:     `{"type":"object","properties":{"amount":{"type":"string","format":"currency"}}}`,
			wantErr: "format currency requires type number",
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

func TestTitleKeySkipsCurrency(t *testing.T) {
	def := json.RawMessage(`{
		"properties": {
			"amount": { "type": "number", "format": "currency" },
			"legalName": { "type": "string" }
		}
	}`)
	if got := TitleKey(def); got != "legalName" {
		t.Fatalf("title key=%q", got)
	}
}

func TestTitleKeySkipsPhone(t *testing.T) {
	def := json.RawMessage(`{
		"properties": {
			"phone": { "type": "string", "format": "phone" },
			"legalName": { "type": "string" }
		}
	}`)
	if got := TitleKey(def); got != "legalName" {
		t.Fatalf("title key=%q", got)
	}
}

func TestValidateDefinition_emptyTableAndColumnTitle(t *testing.T) {
	def := json.RawMessage(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"title": "Orders",
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"status": { "type": "string", "title": "Order status" }
		},
		"required": []
	}`)
	if err := ValidateDefinition(def); err != nil {
		t.Fatal(err)
	}
	properties, err := Properties(def)
	if err != nil {
		t.Fatal(err)
	}
	if len(properties) != 1 || properties[0].Name != "status" {
		t.Fatalf("properties = %+v", properties)
	}
}

func TestValidateDefinition_duplicateFieldNames(t *testing.T) {
	tests := []struct {
		name    string
		def     string
		wantErr string
	}{
		{
			name: "distinct titles",
			def:  `{"type":"object","properties":{"status":{"type":"string","title":"Status"},"state":{"type":"string","title":"State"}}}`,
		},
		{
			name: "untitled keys stay distinct",
			def:  `{"type":"object","properties":{"status":{"type":"string"},"notes":{"type":"string"}}}`,
		},
		{
			name: "humanized key differs from another title",
			def:  `{"type":"object","properties":{"userId":{"type":"string"},"user":{"type":"string","title":"User"}}}`,
		},
		{
			name: "nested titles are ignored",
			def:  `{"type":"object","properties":{"office":{"type":"object","format":"address","properties":{"city":{"type":"string","title":"City"},"town":{"type":"string","title":"City"}}},"name":{"type":"string","title":"Name"}}}`,
		},
		{
			name:    "same title ignoring case",
			def:     `{"type":"object","properties":{"status":{"type":"string","title":"Status"},"state":{"type":"string","title":"status"}}}`,
			wantErr: "a field named status already exists",
		},
		{
			name:    "same title ignoring surrounding space",
			def:     `{"type":"object","properties":{"status":{"type":"string","title":"Status"},"state":{"type":"string","title":" Status "}}}`,
			wantErr: "a field named Status already exists",
		},
		{
			name:    "title matches another field's key label",
			def:     `{"type":"object","properties":{"poNumber":{"type":"string"},"code":{"type":"string","title":"Po Number"}}}`,
			wantErr: "a field named Po Number already exists",
		},
		{
			name:    "title matches humanized id key",
			def:     `{"type":"object","properties":{"userId":{"type":"string"},"label":{"type":"string","title":"User ID"}}}`,
			wantErr: "a field named User ID already exists",
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

func TestPrepareUpdateRetainsRemovedColumn(t *testing.T) {
	def := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": { "name": { "type": "string" } },
		"required": ["name"]
	}`)
	previous := json.RawMessage(`{"name":"Ada","note":"kept"}`)
	next := json.RawMessage(`{"name":"Ada Lovelace","note":"kept"}`)

	got, err := PrepareUpdate(def, previous, next)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(next) {
		t.Fatalf("got %s", got)
	}

	if _, err := PrepareUpdate(def, previous, json.RawMessage(`{"name":"Ada","extra":1}`)); err == nil {
		t.Fatal("expected new unknown key to be rejected")
	}
	if _, err := PrepareUpdate(def, previous, json.RawMessage(`{"note":"kept"}`)); err == nil {
		t.Fatal("expected missing required name to fail")
	}
}

func TestValidateData_integerIsNumber(t *testing.T) {
	def := json.RawMessage(`{"type":"object","properties":{"count":{"type":"integer"},"tags":{"type":"array","items":{"type":"integer"}}}}`)
	if err := ValidateData(def, json.RawMessage(`{"count":1.5,"tags":[1,2.5]}`)); err != nil {
		t.Fatal(err)
	}
}
