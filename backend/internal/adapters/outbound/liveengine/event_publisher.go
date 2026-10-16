package liveengine

import (
	"context"
	"log/slog"

	"github.com/livelaw/backend/internal/application/ports"
)

// LogEventPublisher implements ports.EventPublisher by logging events and dispatching in-process.
type LogEventPublisher struct {
	logger *slog.Logger
}

// NewLogEventPublisher creates a new LogEventPublisher.
func NewLogEventPublisher(logger *slog.Logger) *LogEventPublisher {
	return &LogEventPublisher{logger: logger}
}

// Publish processes an outbox event.
func (p *LogEventPublisher) Publish(ctx context.Context, event ports.OutboxEvent) error {
	p.logger.Info("event published to stream",
		slog.String("event_id", event.ID),
		slog.String("aggregate_type", event.AggregateType),
		slog.String("aggregate_id", event.AggregateID),
		slog.String("event_type", event.EventType),
		slog.Int("schema_version", event.SchemaVersion),
	)
	return nil
}
