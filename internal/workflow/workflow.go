package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/record"
	wf "github.com/lutia-io/huma/pkg/workflow"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func New() {
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

	store := wf.NewPostgresStore(pool)
	svc := wf.NewService(log, store)

	consumer, err := js.CreateOrUpdateConsumer(ctx, record.StreamName, jetstream.ConsumerConfig{
		Durable:       "workflow-executor",
		FilterSubject: record.SubjectCreated,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		log.Error("Unable to create consumer", logger.KeyError, err)
		os.Exit(1)
	}

	_, err = consumer.Consume(func(msg jetstream.Msg) {
		msgCtx := context.Background()

		var event record.CreatedEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			log.Error("Failed to unmarshal record created event", logger.KeyError, err)
			_ = msg.Term() // poison message
			return
		}

		log.Info("Executing workflows for record", logger.KeyID, event.ID, "schema_id", event.SchemaID)

		if err := svc.ExecuteForRecord(msgCtx, event.SchemaID, event.ID, event.Data); err != nil {
			log.Error("Failed to execute workflows for record",
				logger.KeyID, event.ID, logger.KeyError, err)
			_ = msg.Nak()
			return
		}

		_ = msg.Ack()
	})
	if err != nil {
		log.Error("Unable to start consumer", logger.KeyError, err)
		os.Exit(1)
	}

	// health endpoints for k8s (unchanged)
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
	log.Info("Workflow is listening and serving", logger.KeyPort, port)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("Failed to listen and serve", logger.KeyError, err)
	}
}
