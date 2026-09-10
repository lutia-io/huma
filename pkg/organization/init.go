package organization

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
)

// SystemUserSeeder creates the hidden system organization user for a new
// organization. Implemented by organizationuser.Service.
type SystemUserSeeder interface {
	EnsureSystemUser(ctx context.Context, organizationID, networkID string) (string, error)
}

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux, systemUsers SystemUserSeeder) {
	service := newService(
		logger,
		newPostgresStore(pool),
		hasher.NewArgon2IDHasher(),
		systemUsers,
	)
	newHTTPHandler(service, mux)
}
