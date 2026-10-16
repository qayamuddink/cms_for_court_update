-- Migration: 000006_create_case_assignments.up.sql
CREATE TABLE IF NOT EXISTS case_assignments (
    id UUID PRIMARY KEY,
    court_case_id UUID NOT NULL REFERENCES court_cases(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    CONSTRAINT uq_case_assignments_case_user UNIQUE (court_case_id, user_id)
);

CREATE INDEX idx_case_assignments_user ON case_assignments (user_id);
