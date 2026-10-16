package billing

import (
	"errors"
	"time"
)

var (
	ErrSubscriptionExpired = errors.New("subscription has expired")
	ErrInvalidStatusChange = errors.New("invalid subscription status transition")
)

// SubscriptionTier represents subscription level.
type SubscriptionTier string

const (
	TierFree               SubscriptionTier = "free"
	TierPro                SubscriptionTier = "pro"
	TierIndividualMonthly  SubscriptionTier = "individual_monthly"
	TierIndividualYearly   SubscriptionTier = "individual_yearly"
	TierInstitutional      SubscriptionTier = "institutional"
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
	ID                     string             `json:"id"`
	UserID                 string             `json:"user_id"`
	Tier                   SubscriptionTier   `json:"tier"`
	Status                 SubscriptionStatus `json:"status"`
	StartedAt              time.Time          `json:"started_at"`
	CurrentPeriodEnd       time.Time          `json:"current_period_end"`
	CancelledAt            *time.Time         `json:"cancelled_at,omitempty"`
	AutoRenew              bool               `json:"auto_renew"`
	PaymentProvider        string             `json:"payment_provider"`
	ProviderSubscriptionID string             `json:"provider_subscription_id"`
	PriceAmountMinor       int64              `json:"price_amount_minor"`
	Currency               string             `json:"currency"`
	SeatGroupID            *string            `json:"seat_group_id,omitempty"`
	CreatedAt              time.Time          `json:"created_at"`
	UpdatedAt              time.Time          `json:"updated_at"`
}

// IsEntitled returns true if subscription provides active access.
func (s *Subscription) IsEntitled(now time.Time) bool {
	if s.Status == StatusActive || s.Status == StatusTrialing {
		return now.Before(s.CurrentPeriodEnd)
	}
	return false
}

// Cancel marks the subscription as cancelled at period end.
func (s *Subscription) Cancel(now time.Time) {
	utcNow := now.UTC()
	s.Status = StatusCanceled
	s.CancelledAt = &utcNow
	s.AutoRenew = false
	s.UpdatedAt = utcNow
}

// Renew updates the current period end timestamp upon successful recurring payment.
func (s *Subscription) Renew(newPeriodEnd time.Time, now time.Time) {
	utcNow := now.UTC()
	s.Status = StatusActive
	s.CurrentPeriodEnd = newPeriodEnd.UTC()
	s.UpdatedAt = utcNow
}

// Expire transitions the subscription to expired status.
func (s *Subscription) Expire(now time.Time) {
	utcNow := now.UTC()
	s.Status = StatusExpired
	s.AutoRenew = false
	s.UpdatedAt = utcNow
}
