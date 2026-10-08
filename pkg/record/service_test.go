package record

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/schema"
)

type stubSchema struct {
	definition json.RawMessage
}

func (s stubSchema) Definition(context.Context, string) (json.RawMessage, error) {
	return s.definition, nil
}

func (s stubSchema) SnapshotByID(context.Context, string) (*schema.Snapshot, error) {
	return nil, nil
}

func (s stubSchema) ValidateRecordData(context.Context, string, json.RawMessage) error {
	return nil
}

type stubStore struct {
	users []organizationUserRef
}

func (s stubStore) Insert(context.Context, *Record) (string, error) { return "", nil }

func (s stubStore) Get(context.Context, string) (*Record, bool, error) {
	return nil, false, nil
}

func (s stubStore) GetByID(context.Context, string) (*Record, error) { return nil, nil }

func (s stubStore) GetByIDs(context.Context, []string) ([]*Record, error) { return nil, nil }

func (s stubStore) GetOrganizationUsersByIDs(context.Context, []string) ([]organizationUserRef, error) {
	return s.users, nil
}

func (s stubStore) List(context.Context, listParams) (*listResult, error) { return nil, nil }

func (s stubStore) ListBySchema(context.Context, string, string, string, int) ([]*Record, error) {
	return nil, nil
}

func (s stubStore) UpdateData(context.Context, string, json.RawMessage) (bool, error) {
	return false, nil
}

func TestValidateUserRefs(t *testing.T) {
	const (
		schemaID       = "11111111-1111-1111-1111-111111111111"
		networkID      = "22222222-2222-2222-2222-222222222222"
		organizationID = "33333333-3333-3333-3333-333333333333"
		userID         = "550e8400-e29b-41d4-a716-446655440000"
	)
	definition := json.RawMessage(`{"properties":{"owner":{"type":"string","format":"user"}}}`)
	data := json.RawMessage(`{"owner":"` + userID + `"}`)

	tests := []struct {
		name    string
		users   []organizationUserRef
		wantErr string
	}{
		{
			name: "same organization",
			users: []organizationUserRef{{
				ID:             userID,
				OrganizationID: organizationID,
				NetworkID:      networkID,
			}},
		},
		{
			name:    "missing",
			wantErr: "existing organization user",
		},
		{
			name: "other organization",
			users: []organizationUserRef{{
				ID:             userID,
				OrganizationID: "44444444-4444-4444-4444-444444444444",
				NetworkID:      networkID,
			}},
			wantErr: "in this organization",
		},
		{
			name: "internal",
			users: []organizationUserRef{{
				ID:             userID,
				OrganizationID: organizationID,
				NetworkID:      networkID,
				Internal:       true,
			}},
			wantErr: "in this organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{
				logger:        logger.New(),
				store:         stubStore{users: tt.users},
				schemaService: stubSchema{definition: definition},
			}
			err := svc.validateUserRefs(context.Background(), schemaID, networkID, organizationID, data)
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
