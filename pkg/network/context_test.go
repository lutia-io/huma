package network

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolve_bodyPreferredOverHeader(t *testing.T) {
	bodyID := "11111111-1111-4111-8111-111111111111"
	headerID := "22222222-2222-4222-8222-222222222222"
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(HeaderNetworkID, headerID)

	nc, ok := Resolve(r, bodyID)
	if !ok {
		t.Fatal("expected ok")
	}
	if nc.NetworkID != bodyID {
		t.Fatalf("got %s want %s", nc.NetworkID, bodyID)
	}
}

func TestResolve_headerFallback(t *testing.T) {
	headerID := "22222222-2222-4222-8222-222222222222"
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(HeaderNetworkID, headerID)

	nc, ok := Resolve(r, "")
	if !ok {
		t.Fatal("expected ok")
	}
	if nc.NetworkID != headerID {
		t.Fatalf("got %s want %s", nc.NetworkID, headerID)
	}
}

func TestResolve_invalidUUID(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(HeaderNetworkID, "not-a-uuid")
	if _, ok := Resolve(r, ""); ok {
		t.Fatal("expected not ok")
	}
}

func TestContextRoundTrip(t *testing.T) {
	nc := Context{NetworkID: "11111111-1111-4111-8111-111111111111"}
	ctx := WithContext(t.Context(), nc)
	got, ok := FromContext(ctx)
	if !ok || got.NetworkID != nc.NetworkID {
		t.Fatalf("round trip failed: ok=%v got=%+v", ok, got)
	}
}
