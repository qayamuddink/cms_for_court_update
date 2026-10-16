-- Migration: 000009_create_device_tokens.up.sql
CREATE TABLE IF NOT EXISTS device_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(16) NOT NULL,
    push_token TEXT NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')
);

CREATE UNIQUE INDEX idx_device_tokens_user_platform_token ON device_tokens (user_id, platform, push_token);
CREATE INDEX idx_device_tokens_user_id ON device_tokens (user_id);
