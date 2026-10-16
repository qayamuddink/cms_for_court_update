-- Migration: 000008_create_case_follows.up.sql
CREATE TABLE IF NOT EXISTS case_follows (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    court_case_id UUID NOT NULL REFERENCES court_cases(id) ON DELETE CASCADE,
    notify_on JSONB NOT NULL DEFAULT '["verdict", "order"]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    CONSTRAINT uq_case_follows_user_case UNIQUE (user_id, court_case_id)
);

-- Required index for case_follows
CREATE INDEX idx_case_follows_case_user ON case_follows (court_case_id, user_id);
