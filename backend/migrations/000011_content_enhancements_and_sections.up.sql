-- Migration: 000011_content_enhancements_and_sections.up.sql

-- 1. Create sections table for taxonomy
CREATE TABLE IF NOT EXISTS sections (
    id UUID PRIMARY KEY,
    slug VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    display_order INT NOT NULL DEFAULT 0,
    parent_id UUID REFERENCES sections(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')
);

CREATE INDEX IF NOT EXISTS idx_sections_display ON sections (display_order ASC, name ASC);
CREATE INDEX IF NOT EXISTS idx_sections_slug ON sections (slug);

-- 2. Extend articles table with long-form, SEO, scheduling, and editorial safety fields
ALTER TABLE articles
    ADD COLUMN IF NOT EXISTS primary_section_id UUID REFERENCES sections(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS word_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS reading_time_minutes INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS featured_image_key TEXT,
    ADD COLUMN IF NOT EXISTS featured_image_caption TEXT,
    ADD COLUMN IF NOT EXISTS featured_image_alt TEXT,
    ADD COLUMN IF NOT EXISTS meta_title TEXT,
    ADD COLUMN IF NOT EXISTS meta_description TEXT,
    ADD COLUMN IF NOT EXISTS canonical_url TEXT,
    ADD COLUMN IF NOT EXISTS social_image_key TEXT,
    ADD COLUMN IF NOT EXISTS is_top_story BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS top_story_order INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS related_article_ids UUID[] NOT NULL DEFAULT '{}';

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_articles_section_feed ON articles (primary_section_id, published_at DESC) WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_articles_top_stories ON articles (is_top_story, top_story_order ASC, published_at DESC) WHERE status = 'published' AND is_top_story = true AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_articles_scheduled ON articles (status, scheduled_at ASC) WHERE status = 'scheduled' AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_articles_deleted_at ON articles (deleted_at) WHERE deleted_at IS NOT NULL;

-- 3. Create article_revisions table for audit and side-by-side diff
CREATE TABLE IF NOT EXISTS article_revisions (
    id UUID PRIMARY KEY,
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    version INT NOT NULL,
    title TEXT NOT NULL,
    subtitle TEXT,
    summary TEXT NOT NULL,
    body_blocks JSONB NOT NULL DEFAULT '[]'::jsonb,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    CONSTRAINT uq_article_revisions_version UNIQUE(article_id, version)
);

CREATE INDEX IF NOT EXISTS idx_article_revisions_article ON article_revisions (article_id, version DESC);

-- 4. Create article_slug_redirects table for 301 SEO redirects
CREATE TABLE IF NOT EXISTS article_slug_redirects (
    id UUID PRIMARY KEY,
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    old_slug VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')
);

CREATE INDEX IF NOT EXISTS idx_slug_redirects_lookup ON article_slug_redirects (old_slug);

-- 5. Seed default editorial sections
INSERT INTO sections (id, slug, name, description, display_order, is_active)
VALUES
    ('0192e240-0000-7000-8000-000000000001', 'supreme-court', 'Supreme Court', 'Apex court rulings, hearings, and orders', 10, true),
    ('0192e240-0000-7000-8000-000000000002', 'high-courts', 'High Courts', 'High Courts jurisprudence across Indian states', 20, true),
    ('0192e240-0000-7000-8000-000000000003', 'delhi-high-court', 'Delhi High Court', 'Delhi High Court benches and orders', 21, true),
    ('0192e240-0000-7000-8000-000000000004', 'bombay-high-court', 'Bombay High Court', 'Bombay High Court benches and orders', 22, true),
    ('0192e240-0000-7000-8000-000000000005', 'tax', 'Taxation Law', 'Direct, Indirect Tax, and GST tribunal rulings', 30, true),
    ('0192e240-0000-7000-8000-000000000006', 'ibc', 'IBC & Insolvency', 'NCLT and NCLAT corporate insolvency proceedings', 40, true),
    ('0192e240-0000-7000-8000-000000000007', 'arbitration', 'Arbitration', 'Domestic and international commercial arbitration', 50, true),
    ('0192e240-0000-7000-8000-000000000008', 'environmental', 'Environmental Law', 'NGT and environmental constitutional matters', 60, true),
    ('0192e240-0000-7000-8000-000000000009', 'consumer', 'Consumer Protection', 'Consumer commissions and product liability', 70, true),
    ('0192e240-0000-7000-8000-000000000010', 'foreign-international', 'Foreign & International', 'Comparative constitutional and international law', 80, true),
    ('0192e240-0000-7000-8000-000000000011', 'law-schools', 'Law Schools', 'Moot courts, admissions, and legal research', 90, true),
    ('0192e240-0000-7000-8000-000000000012', 'law-firms', 'Law Firms', 'Law firm moves, appointments, and deals', 100, true)
ON CONFLICT (slug) DO NOTHING;
