package authz

import (
	"encoding/json"
	"testing"

	"github.com/lutia-io/huma/pkg/principal"
)

func TestSnapshotCreatorAllowsNetworkActions(t *testing.T) {
	snap := &Snapshot{
		Principal: principal.Principal{Type: principal.TypeUser, ID: "creator"},
		Creator:   true,
		Member:    true,
	}
	if !snap.allowed(ActionDelete, ResourceNetwork, "", "") {
		t.Fatal("creator should delete network")
	}
	if !snap.allowed(ActionManageAccess, ResourceNetwork, "", "") {
		t.Fatal("creator should manage access")
	}
}

func TestSnapshotOrgUserCannotMutateDefinitions(t *testing.T) {
	snap := &Snapshot{
		Principal: principal.Principal{Type: principal.TypeOrganizationUser, ID: "ou"},
		Member:    true,
		Grants: []Grant{{
			Resource: ResourceSchema,
			Actions:  []string{ActionCreate, ActionRead, ActionUpdate, ActionDelete},
		}},
	}
	if snap.allowed(ActionCreate, ResourceSchema, "", "") {
		t.Fatal("org user must not create schemas")
	}
	if !snap.allowed(ActionRead, ResourceSchema, "", "") {
		t.Fatal("org user may read definitions when granted")
	}
}

func TestSnapshotNetworkMemberFullRecords(t *testing.T) {
	snap := &Snapshot{
		Principal: principal.Principal{Type: principal.TypeUser, ID: "collab"},
		Member:    true,
	}
	if !snap.allowed(ActionRead, ResourceRecord, "", "schema-1") {
		t.Fatal("network member should read records")
	}
	if !snap.allowed(ActionDelete, ResourceFile, "", "") {
		t.Fatal("network member should delete files")
	}
}

func TestGrantMatchesResourceID(t *testing.T) {
	g := Grant{Resource: ResourceSchema, Actions: []string{ActionUpdate}, ResourceID: "s1"}
	if g.matches(ResourceSchema, "s2", "") {
		t.Fatal("should not match other schema")
	}
	if !g.matches(ResourceSchema, "s1", "") {
		t.Fatal("should match resource id")
	}
	open := Grant{Resource: ResourceSchema, Actions: []string{ActionUpdate}}
	if !open.matches(ResourceSchema, "s1", "") {
		t.Fatal("empty resource id matches all")
	}
}

func TestRecordViewAndPatch(t *testing.T) {
	access := FieldAccess{Fields: map[string]string{"status": FieldWrite, "email": FieldRead}}
	view, err := RecordView(json.RawMessage(`{"status":"open","email":"a@b.c","ssn":"1"}`), access)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(view, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["ssn"]; ok {
		t.Fatal("ssn should be hidden")
	}
	if _, err := RecordPatch(json.RawMessage(`{"status":"open","email":"a@b.c","ssn":"1"}`), json.RawMessage(`{"email":"x"}`), access); err == nil {
		t.Fatal("expected reject of read-only field")
	}
	merged, err := RecordPatch(json.RawMessage(`{"status":"open","email":"a@b.c","ssn":"1"}`), json.RawMessage(`{"status":"closed"}`), access)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(merged, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["status"] != "closed" || obj["ssn"] != "1" {
		t.Fatalf("merge = %v", obj)
	}
}

func TestMemberFilterSQL(t *testing.T) {
	sql := MemberFilter("s.network_id", "$1")
	if sql == "" {
		t.Fatal("empty filter")
	}
}
