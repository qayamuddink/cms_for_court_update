-- Migration: 000005_create_subscriptions.up.sql
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tier VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    cancelled_at TIMESTAMPTZ,
    auto_renew BOOLEAN NOT NULL DEFAULT true,
    payment_provider VARCHAR(32) NOT NULL,
    provider_subscription_id VARCHAR(128) NOT NULL UNIQUE,
    price_amount_minor BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    seat_group_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Required indexes for subscriptions
CREATE INDEX idx_subscriptions_user_status_period ON subscriptions (user_id, status, current_period_end DESC);
CREATE UNIQUE INDEX idx_subscriptions_user_active_trial ON subscriptions (user_id) WHERE status IN ('active', 'trialing');
