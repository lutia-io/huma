package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewTrailingSlashRedirect(t *testing.T) {
	tests := []struct {
		name         string
		target       string
		wantRedirect bool
		wantLocation string
	}{
		{"root untouched", "/", false, ""},
		{"no trailing slash", "/users", false, ""},
		{"single trailing slash", "/users/", true, "/users"},
		{"multiple trailing slashes", "/users///", true, "/users"},
		{"only slashes collapse to root", "//", true, "/"},
		{"query preserved", "/users/?page=2", true, "/users?page=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			h := NewTrailingSlashRedirect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
			}))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tt.target, nil))

			if tt.wantRedirect {
				if rec.Code != http.StatusPermanentRedirect {
					t.Fatalf("status: got %d want %d", rec.Code, http.StatusPermanentRedirect)
				}
				if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
					t.Fatalf("location: got %q want %q", loc, tt.wantLocation)
				}
				if nextCalled {
					t.Fatal("next should not be called on redirect")
				}
			} else {
				if !nextCalled {
					t.Fatal("next should be called when no redirect")
				}
				if rec.Code != http.StatusOK {
					t.Fatalf("status: got %d want 200", rec.Code)
				}
			}
		})
	}
}
