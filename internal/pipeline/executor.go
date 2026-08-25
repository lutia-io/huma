package pipeline

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/pipeline/executor"
	"github.com/lutia-io/huma/pkg/pipeline/executor/handlers"
)

const (
	defaultWorkerCount = 4
	workerPollInterval = time.Second
	workerLeaseTimeout = 2 * time.Minute
)

func NewExecutor() {
	log := logger.New()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, os.Getenv("HUMA_SERVICE_POSTGRES_RW_URI"))
	if err != nil {
		log.Error("Unable to create db connection pool", logger.KeyError, err)
		os.Exit(1)
	}
	defer pool.Close()

	pipelineStore := executor.NewPostgresPipelineStore(pool, workerLeaseTimeout)
	registry := executor.NewRegistry(
		handlers.NewNoop(),
		handlers.NewHTTP(nil),
	)

	service := executor.NewService(log, pipelineStore, registry)
	service.StartWorkers(ctx, workerCount(log), "pipeline-executor", workerPollInterval)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	port := os.Getenv("HUMA_SERVICE_PORT")
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("Pipeline executor is listening and serving", logger.KeyPort, port)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("Failed to listen and serve", logger.KeyError, err)
	}
}

func workerCount(log *logger.Logger) int {
	raw := os.Getenv("HUMA_PIPELINE_WORKER_COUNT")
	if raw == "" {
		return defaultWorkerCount
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		log.WarnContext(context.Background(), "Invalid HUMA_PIPELINE_WORKER_COUNT, using default", "value", raw, logger.KeyCount, defaultWorkerCount)
		return defaultWorkerCount
	}
	return n
}
