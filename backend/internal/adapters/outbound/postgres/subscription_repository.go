package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/billing"
)

// SQLSubscriptionRepository implements ports.SubscriptionRepository using PostgreSQL.
type SQLSubscriptionRepository struct {
	db *sql.DB
}

// NewSQLSubscriptionRepository creates a new SQLSubscriptionRepository instance.
func NewSQLSubscriptionRepository(db *sql.DB) *SQLSubscriptionRepository {
	return &SQLSubscriptionRepository{db: db}
}

// Create inserts a new subscription.
func (r *SQLSubscriptionRepository) Create(ctx context.Context, s *billing.Subscription) error {
	query := `
		INSERT INTO subscriptions (
			id, user_id, tier, status, started_at, current_period_end,
			cancelled_at, auto_renew, payment_provider, provider_subscription_id,
			price_amount_minor, currency, seat_group_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15
		);
	`

	_, err := r.db.ExecContext(ctx, query,
		s.ID,
		s.UserID,
		string(s.Tier),
		string(s.Status),
		s.StartedAt.UTC(),
		s.CurrentPeriodEnd.UTC(),
		s.CancelledAt,
		s.AutoRenew,
		s.PaymentProvider,
		s.ProviderSubscriptionID,
		s.PriceAmountMinor,
		s.Currency,
		s.SeatGroupID,
		s.CreatedAt.UTC(),
		s.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed inserting subscription: %w", err)
	}
	return nil
}

// GetByID retrieves a subscription by primary key.
func (r *SQLSubscriptionRepository) GetByID(ctx context.Context, id string) (*billing.Subscription, error) {
	query := `
		SELECT
			id, user_id, tier, status, started_at, current_period_end,
			cancelled_at, auto_renew, payment_provider, provider_subscription_id,
			price_amount_minor, currency, seat_group_id, created_at, updated_at
		FROM subscriptions
		WHERE id = $1;
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanSubscription(row)
}

// GetByProviderSubscriptionID retrieves a subscription by external provider ID (for webhook idempotency).
func (r *SQLSubscriptionRepository) GetByProviderSubscriptionID(ctx context.Context, providerSubID string) (*billing.Subscription, error) {
	query := `
		SELECT
			id, user_id, tier, status, started_at, current_period_end,
			cancelled_at, auto_renew, payment_provider, provider_subscription_id,
			price_amount_minor, currency, seat_group_id, created_at, updated_at
		FROM subscriptions
		WHERE provider_subscription_id = $1;
	`
	row := r.db.QueryRowContext(ctx, query, providerSubID)
	return r.scanSubscription(row)
}

// GetActiveByUserID retrieves the current active or trialing subscription for a user.
func (r *SQLSubscriptionRepository) GetActiveByUserID(ctx context.Context, userID string) (*billing.Subscription, error) {
	query := `
		SELECT
			id, user_id, tier, status, started_at, current_period_end,
			cancelled_at, auto_renew, payment_provider, provider_subscription_id,
			price_amount_minor, currency, seat_group_id, created_at, updated_at
		FROM subscriptions
		WHERE user_id = $1
		  AND status IN ('active', 'trialing')
		  AND current_period_end > $2
		ORDER BY current_period_end DESC
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, userID, time.Now().UTC())
	return r.scanSubscription(row)
}

// Update updates an existing subscription record.
func (r *SQLSubscriptionRepository) Update(ctx context.Context, s *billing.Subscription) error {
	query := `
		UPDATE subscriptions SET
			tier = $1,
			status = $2,
			current_period_end = $3,
			cancelled_at = $4,
			auto_renew = $5,
			price_amount_minor = $6,
			currency = $7,
			updated_at = $8
		WHERE id = $9;
	`
	res, err := r.db.ExecContext(ctx, query,
		string(s.Tier),
		string(s.Status),
		s.CurrentPeriodEnd.UTC(),
		s.CancelledAt,
		s.AutoRenew,
		s.PriceAmountMinor,
		s.Currency,
		time.Now().UTC(),
		s.ID,
	)
	if err != nil {
		return fmt.Errorf("failed updating subscription: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ports.ErrSubscriptionNotFound
	}
	return nil
}

// UpsertByProviderID inserts or updates a subscription idempotently using provider_subscription_id.
func (r *SQLSubscriptionRepository) UpsertByProviderID(ctx context.Context, s *billing.Subscription) error {
	query := `
		INSERT INTO subscriptions (
			id, user_id, tier, status, started_at, current_period_end,
			cancelled_at, auto_renew, payment_provider, provider_subscription_id,
			price_amount_minor, currency, seat_group_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15
		)
		ON CONFLICT (provider_subscription_id) DO UPDATE SET
			tier = EXCLUDED.tier,
			status = EXCLUDED.status,
			current_period_end = EXCLUDED.current_period_end,
			cancelled_at = EXCLUDED.cancelled_at,
			auto_renew = EXCLUDED.auto_renew,
			price_amount_minor = EXCLUDED.price_amount_minor,
			currency = EXCLUDED.currency,
			updated_at = EXCLUDED.updated_at;
	`
	_, err := r.db.ExecContext(ctx, query,
		s.ID,
		s.UserID,
		string(s.Tier),
		string(s.Status),
		s.StartedAt.UTC(),
		s.CurrentPeriodEnd.UTC(),
		s.CancelledAt,
		s.AutoRenew,
		s.PaymentProvider,
		s.ProviderSubscriptionID,
		s.PriceAmountMinor,
		s.Currency,
		s.SeatGroupID,
		s.CreatedAt.UTC(),
		s.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed upserting subscription: %w", err)
	}
	return nil
}

// List queries subscriptions for admin management.
func (r *SQLSubscriptionRepository) List(ctx context.Context, limit, offset int, status *billing.SubscriptionStatus) ([]billing.Subscription, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	whereClause := "1=1"
	var args []any
	if status != nil {
		whereClause = "status = $1"
		args = append(args, string(*status))
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM subscriptions WHERE %s;", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed counting subscriptions: %w", err)
	}

	// Query rows
	query := fmt.Sprintf(`
		SELECT
			id, user_id, tier, status, started_at, current_period_end,
			cancelled_at, auto_renew, payment_provider, provider_subscription_id,
			price_amount_minor, currency, seat_group_id, created_at, updated_at
		FROM subscriptions
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d;
	`, whereClause, len(args)+1, len(args)+2)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed querying subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []billing.Subscription
	for rows.Next() {
		s, err := r.scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		subs = append(subs, *s)
	}

	if subs == nil {
		subs = []billing.Subscription{}
	}

	return subs, total, nil
}

func (r *SQLSubscriptionRepository) scanSubscription(row *sql.Row) (*billing.Subscription, error) {
	var s billing.Subscription
	var rawTier, rawStatus string

	err := row.Scan(
		&s.ID,
		&s.UserID,
		&rawTier,
		&rawStatus,
		&s.StartedAt,
		&s.CurrentPeriodEnd,
		&s.CancelledAt,
		&s.AutoRenew,
		&s.PaymentProvider,
		&s.ProviderSubscriptionID,
		&s.PriceAmountMinor,
		&s.Currency,
		&s.SeatGroupID,
		&s.CreatedAt,
		&s.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ports.ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("failed scanning subscription: %w", err)
	}

	s.Tier = billing.SubscriptionTier(rawTier)
	s.Status = billing.SubscriptionStatus(rawStatus)
	return &s, nil
}

func (r *SQLSubscriptionRepository) scanRow(rows *sql.Rows) (*billing.Subscription, error) {
	var s billing.Subscription
	var rawTier, rawStatus string

	err := rows.Scan(
		&s.ID,
		&s.UserID,
		&rawTier,
		&rawStatus,
		&s.StartedAt,
		&s.CurrentPeriodEnd,
		&s.CancelledAt,
		&s.AutoRenew,
		&s.PaymentProvider,
		&s.ProviderSubscriptionID,
		&s.PriceAmountMinor,
		&s.Currency,
		&s.SeatGroupID,
		&s.CreatedAt,
		&s.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed scanning subscription row: %w", err)
	}

	s.Tier = billing.SubscriptionTier(rawTier)
	s.Status = billing.SubscriptionStatus(rawStatus)
	return &s, nil
}
