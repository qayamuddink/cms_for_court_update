package billing

import (
	"testing"
	"time"
)

func TestSubscriptionLifecycle(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	sub := &Subscription{
		ID:                     "sub-001",
		UserID:                 "user-001",
		Tier:                   TierPro,
		Status:                 StatusActive,
		StartedAt:              now.Add(-30 * 24 * time.Hour),
		CurrentPeriodEnd:       now.Add(24 * time.Hour),
		AutoRenew:              true,
		PaymentProvider:        "stripe",
		ProviderSubscriptionID: "sub_stripe_123",
		PriceAmountMinor:       99900,
		Currency:               "INR",
		CreatedAt:              now.Add(-30 * 24 * time.Hour),
		UpdatedAt:              now.Add(-30 * 24 * time.Hour),
	}

	// 1. Active subscription is entitled
	if !sub.IsEntitled(now) {
		t.Errorf("expected active subscription to be entitled")
	}

	// 2. Renew subscription
	newPeriod := now.Add(30 * 24 * time.Hour)
	sub.Renew(newPeriod, now)
	if !sub.CurrentPeriodEnd.Equal(newPeriod) {
		t.Errorf("expected period end %v, got %v", newPeriod, sub.CurrentPeriodEnd)
	}

	// 3. Cancel subscription
	sub.Cancel(now)
	if sub.Status != StatusCanceled {
		t.Errorf("expected status cancelled, got %s", sub.Status)
	}
	if sub.AutoRenew {
		t.Errorf("expected auto_renew false after cancellation")
	}
	if sub.CancelledAt == nil {
		t.Errorf("expected cancelled_at to be set")
	}
	// Once cancelled, user is no longer entitled
	if sub.IsEntitled(now) {
		t.Errorf("expected cancelled subscription not to be entitled")
	}

	// 4. Expire subscription
	sub.Expire(now)
	if sub.Status != StatusExpired {
		t.Errorf("expected status expired, got %s", sub.Status)
	}
}
