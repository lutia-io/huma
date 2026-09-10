package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

type staticSystemUsers struct {
	id string
}

func (s staticSystemUsers) SystemUserID(context.Context, string, string) (string, error) {
	return s.id, nil
}

func TestCreateRecord_usesSystemUser(t *testing.T) {
	records := &fakeRecords{id: "rec-1"}
	h := NewCreateRecord(records, staticSystemUsers{id: "sys-1"})

	out, err := h.Execute(context.Background(), executor.ExecutionContext{
		NetworkID:          "net-1",
		OrganizationID:     "org-1",
		OrganizationUserID: "ou-1",
		IdempotencyKey:     "wf:0",
	}, action.Action{
		Type: action.TypeCreateRecord,
		Context: action.CreateRecordContext{
			SchemaID: "schema-1",
			Data:     map[string]any{"name": "Ada"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got["id"] != "rec-1" {
		t.Fatalf("got %#v", got)
	}
	if records.created.OrganizationUserID != "sys-1" {
		t.Fatalf("organization user id = %s, want sys-1", records.created.OrganizationUserID)
	}
	if records.created.OrganizationID != "org-1" || records.created.NetworkID != "net-1" {
		t.Fatalf("created scope %#v", records.created)
	}
}
