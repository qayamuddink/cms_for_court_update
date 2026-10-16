package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
)

// OutboxWorker consumes unsent outbox rows and passes them to the EventPublisher.
type OutboxWorker struct {
	outboxRepo   ports.OutboxRepository
	publisher    ports.EventPublisher
	pollInterval time.Duration
	clock        ports.Clock
	logger       *slog.Logger
}

// NewOutboxWorker creates a new OutboxWorker instance.
func NewOutboxWorker(
	outboxRepo ports.OutboxRepository,
	publisher ports.EventPublisher,
	pollInterval time.Duration,
	clock ports.Clock,
	logger *slog.Logger,
) *OutboxWorker {
	if pollInterval <= 0 {
		pollInterval = 200 * time.Millisecond
	}
	if clock == nil {
		clock = ports.RealClock{}
	}
	return &OutboxWorker{
		outboxRepo:   outboxRepo,
		publisher:    publisher,
		pollInterval: pollInterval,
		clock:        clock,
		logger:       logger,
	}
}

// Start launches the background polling loop until ctx is cancelled.
func (w *OutboxWorker) Start(ctx context.Context) {
	w.logger.Info("outbox worker started", slog.Duration("poll_interval", w.pollInterval))
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("outbox worker shutting down")
			return
		case <-ticker.C:
			w.drainPending(ctx)
		}
	}
}

func (w *OutboxWorker) drainPending(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		processed, err := w.ProcessBatch(ctx, 50)
		if err != nil {
			w.logger.Error("error processing outbox batch", slog.Any("error", err))
			return
		}

		// If fewer than 50 events processed, the backlog is clear; wait for next tick
		if processed < 50 {
			return
		}
	}
}

// ProcessBatch processes up to limit pending outbox events.
func (w *OutboxWorker) ProcessBatch(ctx context.Context, limit int) (int, error) {
	events, err := w.outboxRepo.GetUnpublished(ctx, limit)
	if err != nil {
		return 0, err
	}

	if len(events) == 0 {
		return 0, nil
	}

	now := w.clock.Now()
	for _, event := range events {
		if err := w.publisher.Publish(ctx, event); err != nil {
			w.logger.Error("failed publishing outbox event, will retry",
				slog.String("event_id", event.ID),
				slog.Any("error", err),
			)
			// Break to retry later
			return 0, err
		}

		if err := w.outboxRepo.MarkPublished(ctx, event.ID, now); err != nil {
			w.logger.Error("failed marking event as published",
				slog.String("event_id", event.ID),
				slog.Any("error", err),
			)
			return 0, err
		}
	}

	return len(events), nil
}
