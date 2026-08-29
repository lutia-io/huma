package schema

import "testing"

func TestVisibleAsForeignTarget(t *testing.T) {
	networkID := "net-1"
	orgA := "org-a"
	orgB := "org-b"
	networkScoped := &schema{NetworkID: networkID}
	orgScoped := &schema{NetworkID: networkID, OrganizationID: &orgA}

	if !visibleAsForeignTarget(networkScoped, networkID, nil) {
		t.Fatal("network schema should be visible to a network schema")
	}
	if !visibleAsForeignTarget(networkScoped, networkID, &orgA) {
		t.Fatal("network schema should be visible to an org schema")
	}
	if visibleAsForeignTarget(orgScoped, networkID, nil) {
		t.Fatal("org schema should not be visible to a network schema")
	}
	if !visibleAsForeignTarget(orgScoped, networkID, &orgA) {
		t.Fatal("org schema should be visible to the same org")
	}
	if visibleAsForeignTarget(orgScoped, networkID, &orgB) {
		t.Fatal("org schema should not be visible to another org")
	}
	if visibleAsForeignTarget(networkScoped, "other", nil) {
		t.Fatal("schema from another network should not be visible")
	}
}
