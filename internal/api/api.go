package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/lutia-io/huma/pkg/middleware"
)

func New() {
	logger := slog.Default()

	mux := http.NewServeMux()
	handler := http.Handler(mux)

	handler = middleware.NewTrailingSlashRedirect(handler)
	handler = middleware.NewBodySizeLimit(5<<20, handler)
	handler = middleware.NewTimeout(30*time.Second, handler)
	handler = middleware.NewRecover(logger, handler)
	handler = middleware.NewLogger(logger, handler)
	handler = middleware.NewRequestID(handler)
	handler = middleware.NewRealIP(handler)

	port := os.Getenv("API_SERVICE_PORT")
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Info("API is listening and serving", "port", port)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("Failed to listen and serve", "error", err)
	}
}
