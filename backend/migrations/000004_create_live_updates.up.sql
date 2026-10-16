-- Migration: 000004_create_live_updates.up.sql
CREATE TABLE IF NOT EXISTS live_updates (
    id UUID PRIMARY KEY,
    court_case_id UUID NOT NULL REFERENCES court_cases(id) ON DELETE CASCADE,
    sequence_no BIGINT NOT NULL,
    update_type VARCHAR(32) NOT NULL,
    speaker_label TEXT,
    content TEXT NOT NULL,
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(32) NOT NULL DEFAULT 'published',
    corrects_update_id UUID REFERENCES live_updates(id) ON DELETE SET NULL,
    event_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    idempotency_key VARCHAR(128)
);

-- Required indexes for live_updates
CREATE UNIQUE INDEX idx_live_updates_case_seq ON live_updates (court_case_id, sequence_no);
CREATE INDEX idx_live_updates_case_seq_desc ON live_updates (court_case_id, sequence_no DESC);
CREATE UNIQUE INDEX idx_live_updates_case_idempotency ON live_updates (court_case_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_live_updates_reporter ON live_updates (reporter_id);
