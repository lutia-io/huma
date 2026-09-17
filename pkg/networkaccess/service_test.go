package networkaccess

import "testing"

func TestValidateGrants(t *testing.T) {
	if err := validateGrants(nil); err == nil {
		t.Fatal("expected error")
	}
	if err := validateGrants([]grant{{Resource: "schema", Actions: []string{"create"}}}); err != nil {
		t.Fatal(err)
	}
	if err := validateGrants([]grant{{Resource: "record", Actions: []string{"create"}}}); err == nil {
		t.Fatal("record is not a network resource")
	}
}
