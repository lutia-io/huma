package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/middleware"
	"github.com/lutia-io/huma/pkg/network"
	"github.com/lutia-io/huma/pkg/organization"
	"github.com/lutia-io/huma/pkg/record"
	"github.com/lutia-io/huma/pkg/schema"
	"github.com/lutia-io/huma/pkg/user"
)

func New() {
	log := logger.New()

	pool, err := pgxpool.New(context.Background(), os.Getenv("API_SERVICE_POSTGRES_RW_URI"))
	if err != nil {
		log.Error("Unable to create db connection pool", logger.KeyError, err)
		os.Exit(1)
	}
	defer pool.Close()

	mux := http.NewServeMux()
	handler := http.Handler(mux)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	user.New(log, pool, mux)
	network.New(log, pool, mux)
	organization.New(log, pool, mux)
	schema.New(log, pool, mux)
	record.New(log, pool, mux)

	handler = middleware.NewTrailingSlashRedirect(handler)
	handler = middleware.NewBodySizeLimit(5<<20, handler)
	handler = middleware.NewTimeout(30*time.Second, handler)
	handler = middleware.NewRecover(log, handler)
	handler = middleware.NewLogger(log, handler)
	handler = middleware.NewRequestID(handler)
	handler = middleware.NewRealIP(handler)

	port := os.Getenv("API_SERVICE_PORT")
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("API is listening and serving", logger.KeyPort, port)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("Failed to listen and serve", logger.KeyError, err)
	}
}
