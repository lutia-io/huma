package workflow

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/nats-io/nats.go"
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

	port := os.Getenv("HUMA_SERVICE_PORT")
	log.Info("Workflow is listening and serving", logger.KeyPort, port)
}
