package pipeline

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/file"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/organizationuser"
	"github.com/lutia-io/huma/pkg/pipeline/executor"
	"github.com/lutia-io/huma/pkg/pipeline/executor/handlers"
	"github.com/lutia-io/huma/pkg/record"
	"github.com/lutia-io/huma/pkg/schema"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
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

	nc, err := nats.Connect(os.Getenv("HUMA_SERVICE_NATS_URI"))
	if err != nil {
		log.Error("Unable to create NATS connection", logger.KeyError, err)
		os.Exit(1)
	}
	defer nc.Drain()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Error("Unable to create JetStream", logger.KeyError, err)
		os.Exit(1)
	}

	objCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	objs, err := js.CreateOrUpdateObjectStore(objCtx, jetstream.ObjectStoreConfig{
		Bucket:  file.ObjectStoreBucket,
		Storage: jetstream.FileStorage,
	})
	if err != nil {
		log.Error("Unable to create files object store", logger.KeyError, err)
		os.Exit(1)
	}

	schemaService := schema.NewWithPool(log, pool)
	recordService := record.NewWithPool(log, pool, js, schemaService)
	fileService := file.NewWithPool(log, pool, objs)
	orgUserService := organizationuser.NewWithPool(log, pool)

	pipelineStore := executor.NewPostgresPipelineStore(pool, workerLeaseTimeout)
	registry := executor.NewRegistry(
		handlers.NewNoop(),
		handlers.NewHTTP(nil),
		handlers.NewMapper(),
		handlers.NewListMapper(),
		handlers.NewFile(fileService),
		handlers.NewRecord(recordService, orgUserService),
		handlers.NewBulk(recordService, orgUserService),
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
	srv := http.Server{
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
