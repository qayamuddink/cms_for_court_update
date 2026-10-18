package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/livelaw/backend/internal/application/ports"
)

// SQLSearchAnalyticsRepository implements ports.SearchAnalyticsRepository using PostgreSQL.
type SQLSearchAnalyticsRepository struct {
	db *sql.DB
}

// NewSQLSearchAnalyticsRepository creates a new SQLSearchAnalyticsRepository.
func NewSQLSearchAnalyticsRepository(db *sql.DB) *SQLSearchAnalyticsRepository {
	return &SQLSearchAnalyticsRepository{db: db}
}

func (r *SQLSearchAnalyticsRepository) RecordQuery(ctx context.Context, query string, hitCount int, ipHash string, hasFilters bool) error {
	id := uuid.New().String()
	isZero := hitCount == 0

	querySQL := `
		INSERT INTO search_analytics (id, query, hit_count, is_zero_result, has_filters, ip_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, querySQL, id, query, hitCount, isZero, hasFilters, ipHash, time.Now().UTC())
	return err
}

func (r *SQLSearchAnalyticsRepository) GetTopQueries(ctx context.Context, limit int) ([]ports.QueryCount, error) {
	if limit <= 0 {
		limit = 10
	}

	querySQL := `
		SELECT query, COUNT(*) as count
		FROM search_analytics
		GROUP BY query
		ORDER BY count DESC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, querySQL, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ports.QueryCount
	for rows.Next() {
		var qc ports.QueryCount
		if err := rows.Scan(&qc.Query, &qc.Count); err != nil {
			return nil, err
		}
		results = append(results, qc)
	}
	return results, rows.Err()
}

func (r *SQLSearchAnalyticsRepository) GetZeroResultQueries(ctx context.Context, limit int) ([]ports.ZeroResultQuery, error) {
	if limit <= 0 {
		limit = 10
	}

	querySQL := `
		SELECT query, created_at, has_filters
		FROM search_analytics
		WHERE is_zero_result = true
		ORDER BY created_at DESC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, querySQL, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ports.ZeroResultQuery
	for rows.Next() {
		var zq ports.ZeroResultQuery
		if err := rows.Scan(&zq.Query, &zq.SearchedAt, &zq.HasFilters); err != nil {
			return nil, err
		}
		results = append(results, zq)
	}
	return results, rows.Err()
}

func (r *SQLSearchAnalyticsRepository) GetTotalSearches(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM search_analytics").Scan(&count)
	return count, err
}
