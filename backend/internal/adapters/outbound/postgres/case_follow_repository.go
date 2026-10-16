package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/livelaw/backend/internal/application/ports"
)

// SQLCaseFollowRepository implements ports.CaseFollowRepository using PostgreSQL.
type SQLCaseFollowRepository struct {
	db *sql.DB
}

// NewSQLCaseFollowRepository creates a new SQLCaseFollowRepository instance.
func NewSQLCaseFollowRepository(db *sql.DB) *SQLCaseFollowRepository {
	return &SQLCaseFollowRepository{db: db}
}

// Follow records or updates a case follow subscription.
func (r *SQLCaseFollowRepository) Follow(ctx context.Context, follow *ports.CaseFollow) error {
	notifyOnJSON, err := json.Marshal(follow.NotifyOn)
	if err != nil {
		return fmt.Errorf("failed marshaling notify_on: %w", err)
	}

	query := `
		INSERT INTO case_follows (id, user_id, court_case_id, notify_on, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, court_case_id) DO UPDATE SET
			notify_on = EXCLUDED.notify_on;
	`

	_, err = r.db.ExecContext(ctx, query,
		follow.ID,
		follow.UserID,
		follow.CourtCaseID,
		notifyOnJSON,
		follow.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed saving case follow: %w", err)
	}
	return nil
}

// Unfollow deletes a case follow subscription.
func (r *SQLCaseFollowRepository) Unfollow(ctx context.Context, userID, caseID string) error {
	query := `DELETE FROM case_follows WHERE user_id = $1 AND court_case_id = $2;`
	_, err := r.db.ExecContext(ctx, query, userID, caseID)
	if err != nil {
		return fmt.Errorf("failed deleting case follow: %w", err)
	}
	return nil
}

// IsFollowing checks if a user is currently following a court case.
func (r *SQLCaseFollowRepository) IsFollowing(ctx context.Context, userID, caseID string) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM case_follows WHERE user_id = $1 AND court_case_id = $2);`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, userID, caseID).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed checking case follow: %w", err)
	}
	return exists, nil
}

// GetFollow retrieves a case follow record for a user and court case.
func (r *SQLCaseFollowRepository) GetFollow(ctx context.Context, userID, caseID string) (*ports.CaseFollow, error) {
	query := `
		SELECT id, user_id, court_case_id, notify_on, created_at
		FROM case_follows
		WHERE user_id = $1 AND court_case_id = $2;
	`
	var f ports.CaseFollow
	var notifyOnRaw []byte

	err := r.db.QueryRowContext(ctx, query, userID, caseID).Scan(
		&f.ID,
		&f.UserID,
		&f.CourtCaseID,
		&notifyOnRaw,
		&f.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed querying case follow: %w", err)
	}

	if len(notifyOnRaw) > 0 {
		if err := json.Unmarshal(notifyOnRaw, &f.NotifyOn); err != nil {
			return nil, fmt.Errorf("failed unmarshaling notify_on: %w", err)
		}
	}

	return &f, nil
}

// GetFollowersByCaseID returns user IDs of users following the given case.
func (r *SQLCaseFollowRepository) GetFollowersByCaseID(ctx context.Context, caseID string, eventType string) ([]string, error) {
	var query string
	var args []any

	if eventType != "" {
		query = `
			SELECT user_id FROM case_follows
			WHERE court_case_id = $1 AND notify_on ? $2;
		`
		args = []any{caseID, eventType}
	} else {
		query = `
			SELECT user_id FROM case_follows
			WHERE court_case_id = $1;
		`
		args = []any{caseID}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed querying case followers: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("failed scanning follower user_id: %w", err)
		}
		userIDs = append(userIDs, uid)
	}

	if userIDs == nil {
		userIDs = []string{}
	}

	return userIDs, nil
}

// CountFollowers returns total number of followers for a given case.
func (r *SQLCaseFollowRepository) CountFollowers(ctx context.Context, caseID string) (int, error) {
	query := `SELECT COUNT(*) FROM case_follows WHERE court_case_id = $1;`
	var count int
	if err := r.db.QueryRowContext(ctx, query, caseID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed counting case followers: %w", err)
	}
	return count, nil
}
