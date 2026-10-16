package ports

import (
	"context"
	"time"
)

// PaymentWebhookPayload represents the normalized payload from payment providers.
type PaymentWebhookPayload struct {
	EventID                string    `json:"event_id"`
	EventType              string    `json:"event_type"` // "subscription.created", "subscription.renewed", "subscription.cancelled"
	ProviderSubscriptionID string    `json:"provider_subscription_id"`
	UserID                 string    `json:"user_id"`
	Tier                   string    `json:"tier"`
	Status                 string    `json:"status"`
	StartedAt              time.Time `json:"started_at"`
	CurrentPeriodEnd       time.Time `json:"current_period_end"`
	PriceAmountMinor       int64     `json:"price_amount_minor"`
	Currency               string    `json:"currency"`
}

// PaymentGateway defines interactions with payment provider.
type PaymentGateway interface {
	VerifyWebhookSignature(ctx context.Context, payload []byte, signature string) bool
	CancelSubscription(ctx context.Context, providerSubscriptionID string) error
}
