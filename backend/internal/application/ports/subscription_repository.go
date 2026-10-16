package ports

import (
	"context"
	"errors"

	"github.com/livelaw/backend/internal/domain/billing"
)

var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
)

// SubscriptionRepository defines database persistence operations for subscriptions.
type SubscriptionRepository interface {
	Create(ctx context.Context, sub *billing.Subscription) error
	GetByID(ctx context.Context, id string) (*billing.Subscription, error)
	GetByProviderSubscriptionID(ctx context.Context, providerSubID string) (*billing.Subscription, error)
	GetActiveByUserID(ctx context.Context, userID string) (*billing.Subscription, error)
	Update(ctx context.Context, sub *billing.Subscription) error
	UpsertByProviderID(ctx context.Context, sub *billing.Subscription) error
	List(ctx context.Context, limit int, offset int, status *billing.SubscriptionStatus) ([]billing.Subscription, int, error)
}
