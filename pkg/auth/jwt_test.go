package auth

import (
	"testing"
	"time"
)

func TestSignAndParseAccessToken(t *testing.T) {
	secret := []byte("test-secret")
	now := time.Now().UTC()
	claims := accessClaims{
		Issuer:        defaultIssuer,
		Audience:      defaultAudience,
		Subject:       "11111111-1111-4111-8111-111111111111",
		PrincipalType: "user",
		NetworkID:     "22222222-2222-4222-8222-222222222222",
		JWTID:         "33333333-3333-4333-8333-333333333333",
		IssuedAt:      now.Unix(),
		ExpiresAt:     now.Add(time.Minute).Unix(),
	}
	token, err := signAccessToken(secret, claims)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	got, err := parseAccessToken(secret, token, now, defaultIssuer, defaultAudience)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Subject != claims.Subject || got.NetworkID != claims.NetworkID {
		t.Fatalf("claims mismatch: %+v", got)
	}
}

func TestParseAccessToken_expired(t *testing.T) {
	secret := []byte("test-secret")
	now := time.Now().UTC()
	token, err := signAccessToken(secret, accessClaims{
		Issuer:        defaultIssuer,
		Audience:      defaultAudience,
		Subject:       "11111111-1111-4111-8111-111111111111",
		PrincipalType: "user",
		NetworkID:     "22222222-2222-4222-8222-222222222222",
		JWTID:         "33333333-3333-4333-8333-333333333333",
		IssuedAt:      now.Add(-2 * time.Minute).Unix(),
		ExpiresAt:     now.Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := parseAccessToken(secret, token, now, defaultIssuer, defaultAudience); err == nil {
		t.Fatal("expected expiry error")
	}
}

func TestLoginRateLimiter(t *testing.T) {
	l := newLoginRateLimiter(2, time.Minute)
	if !l.Allow("a") || !l.Allow("a") {
		t.Fatal("first two should allow")
	}
	if l.Allow("a") {
		t.Fatal("third should deny")
	}
	if !l.Allow("b") {
		t.Fatal("other key should allow")
	}
}
