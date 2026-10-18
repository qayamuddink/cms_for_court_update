-- Migration: 000013_search_analytics.up.sql
-- Phase 11: Search Analytics, Query Tracking, and Zero-Result Monitoring

CREATE TABLE IF NOT EXISTS search_analytics (
    id UUID PRIMARY KEY,
    query TEXT NOT NULL,
    hit_count INT NOT NULL DEFAULT 0,
    is_zero_result BOOLEAN NOT NULL DEFAULT false,
    has_filters BOOLEAN NOT NULL DEFAULT false,
    ip_hash VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')
);

CREATE INDEX IF NOT EXISTS idx_search_analytics_query ON search_analytics (query);
CREATE INDEX IF NOT EXISTS idx_search_analytics_zero ON search_analytics (is_zero_result, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_search_analytics_created ON search_analytics (created_at DESC);
