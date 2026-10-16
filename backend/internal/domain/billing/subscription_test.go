package billing

import (
	"testing"
	"time"
)

func TestSubscriptionIsEntitled(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	activeSub := &Subscription{
		Status:           StatusActive,
		CurrentPeriodEnd: now.Add(24 * time.Hour),
	}
	if !activeSub.IsEntitled(now) {
		t.Errorf("expected active subscription to be entitled")
	}

	expiredSub := &Subscription{
		Status:           StatusActive,
		CurrentPeriodEnd: now.Add(-1 * time.Hour),
	}
	if expiredSub.IsEntitled(now) {
		t.Errorf("expected expired subscription not to be entitled")
	}

	cancelledSub := &Subscription{
		Status:           StatusCanceled,
		CurrentPeriodEnd: now.Add(24 * time.Hour),
	}
	if cancelledSub.IsEntitled(now) {
		t.Errorf("expected cancelled subscription not to be entitled")
	}
}
