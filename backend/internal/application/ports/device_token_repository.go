package ports

import (
	"context"
	"time"
)

// DeviceToken represents an active mobile or browser push token registered for a user.
type DeviceToken struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Platform   string    `json:"platform"` // "ios", "android", "web"
	PushToken  string    `json:"push_token"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// DeviceTokenRepository defines persistence operations for user push device tokens.
type DeviceTokenRepository interface {
	Upsert(ctx context.Context, token *DeviceToken) error
	GetTokensByUserIDs(ctx context.Context, userIDs []string) ([]DeviceToken, error)
	Delete(ctx context.Context, userID, pushToken string) error
}
