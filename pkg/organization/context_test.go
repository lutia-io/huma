package organization

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolve_bodyPreferredOverHeader(t *testing.T) {
	bodyID := "11111111-1111-4111-8111-111111111111"
	headerID := "22222222-2222-4222-8222-222222222222"
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(HeaderOrganizationID, headerID)

	oc, ok := Resolve(r, bodyID)
	if !ok {
		t.Fatal("expected ok")
	}
	if oc.OrganizationID != bodyID {
		t.Fatalf("got %s want %s", oc.OrganizationID, bodyID)
	}
}

func TestResolve_headerFallback(t *testing.T) {
	headerID := "22222222-2222-4222-8222-222222222222"
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(HeaderOrganizationID, headerID)

	oc, ok := Resolve(r, "")
	if !ok {
		t.Fatal("expected ok")
	}
	if oc.OrganizationID != headerID {
		t.Fatalf("got %s want %s", oc.OrganizationID, headerID)
	}
}

func TestContextRoundTrip(t *testing.T) {
	oc := Context{OrganizationID: "11111111-1111-4111-8111-111111111111"}
	ctx := WithContext(t.Context(), oc)
	got, ok := FromContext(ctx)
	if !ok || got.OrganizationID != oc.OrganizationID {
		t.Fatalf("round trip failed: ok=%v got=%+v", ok, got)
	}
}
