package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type accessClaims struct {
	Issuer         string `json:"iss"`
	Audience       string `json:"aud"`
	Subject        string `json:"sub"`
	PrincipalType  string `json:"pty"`
	NetworkID      string `json:"nid"`
	OrganizationID string `json:"oid,omitempty"`
	JWTID          string `json:"jti"`
	IssuedAt       int64  `json:"iat"`
	ExpiresAt      int64  `json:"exp"`
}

func signAccessToken(secret []byte, claims accessClaims) (string, error) {
	headerJSON, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

func parseAccessToken(secret []byte, token string, now time.Time, issuer, audience string) (accessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return accessClaims{}, errors.New("invalid token format")
	}
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signingInput))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return accessClaims{}, fmt.Errorf("invalid token signature encoding: %w", err)
	}
	if !hmac.Equal(expected, got) {
		return accessClaims{}, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return accessClaims{}, fmt.Errorf("invalid token payload: %w", err)
	}
	var claims accessClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return accessClaims{}, fmt.Errorf("invalid token claims: %w", err)
	}
	if claims.Issuer != issuer || claims.Audience != audience {
		return accessClaims{}, errors.New("invalid token audience")
	}
	if claims.ExpiresAt <= now.Unix() {
		return accessClaims{}, errors.New("token expired")
	}
	if claims.Subject == "" || claims.PrincipalType == "" || claims.NetworkID == "" {
		return accessClaims{}, errors.New("incomplete token claims")
	}
	return claims, nil
}
