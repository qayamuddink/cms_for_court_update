package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/livelaw/backend/internal/application/ports"
)

// SQLDeviceTokenRepository implements ports.DeviceTokenRepository using PostgreSQL.
type SQLDeviceTokenRepository struct {
	db *sql.DB
}

// NewSQLDeviceTokenRepository creates a new SQLDeviceTokenRepository instance.
func NewSQLDeviceTokenRepository(db *sql.DB) *SQLDeviceTokenRepository {
	return &SQLDeviceTokenRepository{db: db}
}

// Upsert registers or updates an active device push token.
func (r *SQLDeviceTokenRepository) Upsert(ctx context.Context, token *ports.DeviceToken) error {
	query := `
		INSERT INTO device_tokens (id, user_id, platform, push_token, last_seen_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, platform, push_token) DO UPDATE SET
			last_seen_at = EXCLUDED.last_seen_at;
	`
	_, err := r.db.ExecContext(ctx, query,
		token.ID,
		token.UserID,
		token.Platform,
		token.PushToken,
		token.LastSeenAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed upserting device token: %w", err)
	}
	return nil
}

// GetTokensByUserIDs retrieves all registered device tokens for the provided user IDs.
func (r *SQLDeviceTokenRepository) GetTokensByUserIDs(ctx context.Context, userIDs []string) ([]ports.DeviceToken, error) {
	if len(userIDs) == 0 {
		return []ports.DeviceToken{}, nil
	}

	placeholders := make([]string, len(userIDs))
	args := make([]any, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, platform, push_token, last_seen_at
		FROM device_tokens
		WHERE user_id IN (%s);
	`, strings.Join(placeholders, ", "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed querying device tokens: %w", err)
	}
	defer rows.Close()

	var tokens []ports.DeviceToken
	for rows.Next() {
		var t ports.DeviceToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Platform, &t.PushToken, &t.LastSeenAt); err != nil {
			return nil, fmt.Errorf("failed scanning device token: %w", err)
		}
		tokens = append(tokens, t)
	}

	if tokens == nil {
		tokens = []ports.DeviceToken{}
	}

	return tokens, nil
}

// Delete removes a registered device token for a user.
func (r *SQLDeviceTokenRepository) Delete(ctx context.Context, userID, pushToken string) error {
	query := `DELETE FROM device_tokens WHERE user_id = $1 AND push_token = $2;`
	_, err := r.db.ExecContext(ctx, query, userID, pushToken)
	if err != nil {
		return fmt.Errorf("failed deleting device token: %w", err)
	}
	return nil
}
