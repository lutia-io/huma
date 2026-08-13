package auth

import (
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
)

// New wires auth routes and returns the service for middleware.
func New(log *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux) *service {
	secret := os.Getenv("HUMA_SERVICE_JWT_SECRET")
	if secret == "" {
		log.Error("HUMA_SERVICE_JWT_SECRET is required")
		os.Exit(1)
	}
	svc := newService(log, newPostgresStore(pool), hasher.NewArgon2IDHasher(), []byte(secret))
	newHTTPHandler(svc, mux)
	return svc
}
