package middleware

import (
	"net"
	"net/http"
	"strings"
)

// NewRealIP overwrites r.RemoteAddr based on proxy headers.
//
// This assumes your reverse proxy is trusted to set these headers correctly.
// If the service is exposed directly to the internet, consider restricting this
// to known proxy IP ranges.
func NewRealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip, ok := realClientIP(r); ok {
			r.RemoteAddr = withOriginalPort(r.RemoteAddr, ip)
		}
		next.ServeHTTP(w, r)
	})
}

// realClientIP extracts the originating client IP from proxy headers, returning
// false when no valid IP can be found. Only syntactically valid IPs are
// accepted so a malformed header can never overwrite RemoteAddr.
func realClientIP(r *http.Request) (string, bool) {
	// Prefer X-Forwarded-For: first IP is original client.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if parsed := net.ParseIP(ip); parsed != nil {
				return ip, true
			}
		}
	}
	// Fall back to X-Real-IP, which carries a single client IP.
	if xri := strings.TrimSpace(r.Header.Get("X-Real-Ip")); xri != "" {
		if parsed := net.ParseIP(xri); parsed != nil {
			return xri, true
		}
	}
	return "", false
}

// withOriginalPort joins host with the port from remoteAddr when present, so we
// preserve the original source port while only swapping the host portion.
func withOriginalPort(remoteAddr, host string) string {
	if _, port, err := net.SplitHostPort(remoteAddr); err == nil && port != "" {
		return net.JoinHostPort(host, port)
	}
	return host
}
