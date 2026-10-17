-- Rollback Phase 10: Judgments, Citations, Headnotes, PDF Extraction & Audit Logging

DROP TABLE IF EXISTS judgment_downloads CASCADE;
DROP TABLE IF EXISTS judgment_pages CASCADE;
DROP TABLE IF EXISTS judgment_statutes CASCADE;
DROP TABLE IF EXISTS judgments CASCADE;
DROP TABLE IF EXISTS citation_counters CASCADE;
