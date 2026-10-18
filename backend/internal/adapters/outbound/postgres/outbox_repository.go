package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
)

// SQLOutboxRepository implements ports.OutboxRepository using PostgreSQL.
type SQLOutboxRepository struct {
	db *sql.DB
}

// NewSQLOutboxRepository creates a new SQLOutboxRepository.
func NewSQLOutboxRepository(db *sql.DB) *SQLOutboxRepository {
	return &SQLOutboxRepository{db: db}
}

// SaveInTx inserts an outbox event within an active transaction.
func (r *SQLOutboxRepository) SaveInTx(ctx context.Context, tx ports.Tx, event *ports.OutboxEvent) error {
	sqlTx, ok := tx.(*SQLTx)
	if !ok {
		return errors.New("invalid transaction type for outbox save")
	}

	query := `
		INSERT INTO outbox_events (
			id, aggregate_type, aggregate_id, event_type,
			payload, schema_version, created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7
		);
	`

	_, err := sqlTx.tx.ExecContext(ctx, query,
		event.ID,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.Payload,
		event.SchemaVersion,
		event.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed inserting outbox event: %w", err)
	}

	return nil
}

// Save inserts an outbox event outside of an explicit transaction.
func (r *SQLOutboxRepository) Save(ctx context.Context, event *ports.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (
			id, aggregate_type, aggregate_id, event_type,
			payload, schema_version, created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7
		);
	`
	_, err := r.db.ExecContext(ctx, query,
		event.ID,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.Payload,
		event.SchemaVersion,
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed inserting outbox event: %w", err)
	}
	return nil
}

// GetUnpublished retrieves up to limit unpublished events using SKIP LOCKED for safe concurrent worker execution.
func (r *SQLOutboxRepository) GetUnpublished(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT
			id, aggregate_type, aggregate_id, event_type,
			payload, schema_version, created_at::text, published_at::text
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED;
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed querying unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var events []ports.OutboxEvent
	for rows.Next() {
		var e ports.OutboxEvent
		var pubAt sql.NullString

		err := rows.Scan(
			&e.ID,
			&e.AggregateType,
			&e.AggregateID,
			&e.EventType,
			&e.Payload,
			&e.SchemaVersion,
			&e.CreatedAt,
			&pubAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed scanning outbox event: %w", err)
		}

		if pubAt.Valid {
			e.PublishedAt = &pubAt.String
		}

		events = append(events, e)
	}

	return events, rows.Err()
}

// MarkPublished sets published_at on an outbox event.
func (r *SQLOutboxRepository) MarkPublished(ctx context.Context, id string, publishedAt time.Time) error {
	query := `UPDATE outbox_events SET published_at = $1 WHERE id = $2;`
	_, err := r.db.ExecContext(ctx, query, publishedAt, id)
	if err != nil {
		return fmt.Errorf("failed marking outbox event as published: %w", err)
	}
	return nil
}
