package postgres

import (
	"context"
	"database/sql"
)

// HealthChecker checks database connectivity via PingContext.
type HealthChecker struct {
	db *sql.DB
}

// NewHealthChecker creates a new database HealthChecker.
func NewHealthChecker(db *sql.DB) *HealthChecker {
	return &HealthChecker{db: db}
}

// Ping pings the underlying database.
func (h *HealthChecker) Ping(ctx context.Context) error {
	return h.db.PingContext(ctx)
}
