package middleware

import (
	"net/http"
	"strings"
)

// NewTrailingSlashRedirect normalizes request paths by stripping trailing
// slashes (e.g. "/path/" or "/path///" -> "/path"), with "/" left untouched.
//
// It redirects with 308 Permanent Redirect so the HTTP method and body are
// preserved (important for POST/PUT/PATCH). Only the path is rewritten and the
// query string is carried over; because the redirect target is derived solely
// from the request URL's path it stays relative and cannot be used for an open
// redirect.
func NewTrailingSlashRedirect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p != "/" && strings.HasSuffix(p, "/") {
			trimmed := strings.TrimRight(p, "/")
			if trimmed == "" {
				// Path was nothing but slashes (e.g. "//"); collapse to root.
				trimmed = "/"
			}
			u := *r.URL
			u.Path = trimmed
			http.Redirect(w, r, u.String(), http.StatusPermanentRedirect)
			return
		}
		next.ServeHTTP(w, r)
	})
}
