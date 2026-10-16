package queries

import (
	"context"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/billing"
	"github.com/livelaw/backend/internal/domain/identity"
)

// SubscriptionQueries handles read queries for subscription management.
type SubscriptionQueries struct {
	subRepo ports.SubscriptionRepository
}

// NewSubscriptionQueries creates a new SubscriptionQueries instance.
func NewSubscriptionQueries(subRepo ports.SubscriptionRepository) *SubscriptionQueries {
	return &SubscriptionQueries{subRepo: subRepo}
}

// GetMySubscription retrieves the current active subscription for a reader.
func (q *SubscriptionQueries) GetMySubscription(ctx context.Context, userID string) (*billing.Subscription, error) {
	sub, err := q.subRepo.GetActiveByUserID(ctx, userID)
	if err != nil {
		if err == ports.ErrSubscriptionNotFound {
			return nil, nil
		}
		return nil, err
	}
	return sub, nil
}

// SubscriptionListResponse represents paginated subscriptions for CMS administrators.
type SubscriptionListResponse struct {
	Subscriptions []billing.Subscription `json:"subscriptions"`
	Total         int                    `json:"total"`
}

// ListSubscriptions returns paginated subscriptions for staff administrators.
func (q *SubscriptionQueries) ListSubscriptions(
	ctx context.Context,
	actorRole identity.SystemRole,
	limit, offset int,
	status *billing.SubscriptionStatus,
) (*SubscriptionListResponse, error) {
	if actorRole != identity.RoleAdmin && actorRole != identity.RoleEditor {
		return nil, ports.ErrUnauthorized
	}

	subs, total, err := q.subRepo.List(ctx, limit, offset, status)
	if err != nil {
		return nil, err
	}

	return &SubscriptionListResponse{
		Subscriptions: subs,
		Total:         total,
	}, nil
}
