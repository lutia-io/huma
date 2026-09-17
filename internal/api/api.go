package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/auth"
	"github.com/lutia-io/huma/pkg/authz"
	"github.com/lutia-io/huma/pkg/file"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/middleware"
	"github.com/lutia-io/huma/pkg/network"
	"github.com/lutia-io/huma/pkg/networkaccess"
	"github.com/lutia-io/huma/pkg/organization"
	"github.com/lutia-io/huma/pkg/organizationaccess"
	"github.com/lutia-io/huma/pkg/organizationuser"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/record"
	"github.com/lutia-io/huma/pkg/schema"
	"github.com/lutia-io/huma/pkg/user"
	"github.com/lutia-io/huma/pkg/workflow"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func New() {
	log := logger.New()

	pool, err := pgxpool.New(context.Background(), os.Getenv("HUMA_SERVICE_POSTGRES_RW_URI"))
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "RECORDS",
		Subjects: []string{"records.>"},
		Storage:  jetstream.FileStorage,
	})
	if err != nil {
		log.Error("Unable to create stream", logger.KeyError, err)
		os.Exit(1)
	}
	objs, err := js.CreateOrUpdateObjectStore(ctx, jetstream.ObjectStoreConfig{
		Bucket:  file.ObjectStoreBucket,
		Storage: jetstream.FileStorage,
	})
	if err != nil {
		log.Error("Unable to create files object store", logger.KeyError, err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	engine := authz.New(pool)
	authz.RegisterHTTP(engine, mux)
	authService := auth.New(log, pool, mux)
	user.New(log, pool, mux)
	network.New(log, pool, mux, engine)
	networkaccess.New(log, pool, mux, engine)
	orgUserService := organizationuser.New(log, pool, mux, engine)
	organization.New(log, pool, mux, orgUserService, engine)
	organizationaccess.New(log, pool, mux, engine)
	schemaService := schema.New(log, pool, mux, engine)
	pipeline.New(log, pool, mux, engine)
	workflow.New(log, pool, mux, engine)
	file.New(log, pool, mux, objs, orgUserService, engine)
	record.New(log, pool, mux, js, schemaService, orgUserService, engine)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := http.Handler(mux)
	handler = auth.Middleware(authService)(handler)
	handler = organization.Middleware(handler)
	handler = network.Middleware(handler)
	handler = middleware.NewTrailingSlashRedirect(handler)
	// 32 MiB to accommodate multipart file uploads (JSON endpoints stay small).
	handler = middleware.NewBodySizeLimit(32<<20, handler)
	handler = middleware.NewTimeout(30*time.Second, handler)
	handler = middleware.NewRecover(log, handler)
	handler = middleware.NewLogger(log, handler)
	handler = middleware.NewRequestID(handler)
	handler = middleware.NewRealIP(handler)

	port := os.Getenv("HUMA_SERVICE_PORT")
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
