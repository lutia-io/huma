package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRealIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xrealip    string
		wantRemote string
	}{
		{
			name:       "x-forwarded-for first ip wins and port preserved",
			remoteAddr: "10.0.0.1:5555",
			xff:        "203.0.113.7, 70.41.3.18, 150.172.238.178",
			wantRemote: "203.0.113.7:5555",
		},
		{
			name:       "falls back to x-real-ip when xff invalid",
			remoteAddr: "10.0.0.1:5555",
			xff:        "not-an-ip",
			xrealip:    "198.51.100.23",
			wantRemote: "198.51.100.23:5555",
		},
		{
			name:       "ipv6 client",
			remoteAddr: "[::1]:5555",
			xff:        "2001:db8::1",
			wantRemote: "[2001:db8::1]:5555",
		},
		{
			name:       "no proxy headers leaves remote addr untouched",
			remoteAddr: "10.0.0.1:5555",
			wantRemote: "10.0.0.1:5555",
		},
		{
			name:       "invalid headers leave remote addr untouched",
			remoteAddr: "10.0.0.1:5555",
			xff:        "garbage",
			xrealip:    "also-garbage",
			wantRemote: "10.0.0.1:5555",
		},
		{
			name:       "no port on remote addr keeps bare host",
			remoteAddr: "10.0.0.1",
			xff:        "203.0.113.7",
			wantRemote: "203.0.113.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			h := NewRealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.RemoteAddr
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xrealip != "" {
				req.Header.Set("X-Real-Ip", tt.xrealip)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)

			if got != tt.wantRemote {
				t.Fatalf("RemoteAddr: got %q want %q", got, tt.wantRemote)
			}
		})
	}
}
