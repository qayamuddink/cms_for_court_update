-- Migration: 000003_create_articles.down.sql
ALTER TABLE court_cases DROP CONSTRAINT IF EXISTS fk_court_cases_judgment_article;
DROP TABLE IF EXISTS articles CASCADE;
