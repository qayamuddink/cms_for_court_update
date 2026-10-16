package billing

import "time"

// SubscriptionTier represents subscription level.
type SubscriptionTier string

const (
	TierFree          SubscriptionTier = "free"
	TierPro           SubscriptionTier = "pro"
	TierInstitutional SubscriptionTier = "institutional"
)

// SubscriptionStatus represents subscription lifecycle status.
type SubscriptionStatus string

const (
	StatusTrialing SubscriptionStatus = "trialing"
	StatusActive   SubscriptionStatus = "active"
	StatusPastDue  SubscriptionStatus = "past_due"
	StatusCanceled SubscriptionStatus = "cancelled"
	StatusExpired  SubscriptionStatus = "expired"
)

// Subscription represents the user's subscription record.
type Subscription struct {
	ID                     string
	UserID                 string
	Tier                   SubscriptionTier
	Status                 SubscriptionStatus
	StartedAt              time.Time
	CurrentPeriodEnd       time.Time
	CancelledAt            *time.Time
	AutoRenew              bool
	PaymentProvider        string
	ProviderSubscriptionID string
	PriceAmountMinor       int64
	Currency               string
	SeatGroupID            *string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// IsEntitled returns true if subscription provides active access.
func (s *Subscription) IsEntitled(now time.Time) bool {
	if s.Status == StatusActive || s.Status == StatusTrialing {
		return now.Before(s.CurrentPeriodEnd)
	}
	return false
}
