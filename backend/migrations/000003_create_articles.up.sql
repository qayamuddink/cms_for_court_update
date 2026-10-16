-- Migration: 000003_create_articles.up.sql
CREATE TABLE IF NOT EXISTS articles (
    id UUID PRIMARY KEY,
    slug VARCHAR(255) NOT NULL UNIQUE,
    title TEXT NOT NULL,
    subtitle TEXT,
    summary TEXT NOT NULL,
    body_blocks JSONB NOT NULL DEFAULT '[]'::jsonb,
    article_type VARCHAR(32) NOT NULL,
    access_level VARCHAR(32) NOT NULL DEFAULT 'free',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    editor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    court_case_id UUID REFERENCES court_cases(id) ON DELETE SET NULL,
    category VARCHAR(64) NOT NULL,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    court_name VARCHAR(128),
    citation VARCHAR(128),
    pdf_storage_key TEXT,
    pdf_size_bytes BIGINT,
    pdf_checksum VARCHAR(64),
    published_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Add foreign key reference from court_cases to articles for judgment_article_id
ALTER TABLE court_cases
    ADD CONSTRAINT fk_court_cases_judgment_article
    FOREIGN KEY (judgment_article_id)
    REFERENCES articles(id)
    ON DELETE SET NULL;

-- Required indexes for articles
CREATE INDEX idx_articles_published_feed ON articles (status, published_at DESC, id DESC) WHERE status = 'published';
CREATE INDEX idx_articles_type_published ON articles (article_type, published_at DESC);
CREATE INDEX idx_articles_court_case_published ON articles (court_case_id, published_at DESC);
CREATE INDEX idx_articles_author_created ON articles (author_id, created_at DESC);
