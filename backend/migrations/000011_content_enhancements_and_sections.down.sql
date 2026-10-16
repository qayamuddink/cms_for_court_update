-- Migration: 000011_content_enhancements_and_sections.down.sql

-- 1. Drop redirects table
DROP TABLE IF EXISTS article_slug_redirects CASCADE;

-- 2. Drop revisions table
DROP TABLE IF EXISTS article_revisions CASCADE;

-- 3. Drop indexes
DROP INDEX IF EXISTS idx_articles_section_feed;
DROP INDEX IF EXISTS idx_articles_top_stories;
DROP INDEX IF EXISTS idx_articles_scheduled;
DROP INDEX IF EXISTS idx_articles_deleted_at;

-- 4. Revert article columns
ALTER TABLE articles
    DROP COLUMN IF EXISTS primary_section_id,
    DROP COLUMN IF EXISTS word_count,
    DROP COLUMN IF EXISTS reading_time_minutes,
    DROP COLUMN IF EXISTS featured_image_key,
    DROP COLUMN IF EXISTS featured_image_caption,
    DROP COLUMN IF EXISTS featured_image_alt,
    DROP COLUMN IF EXISTS meta_title,
    DROP COLUMN IF EXISTS meta_description,
    DROP COLUMN IF EXISTS canonical_url,
    DROP COLUMN IF EXISTS social_image_key,
    DROP COLUMN IF EXISTS is_top_story,
    DROP COLUMN IF EXISTS top_story_order,
    DROP COLUMN IF EXISTS scheduled_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS related_article_ids;

-- 5. Drop sections table
DROP TABLE IF EXISTS sections CASCADE;
