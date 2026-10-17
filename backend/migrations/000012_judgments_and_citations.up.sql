-- Phase 10: Judgments, Citations, Headnotes, PDF Extraction & Audit Logging

CREATE TABLE IF NOT EXISTS citation_counters (
    court_code VARCHAR(32) NOT NULL,
    year INT NOT NULL,
    current_number INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (court_code, year)
);

CREATE TABLE IF NOT EXISTS judgments (
    id UUID PRIMARY KEY,
    citation VARCHAR(128) UNIQUE,
    citation_year INT,
    citation_court_code VARCHAR(32),
    citation_number INT,
    case_title VARCHAR(512) NOT NULL,
    case_number VARCHAR(128) NOT NULL,
    court VARCHAR(255) NOT NULL,
    court_code VARCHAR(32) NOT NULL,
    bench TEXT[] NOT NULL DEFAULT '{}',
    judgment_date DATE NOT NULL,
    decision_date DATE,
    appellant VARCHAR(512) NOT NULL,
    respondent VARCHAR(512) NOT NULL,
    advocates TEXT[] NOT NULL DEFAULT '{}',
    subject_tags TEXT[] NOT NULL DEFAULT '{}',
    issues TEXT,
    held TEXT,
    headnote TEXT,
    summary TEXT NOT NULL,
    key_takeaways TEXT[] NOT NULL DEFAULT '{}',
    court_case_id UUID REFERENCES court_cases(id) ON DELETE SET NULL,
    article_id UUID REFERENCES articles(id) ON DELETE SET NULL,
    access_level VARCHAR(32) NOT NULL DEFAULT 'free',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    pdf_storage_key VARCHAR(512),
    pdf_filename VARCHAR(255),
    pdf_size_bytes BIGINT,
    pdf_checksum VARCHAR(64),
    pdf_page_count INT,
    pdf_thumbnail_key VARCHAR(512),
    pdf_preview_key VARCHAR(512),
    extraction_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_judgments_court_year ON judgments(court_code, citation_year);
CREATE INDEX IF NOT EXISTS idx_judgments_status_date ON judgments(status, judgment_date DESC);
CREATE INDEX IF NOT EXISTS idx_judgments_checksum ON judgments(pdf_checksum) WHERE pdf_checksum IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_judgments_extraction ON judgments(extraction_status);
CREATE INDEX IF NOT EXISTS idx_judgments_case ON judgments(court_case_id);

CREATE TABLE IF NOT EXISTS judgment_statutes (
    id UUID PRIMARY KEY,
    judgment_id UUID NOT NULL REFERENCES judgments(id) ON DELETE CASCADE,
    statute_name VARCHAR(255) NOT NULL,
    sections_cited TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_judgment_statutes_judgment ON judgment_statutes(judgment_id);
CREATE INDEX IF NOT EXISTS idx_judgment_statutes_name ON judgment_statutes(statute_name);

CREATE TABLE IF NOT EXISTS judgment_pages (
    id UUID PRIMARY KEY,
    judgment_id UUID NOT NULL REFERENCES judgments(id) ON DELETE CASCADE,
    page_number INT NOT NULL,
    text_content TEXT NOT NULL,
    has_ocr BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (judgment_id, page_number)
);

CREATE INDEX IF NOT EXISTS idx_judgment_pages_judgment ON judgment_pages(judgment_id);

CREATE TABLE IF NOT EXISTS judgment_downloads (
    id UUID PRIMARY KEY,
    judgment_id UUID NOT NULL REFERENCES judgments(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ip_address VARCHAR(64) NOT NULL,
    user_agent TEXT NOT NULL,
    downloaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    watermarked BOOLEAN NOT NULL DEFAULT false
);

ALTER TABLE judgment_downloads ADD COLUMN IF NOT EXISTS watermarked BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_judgment_downloads_judgment ON judgment_downloads(judgment_id);
CREATE INDEX IF NOT EXISTS idx_judgment_downloads_user ON judgment_downloads(user_id);
