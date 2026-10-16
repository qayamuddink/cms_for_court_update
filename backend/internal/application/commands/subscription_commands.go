package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/billing"
	"github.com/livelaw/backend/internal/domain/identity"
)

var (
	ErrInvalidWebhookSignature = errors.New("invalid webhook signature")
	ErrInvalidSubscriptionTier = errors.New("invalid subscription tier")
)

// SubscriptionUseCases orchestrates subscription billing lifecycle, webhook processing, and admin grants.
type SubscriptionUseCases struct {
	subRepo  ports.SubscriptionRepository
	userRepo ports.UserRepository
	gateway  ports.PaymentGateway
	idGen    ports.IDGenerator
	clock    ports.Clock
}

// NewSubscriptionUseCases creates a new SubscriptionUseCases instance.
func NewSubscriptionUseCases(
	subRepo ports.SubscriptionRepository,
	userRepo ports.UserRepository,
	gateway ports.PaymentGateway,
	idGen ports.IDGenerator,
	clock ports.Clock,
) *SubscriptionUseCases {
	return &SubscriptionUseCases{
		subRepo:  subRepo,
		userRepo: userRepo,
		gateway:  gateway,
		idGen:    idGen,
		clock:    clock,
	}
}

// ProcessPaymentWebhook handles incoming payment webhooks idempotently.
func (uc *SubscriptionUseCases) ProcessPaymentWebhook(
	ctx context.Context,
	rawPayload []byte,
	signature string,
	payload ports.PaymentWebhookPayload,
) error {
	if uc.gateway != nil {
		if !uc.gateway.VerifyWebhookSignature(ctx, rawPayload, signature) {
			return ErrInvalidWebhookSignature
		}
	}

	now := uc.clock.Now()

	switch payload.EventType {
	case "subscription.created", "payment.succeeded", "subscription.activated":
		startedAt := payload.StartedAt
		if startedAt.IsZero() {
			startedAt = now
		}
		currentPeriodEnd := payload.CurrentPeriodEnd
		if currentPeriodEnd.IsZero() {
			currentPeriodEnd = now.AddDate(0, 1, 0)
		}

		tier := billing.SubscriptionTier(payload.Tier)
		if tier == "" {
			tier = billing.TierPro
		}

		// Check if user already has an active subscription with a different provider id
		existingActive, err := uc.subRepo.GetActiveByUserID(ctx, payload.UserID)
		if err == nil && existingActive != nil && existingActive.ProviderSubscriptionID != payload.ProviderSubscriptionID {
			existingActive.Cancel(now)
			_ = uc.subRepo.Update(ctx, existingActive)
		}

		sub := &billing.Subscription{
			ID:                     uc.idGen.NewID(),
			UserID:                 payload.UserID,
			Tier:                   tier,
			Status:                 billing.StatusActive,
			StartedAt:              startedAt,
			CurrentPeriodEnd:       currentPeriodEnd,
			AutoRenew:              true,
			PaymentProvider:        "razorpay",
			ProviderSubscriptionID: payload.ProviderSubscriptionID,
			PriceAmountMinor:       payload.PriceAmountMinor,
			Currency:               payload.Currency,
			CreatedAt:              now,
			UpdatedAt:              now,
		}
		if sub.Currency == "" {
			sub.Currency = "INR"
		}

		return uc.subRepo.UpsertByProviderID(ctx, sub)

	case "subscription.renewed":
		sub, err := uc.subRepo.GetByProviderSubscriptionID(ctx, payload.ProviderSubscriptionID)
		if err != nil {
			if errors.Is(err, ports.ErrSubscriptionNotFound) {
				return nil
			}
			return err
		}

		periodEnd := payload.CurrentPeriodEnd
		if periodEnd.IsZero() {
			periodEnd = sub.CurrentPeriodEnd.AddDate(0, 1, 0)
		}
		sub.Renew(periodEnd, now)
		return uc.subRepo.Update(ctx, sub)

	case "subscription.cancelled":
		sub, err := uc.subRepo.GetByProviderSubscriptionID(ctx, payload.ProviderSubscriptionID)
		if err != nil {
			if errors.Is(err, ports.ErrSubscriptionNotFound) {
				return nil
			}
			return err
		}

		sub.Cancel(now)
		return uc.subRepo.Update(ctx, sub)

	default:
		// Unsupported or no-op event type
		return nil
	}
}

// GrantSubscriptionCommand carries admin input to grant a subscription plan manually.
type GrantSubscriptionCommand struct {
	AdminUserID  string
	TargetUserID string
	Tier         billing.SubscriptionTier
	DurationDays int
}

// GrantSubscription allows administrators to grant subscription access to a user manually.
func (uc *SubscriptionUseCases) GrantSubscription(ctx context.Context, cmd GrantSubscriptionCommand) (*billing.Subscription, error) {
	admin, err := uc.userRepo.GetByID(ctx, cmd.AdminUserID)
	if err != nil {
		return nil, err
	}
	if admin == nil || admin.SystemRole != identity.RoleAdmin {
		return nil, ErrForbidden
	}

	target, err := uc.userRepo.GetByID(ctx, cmd.TargetUserID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, ErrUserNotFound
	}

	if cmd.DurationDays <= 0 {
		cmd.DurationDays = 30
	}

	now := uc.clock.Now()

	// If target user already has an active subscription, expire it cleanly first
	active, err := uc.subRepo.GetActiveByUserID(ctx, target.ID)
	if err == nil && active != nil {
		active.Cancel(now)
		_ = uc.subRepo.Update(ctx, active)
	}

	sub := &billing.Subscription{
		ID:                     uc.idGen.NewID(),
		UserID:                 target.ID,
		Tier:                   cmd.Tier,
		Status:                 billing.StatusActive,
		StartedAt:              now,
		CurrentPeriodEnd:       now.AddDate(0, 0, cmd.DurationDays),
		AutoRenew:              false,
		PaymentProvider:        "manual",
		ProviderSubscriptionID: fmt.Sprintf("manual_%s", uc.idGen.NewID()),
		PriceAmountMinor:       0,
		Currency:               "INR",
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := uc.subRepo.Create(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// CancelSubscription allows administrators to cancel an active subscription.
func (uc *SubscriptionUseCases) CancelSubscription(ctx context.Context, adminUserID, subscriptionID string) error {
	admin, err := uc.userRepo.GetByID(ctx, adminUserID)
	if err != nil {
		return err
	}
	if admin == nil || admin.SystemRole != identity.RoleAdmin {
		return ErrForbidden
	}

	sub, err := uc.subRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return err
	}
	if sub == nil {
		return ports.ErrSubscriptionNotFound
	}

	now := uc.clock.Now()
	sub.Cancel(now)

	if err := uc.subRepo.Update(ctx, sub); err != nil {
		return err
	}

	if uc.gateway != nil && sub.PaymentProvider != "manual" {
		_ = uc.gateway.CancelSubscription(ctx, sub.ProviderSubscriptionID)
	}

	return nil
}
