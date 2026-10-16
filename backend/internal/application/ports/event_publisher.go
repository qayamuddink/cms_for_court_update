package ports

import "context"

// OutboxEvent represents an event envelope stored in the database.
type OutboxEvent struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	SchemaVersion int
	CreatedAt     string
	PublishedAt   *string
}

// EventPublisher defines the port for publishing domain and outbox events.
type EventPublisher interface {
	Publish(ctx context.Context, event OutboxEvent) error
}
