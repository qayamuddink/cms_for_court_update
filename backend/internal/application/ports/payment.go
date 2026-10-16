package ports

import "context"

// PaymentWebhookPayload represents the normalized payload from payment providers.
type PaymentWebhookPayload struct {
	ProviderSubscriptionID string
	UserID                 string
	Tier                   string
	Status                 string
	CurrentPeriodEnd       string
	EventID                string
}

// PaymentGateway defines interactions with payment provider.
type PaymentGateway interface {
	VerifyWebhookSignature(ctx context.Context, payload []byte, signature string) bool
	CancelSubscription(ctx context.Context, providerSubscriptionID string) error
}
