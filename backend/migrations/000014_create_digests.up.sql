-- Phase 12: Digests and Round-ups schema

CREATE TABLE IF NOT EXISTS digests (
    id UUID PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    court VARCHAR(255) NOT NULL,
    court_code VARCHAR(32) NOT NULL,
    digest_type VARCHAR(32) NOT NULL DEFAULT 'digest', -- 'digest', 'round_up'
    period VARCHAR(32) NOT NULL, -- 'weekly', 'monthly', 'quarterly', 'annual', 'thematic'
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    citation_range_start INT,
    citation_range_end INT,
    citation_prefix VARCHAR(64), -- e.g. "2026 LiveLaw (SC)"
    is_part BOOLEAN NOT NULL DEFAULT FALSE,
    part_number INT NOT NULL DEFAULT 1,
    total_parts INT NOT NULL DEFAULT 1,
    intro TEXT,
    conclusion TEXT,
    article_id UUID REFERENCES articles(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft', -- 'draft', 'published', 'archived'
    access_level VARCHAR(32) NOT NULL DEFAULT 'premium', -- 'free', 'premium'
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS digest_items (
    id UUID PRIMARY KEY,
    digest_id UUID NOT NULL REFERENCES digests(id) ON DELETE CASCADE,
    judgment_id UUID NOT NULL REFERENCES judgments(id) ON DELETE CASCADE,
    display_order INT NOT NULL DEFAULT 0,
    custom_title VARCHAR(255),
    blurb TEXT,
    commentary TEXT,
    subject_tag VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_digest_item UNIQUE (digest_id, judgment_id)
);

CREATE INDEX IF NOT EXISTS idx_digests_court_period ON digests(court_code, period, period_start, period_end);
CREATE INDEX IF NOT EXISTS idx_digests_status ON digests(status);
CREATE INDEX IF NOT EXISTS idx_digests_article_id ON digests(article_id);
CREATE INDEX IF NOT EXISTS idx_digest_items_digest_order ON digest_items(digest_id, display_order ASC);
CREATE INDEX IF NOT EXISTS idx_digest_items_judgment_id ON digest_items(judgment_id);
