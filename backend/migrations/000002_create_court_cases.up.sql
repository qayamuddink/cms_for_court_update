-- Migration: 000002_create_court_cases.up.sql
CREATE TABLE IF NOT EXISTS court_cases (
    id UUID PRIMARY KEY,
    court_name VARCHAR(128) NOT NULL,
    case_number VARCHAR(128) NOT NULL,
    case_type VARCHAR(64) NOT NULL,
    title TEXT NOT NULL,
    bench_judges JSONB NOT NULL DEFAULT '[]'::jsonb,
    presiding_judge TEXT NOT NULL,
    petitioner_parties JSONB NOT NULL DEFAULT '[]'::jsonb,
    respondent_parties JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'listed',
    next_hearing_at TIMESTAMPTZ,
    last_sequence_no BIGINT NOT NULL DEFAULT 0,
    is_live BOOLEAN NOT NULL DEFAULT false,
    judgment_article_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    CONSTRAINT uq_court_cases_court_number UNIQUE (court_name, case_number)
);

CREATE INDEX idx_court_cases_live_hearing ON court_cases (is_live, next_hearing_at) WHERE is_live = true;
CREATE INDEX idx_court_cases_status_updated_at ON court_cases (status, updated_at DESC);
