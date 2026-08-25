package executor

import (
	"context"
	"time"

	"github.com/lutia-io/huma/pkg/logger"
)

type Service struct {
	logger    *logger.Logger
	pipelines PipelineStore
	registry  *Registry
}

func NewService(logger *logger.Logger, pipelines PipelineStore, registry *Registry) *Service {
	return &Service{
		logger:    logger,
		pipelines: pipelines,
		registry:  registry,
	}
}

func (s *Service) StartWorkers(ctx context.Context, count int, workerIDPrefix string, pollInterval time.Duration) {
	for i := range count {
		w := &worker{
			id:           workerID(workerIDPrefix, i),
			service:      s,
			pollInterval: pollInterval,
		}
		go w.run(ctx)
	}
	go s.sweepExhausted(ctx)
	s.logger.Info("Pipeline workers started", logger.KeyCount, count)
}

func (s *Service) sweepExhausted(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.pipelines.FailExhausted(ctx)
			if err != nil {
				s.logger.ErrorContext(ctx, "Failed to sweep exhausted pipelines", logger.KeyError, err)
				continue
			}
			if n > 0 {
				s.logger.WarnContext(ctx, "Marked exhausted pipelines as failed", logger.KeyCount, n)
			}
		}
	}
}
