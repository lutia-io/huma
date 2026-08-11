package auth

import (
	"net/http"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/render"
)

// publicExact routes are reachable without an access token.
var publicExact = map[string]struct{}{
	"GET /healthz":                       {},
	"GET /readyz":                        {},
	"POST /auth/login/user":              {},
	"POST /auth/login/organization-user": {},
	"POST /auth/refresh":                 {},
	"POST /auth/logout":                  {},
	"POST /user":                         {}, // bootstrap registration
}

// Middleware requires a valid access token except for public routes.
func Middleware(svc *service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Method + " " + r.URL.Path
			if _, ok := publicExact[key]; ok {
				next.ServeHTTP(w, r)
				return
			}

			raw, ok := bearerToken(r)
			if !ok {
				render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
				return
			}
			p, err := svc.ParseAccessToken(raw)
			if err != nil {
				render.WriteError(w, err)
				return
			}
			ctx := principal.WithContext(r.Context(), p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
