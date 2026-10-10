package validator

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRejectRenamedFieldKeys(t *testing.T) {
	previous := json.RawMessage(`{
		"type": "object",
		"properties": {
			"createdAt": {"type": "string", "title": "Created At"},
			"name": {"type": "string", "title": "Name"}
		}
	}`)

	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name: "title change",
			next: `{
				"type": "object",
				"properties": {
					"createdAt": {"type": "string", "title": "Check In Date"},
					"name": {"type": "string", "title": "Name"}
				}
			}`,
		},
		{
			name: "add key",
			next: `{
				"type": "object",
				"properties": {
					"createdAt": {"type": "string", "title": "Created At"},
					"name": {"type": "string", "title": "Name"},
					"status": {"type": "string", "title": "Status"}
				}
			}`,
		},
		{
			name: "remove key",
			next: `{
				"type": "object",
				"properties": {
					"name": {"type": "string", "title": "Name"}
				}
			}`,
		},
		{
			name: "rename key",
			next: `{
				"type": "object",
				"properties": {
					"checkInDate": {"type": "string", "title": "Check In Date"},
					"name": {"type": "string", "title": "Name"}
				}
			}`,
			wantErr: "field keys cannot be renamed",
		},
		{
			name: "add and remove",
			next: `{
				"type": "object",
				"properties": {
					"createdAt": {"type": "string", "title": "Created At"},
					"status": {"type": "string", "title": "Status"}
				}
			}`,
			wantErr: "field keys cannot be renamed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RejectRenamedFieldKeys(previous, json.RawMessage(tt.next))
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
