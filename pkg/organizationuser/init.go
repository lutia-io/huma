package organizationuser

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux) {
	service := newService(
		logger,
		newPostgresStore(pool),
		hasher.NewArgon2IDHasher(),
	)
	newHTTPHandler(service, mux)
}
