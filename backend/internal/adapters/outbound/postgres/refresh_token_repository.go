package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
)

// SQLRefreshTokenRepository implements ports.RefreshTokenRepository using PostgreSQL.
type SQLRefreshTokenRepository struct {
	db *sql.DB
}

// NewSQLRefreshTokenRepository creates a new SQLRefreshTokenRepository.
func NewSQLRefreshTokenRepository(db *sql.DB) *SQLRefreshTokenRepository {
	return &SQLRefreshTokenRepository{db: db}
}

// Create inserts a new refresh token record.
func (r *SQLRefreshTokenRepository) Create(ctx context.Context, token *ports.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (
			id, user_id, token_hash, expires_at, revoked_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6
		);
	`

	_, err := r.db.ExecContext(ctx, query,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.RevokedAt,
		token.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert refresh token: %w", err)
	}

	return nil
}

// GetByHash retrieves a refresh token by its SHA-256 hash.
func (r *SQLRefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*ports.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1;
	`

	row := r.db.QueryRowContext(ctx, query, hash)

	var t ports.RefreshToken
	err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to scan refresh token: %w", err)
	}

	return &t, nil
}

// Revoke marks a specific refresh token as revoked.
func (r *SQLRefreshTokenRepository) Revoke(ctx context.Context, hash string, revokedAt time.Time) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $1
		WHERE token_hash = $2 AND revoked_at IS NULL;
	`

	_, err := r.db.ExecContext(ctx, query, revokedAt, hash)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}

	return nil
}

// RevokeAllForUser marks all active refresh tokens for a user as revoked.
func (r *SQLRefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string, revokedAt time.Time) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $1
		WHERE user_id = $2 AND revoked_at IS NULL;
	`

	_, err := r.db.ExecContext(ctx, query, revokedAt, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke user refresh tokens: %w", err)
	}

	return nil
}
