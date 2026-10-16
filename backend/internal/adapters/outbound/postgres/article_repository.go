package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
)

const articleColumns = `
	id, slug, title, subtitle, summary, body_blocks,
	article_type, access_level, status, author_id, editor_id,
	court_case_id, category, tags, court_name, citation,
	pdf_storage_key, pdf_size_bytes, pdf_checksum, published_at,
	version, created_at, updated_at,
	primary_section_id, word_count, reading_time_minutes,
	featured_image_key, featured_image_caption, featured_image_alt,
	meta_title, meta_description, canonical_url, social_image_key,
	is_top_story, top_story_order, scheduled_at, deleted_at,
	COALESCE(array_to_json(related_article_ids)::text, '[]')
`

// SQLArticleRepository implements ports.ArticleRepository using PostgreSQL.
type SQLArticleRepository struct {
	db *sql.DB
}

// NewSQLArticleRepository creates a new SQLArticleRepository instance.
func NewSQLArticleRepository(db *sql.DB) *SQLArticleRepository {
	return &SQLArticleRepository{db: db}
}

func formatUUIDArray(ids []string) string {
	if len(ids) == 0 {
		return "{}"
	}
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		t := strings.TrimSpace(id)
		if t != "" {
			clean = append(clean, t)
		}
	}
	if len(clean) == 0 {
		return "{}"
	}
	return "{" + strings.Join(clean, ",") + "}"
}

// Create inserts a new article into the database.
func (r *SQLArticleRepository) Create(ctx context.Context, a *content.Article) error {
	bodyBlocks := a.BodyBlocks
	if len(bodyBlocks) == 0 {
		bodyBlocks = []byte("[]")
	}

	tagsJSON, err := json.Marshal(a.Tags)
	if err != nil {
		tagsJSON = []byte("[]")
	}

	// Compute word count and reading time if not yet calculated
	if a.WordCount == 0 && len(bodyBlocks) > 0 {
		a.WordCount, a.ReadingTimeMinutes = content.CalculateWordCountAndReadingTime(a.Summary, bodyBlocks)
	}

	query := `
		INSERT INTO articles (
			id, slug, title, subtitle, summary, body_blocks,
			article_type, access_level, status, author_id, editor_id,
			court_case_id, category, tags, court_name, citation,
			pdf_storage_key, pdf_size_bytes, pdf_checksum, published_at,
			version, created_at, updated_at,
			primary_section_id, word_count, reading_time_minutes,
			featured_image_key, featured_image_caption, featured_image_alt,
			meta_title, meta_description, canonical_url, social_image_key,
			is_top_story, top_story_order, scheduled_at, deleted_at, related_article_ids
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, $20,
			$21, $22, $23,
			$24, $25, $26,
			$27, $28, $29,
			$30, $31, $32, $33,
			$34, $35, $36, $37, $38::uuid[]
		);
	`

	if a.Version <= 0 {
		a.Version = 1
	}

	_, err = r.db.ExecContext(ctx, query,
		a.ID,
		a.Slug,
		a.Title,
		a.Subtitle,
		a.Summary,
		bodyBlocks,
		string(a.ArticleType),
		string(a.AccessLevel),
		string(a.Status),
		a.AuthorID,
		a.EditorID,
		a.CourtCaseID,
		a.Category,
		tagsJSON,
		a.CourtName,
		a.Citation,
		a.PDFStorageKey,
		a.PDFSizeBytes,
		a.PDFChecksum,
		a.PublishedAt,
		a.Version,
		a.CreatedAt.UTC(),
		a.UpdatedAt.UTC(),
		a.PrimarySectionID,
		a.WordCount,
		a.ReadingTimeMinutes,
		a.FeaturedImageKey,
		a.FeaturedImageCaption,
		a.FeaturedImageAlt,
		a.MetaTitle,
		a.MetaDescription,
		a.CanonicalURL,
		a.SocialImageKey,
		a.IsTopStory,
		a.TopStoryOrder,
		a.ScheduledAt,
		a.DeletedAt,
		formatUUIDArray(a.RelatedArticleIDs),
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "slug") {
				return ports.ErrSlugAlreadyExists
			}
		}
		return fmt.Errorf("failed to insert article: %w", err)
	}

	return nil
}

// GetByID retrieves an article by its unique UUID.
func (r *SQLArticleRepository) GetByID(ctx context.Context, id string) (*content.Article, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM articles
		WHERE id = $1;
	`, articleColumns)

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanArticle(row)
}

// GetBySlug retrieves a non-deleted article by its URL-friendly slug.
func (r *SQLArticleRepository) GetBySlug(ctx context.Context, slug string) (*content.Article, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM articles
		WHERE slug = $1 AND deleted_at IS NULL;
	`, articleColumns)

	row := r.db.QueryRowContext(ctx, query, slug)
	return r.scanArticle(row)
}

// Update updates an article with optimistic locking, snapshots previous version into article_revisions, and records slug redirects.
func (r *SQLArticleRepository) Update(ctx context.Context, a *content.Article, expectedVersion int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update tx: %w", err)
	}
	defer tx.Rollback()

	// 1. Fetch current row to snapshot into revisions and detect slug changes
	var oldVersion int
	var oldTitle, oldSummary, oldSlug string
	var oldSubtitle *string
	var oldBody []byte
	var oldAuthorID string

	err = tx.QueryRowContext(ctx, `
		SELECT version, title, subtitle, summary, body_blocks, author_id, slug
		FROM articles
		WHERE id = $1
		FOR UPDATE;
	`, a.ID).Scan(&oldVersion, &oldTitle, &oldSubtitle, &oldSummary, &oldBody, &oldAuthorID, &oldSlug)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.ErrArticleNotFound
		}
		return fmt.Errorf("failed fetching article for revision snapshot: %w", err)
	}

	if oldVersion != expectedVersion {
		return content.ErrOptimisticLockConflict
	}

	// 2. Snapshot current state as a revision
	revQuery := `
		INSERT INTO article_revisions (
			id, article_id, version, title, subtitle, summary, body_blocks, author_id, created_at
		) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, NOW() AT TIME ZONE 'UTC')
		ON CONFLICT (article_id, version) DO NOTHING;
	`
	if _, err := tx.ExecContext(ctx, revQuery, a.ID, oldVersion, oldTitle, oldSubtitle, oldSummary, oldBody, oldAuthorID); err != nil {
		return fmt.Errorf("failed creating article revision snapshot: %w", err)
	}

	// 3. If slug changed, save slug redirect for 301 routing
	if oldSlug != "" && a.Slug != "" && oldSlug != a.Slug {
		redirQuery := `
			INSERT INTO article_slug_redirects (id, article_id, old_slug, created_at)
			VALUES (gen_random_uuid(), $1, $2, NOW() AT TIME ZONE 'UTC')
			ON CONFLICT (old_slug) DO UPDATE SET article_id = EXCLUDED.article_id;
		`
		if _, err := tx.ExecContext(ctx, redirQuery, a.ID, oldSlug); err != nil {
			return fmt.Errorf("failed saving slug redirect: %w", err)
		}
	}

	bodyBlocks := a.BodyBlocks
	if len(bodyBlocks) == 0 {
		bodyBlocks = []byte("[]")
	}

	tagsJSON, err := json.Marshal(a.Tags)
	if err != nil {
		tagsJSON = []byte("[]")
	}

	// Recalculate word count and reading time
	a.WordCount, a.ReadingTimeMinutes = content.CalculateWordCountAndReadingTime(a.Summary, bodyBlocks)

	query := `
		UPDATE articles SET
			slug = $1,
			title = $2,
			subtitle = $3,
			summary = $4,
			body_blocks = $5,
			article_type = $6,
			access_level = $7,
			status = $8,
			editor_id = $9,
			court_case_id = $10,
			category = $11,
			tags = $12,
			court_name = $13,
			citation = $14,
			pdf_storage_key = $15,
			pdf_size_bytes = $16,
			pdf_checksum = $17,
			published_at = $18,
			primary_section_id = $19,
			word_count = $20,
			reading_time_minutes = $21,
			featured_image_key = $22,
			featured_image_caption = $23,
			featured_image_alt = $24,
			meta_title = $25,
			meta_description = $26,
			canonical_url = $27,
			social_image_key = $28,
			is_top_story = $29,
			top_story_order = $30,
			scheduled_at = $31,
			related_article_ids = $32::uuid[],
			version = version + 1,
			updated_at = $33
		WHERE id = $34 AND version = $35;
	`

	res, err := tx.ExecContext(ctx, query,
		a.Slug,
		a.Title,
		a.Subtitle,
		a.Summary,
		bodyBlocks,
		string(a.ArticleType),
		string(a.AccessLevel),
		string(a.Status),
		a.EditorID,
		a.CourtCaseID,
		a.Category,
		tagsJSON,
		a.CourtName,
		a.Citation,
		a.PDFStorageKey,
		a.PDFSizeBytes,
		a.PDFChecksum,
		a.PublishedAt,
		a.PrimarySectionID,
		a.WordCount,
		a.ReadingTimeMinutes,
		a.FeaturedImageKey,
		a.FeaturedImageCaption,
		a.FeaturedImageAlt,
		a.MetaTitle,
		a.MetaDescription,
		a.CanonicalURL,
		a.SocialImageKey,
		a.IsTopStory,
		a.TopStoryOrder,
		a.ScheduledAt,
		formatUUIDArray(a.RelatedArticleIDs),
		time.Now().UTC(),
		a.ID,
		expectedVersion,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "slug") {
				return ports.ErrSlugAlreadyExists
			}
		}
		return fmt.Errorf("failed to update article: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return content.ErrOptimisticLockConflict
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed committing article update tx: %w", err)
	}

	a.Version = expectedVersion + 1
	return nil
}

// Delete removes an article from the database.
func (r *SQLArticleRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM articles WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete article: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ports.ErrArticleNotFound
	}
	return nil
}

// SoftDelete marks the article as deleted.
func (r *SQLArticleRepository) SoftDelete(ctx context.Context, id string, now time.Time) error {
	res, err := r.db.ExecContext(ctx, "UPDATE articles SET deleted_at = $1, updated_at = $1 WHERE id = $2 AND deleted_at IS NULL", now.UTC(), id)
	if err != nil {
		return fmt.Errorf("failed soft-deleting article: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ports.ErrArticleNotFound
	}
	return nil
}

// Restore restores a soft-deleted article.
func (r *SQLArticleRepository) Restore(ctx context.Context, id string, now time.Time) error {
	res, err := r.db.ExecContext(ctx, "UPDATE articles SET deleted_at = NULL, updated_at = $1 WHERE id = $2 AND deleted_at IS NOT NULL", now.UTC(), id)
	if err != nil {
		return fmt.Errorf("failed restoring article: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ports.ErrArticleNotFound
	}
	return nil
}

// CreateRevision explicitly stores a new article revision snapshot.
func (r *SQLArticleRepository) CreateRevision(ctx context.Context, rev *content.ArticleRevision) error {
	query := `
		INSERT INTO article_revisions (
			id, article_id, version, title, subtitle, summary, body_blocks, author_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (article_id, version) DO UPDATE SET
			title = EXCLUDED.title,
			subtitle = EXCLUDED.subtitle,
			summary = EXCLUDED.summary,
			body_blocks = EXCLUDED.body_blocks;
	`
	now := time.Now().UTC()
	if rev.CreatedAt.IsZero() {
		rev.CreatedAt = now
	}
	_, err := r.db.ExecContext(ctx, query,
		rev.ID,
		rev.ArticleID,
		rev.Version,
		rev.Title,
		rev.Subtitle,
		rev.Summary,
		rev.BodyBlocks,
		rev.AuthorID,
		rev.CreatedAt.UTC(),
	)
	return err
}

// ListRevisions lists all revision snapshots for an article, ordered by version DESC.
func (r *SQLArticleRepository) ListRevisions(ctx context.Context, articleID string) ([]content.ArticleRevision, error) {
	query := `
		SELECT id, article_id, version, title, subtitle, summary, body_blocks, author_id, created_at
		FROM article_revisions
		WHERE article_id = $1
		ORDER BY version DESC;
	`
	rows, err := r.db.QueryContext(ctx, query, articleID)
	if err != nil {
		return nil, fmt.Errorf("failed listing revisions: %w", err)
	}
	defer rows.Close()

	var revs []content.ArticleRevision
	for rows.Next() {
		var rev content.ArticleRevision
		if err := rows.Scan(
			&rev.ID,
			&rev.ArticleID,
			&rev.Version,
			&rev.Title,
			&rev.Subtitle,
			&rev.Summary,
			&rev.BodyBlocks,
			&rev.AuthorID,
			&rev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed scanning revision: %w", err)
		}
		revs = append(revs, rev)
	}
	if revs == nil {
		revs = []content.ArticleRevision{}
	}
	return revs, nil
}

// GetRevision retrieves a specific article revision snapshot.
func (r *SQLArticleRepository) GetRevision(ctx context.Context, articleID string, version int) (*content.ArticleRevision, error) {
	query := `
		SELECT id, article_id, version, title, subtitle, summary, body_blocks, author_id, created_at
		FROM article_revisions
		WHERE article_id = $1 AND version = $2;
	`
	var rev content.ArticleRevision
	err := r.db.QueryRowContext(ctx, query, articleID, version).Scan(
		&rev.ID,
		&rev.ArticleID,
		&rev.Version,
		&rev.Title,
		&rev.Subtitle,
		&rev.Summary,
		&rev.BodyBlocks,
		&rev.AuthorID,
		&rev.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, content.ErrRevisionNotFound
		}
		return nil, fmt.Errorf("failed scanning revision: %w", err)
	}
	return &rev, nil
}

// SaveSlugRedirect records an old slug mapping to an article for 301 redirects.
func (r *SQLArticleRepository) SaveSlugRedirect(ctx context.Context, articleID string, oldSlug string) error {
	query := `
		INSERT INTO article_slug_redirects (id, article_id, old_slug, created_at)
		VALUES (gen_random_uuid(), $1, $2, NOW() AT TIME ZONE 'UTC')
		ON CONFLICT (old_slug) DO UPDATE SET article_id = EXCLUDED.article_id;
	`
	_, err := r.db.ExecContext(ctx, query, articleID, oldSlug)
	return err
}

// GetSlugRedirect checks if an old slug exists for 301 redirection.
func (r *SQLArticleRepository) GetSlugRedirect(ctx context.Context, oldSlug string) (*content.SlugRedirect, error) {
	query := `
		SELECT id, article_id, old_slug, created_at
		FROM article_slug_redirects
		WHERE old_slug = $1;
	`
	var red content.SlugRedirect
	err := r.db.QueryRowContext(ctx, query, oldSlug).Scan(
		&red.ID,
		&red.ArticleID,
		&red.OldSlug,
		&red.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed retrieving slug redirect: %w", err)
	}
	return &red, nil
}

// GetScheduledDue retrieves scheduled articles due for publication.
func (r *SQLArticleRepository) GetScheduledDue(ctx context.Context, now time.Time, limit int) ([]content.Article, error) {
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf(`
		SELECT %s
		FROM articles
		WHERE status = 'scheduled'
		  AND scheduled_at <= $1
		  AND deleted_at IS NULL
		ORDER BY scheduled_at ASC
		LIMIT $2;
	`, articleColumns)

	rows, err := r.db.QueryContext(ctx, query, now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed querying due scheduled articles: %w", err)
	}
	defer rows.Close()

	var list []content.Article
	for rows.Next() {
		a, err := r.scanRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *a)
	}
	if list == nil {
		list = []content.Article{}
	}
	return list, nil
}

// PublishScheduled transitions a scheduled article to published.
func (r *SQLArticleRepository) PublishScheduled(ctx context.Context, articleID string, publishedAt time.Time) error {
	query := `
		UPDATE articles SET
			status = 'published',
			published_at = $1,
			updated_at = $2
		WHERE id = $3 AND status = 'scheduled';
	`
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, query, publishedAt.UTC(), now, articleID)
	if err != nil {
		return fmt.Errorf("failed publishing scheduled article: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ports.ErrArticleNotFound
	}
	return nil
}

// ListTopStories retrieves pinned top stories ordered by top_story_order ASC.
func (r *SQLArticleRepository) ListTopStories(ctx context.Context, limit int) ([]content.Article, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	query := fmt.Sprintf(`
		SELECT %s
		FROM articles
		WHERE is_top_story = true
		  AND status = 'published'
		  AND deleted_at IS NULL
		ORDER BY top_story_order ASC, published_at DESC
		LIMIT $1;
	`, articleColumns)

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed querying top stories: %w", err)
	}
	defer rows.Close()

	var list []content.Article
	for rows.Next() {
		a, err := r.scanRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *a)
	}
	if list == nil {
		list = []content.Article{}
	}
	return list, nil
}

// UpdateTopStoriesOrder updates pinned top stories orders.
func (r *SQLArticleRepository) UpdateTopStoriesOrder(ctx context.Context, items []content.TopStoryOrderItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed starting top stories tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE articles SET
			is_top_story = $1,
			top_story_order = $2,
			updated_at = NOW() AT TIME ZONE 'UTC'
		WHERE id = $3;
	`)
	if err != nil {
		return fmt.Errorf("failed preparing top story update: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		if _, err := stmt.ExecContext(ctx, item.IsTopStory, item.TopStoryOrder, item.ArticleID); err != nil {
			return fmt.Errorf("failed updating top story item %s: %w", item.ArticleID, err)
		}
	}

	return tx.Commit()
}

// GetRelatedArticles retrieves related articles by explicit IDs or same primary section.
func (r *SQLArticleRepository) GetRelatedArticles(ctx context.Context, articleID string, limit int) ([]content.Article, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	var relatedIDsJSON []byte
	var sectionID sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(array_to_json(related_article_ids)::text, '[]'), primary_section_id
		FROM articles
		WHERE id = $1;
	`, articleID).Scan(&relatedIDsJSON, &sectionID)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed querying article related metadata: %w", err)
	}

	var explicitIDs []string
	if len(relatedIDsJSON) > 0 {
		_ = json.Unmarshal(relatedIDsJSON, &explicitIDs)
	}

	var related []content.Article
	seenIDs := make(map[string]bool)
	seenIDs[articleID] = true

	if len(explicitIDs) > 0 {
		placeholders := make([]string, len(explicitIDs))
		args := make([]any, len(explicitIDs))
		for i, id := range explicitIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = id
		}
		query := fmt.Sprintf(`
			SELECT %s
			FROM articles
			WHERE id IN (%s) AND status = 'published' AND deleted_at IS NULL
			LIMIT %d;
		`, articleColumns, strings.Join(placeholders, ","), limit)

		rows, err := r.db.QueryContext(ctx, query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				a, err := r.scanRow(rows)
				if err == nil {
					related = append(related, *a)
					seenIDs[a.ID] = true
				}
			}
		}
	}

	// If needed, fill remaining slots from same primary_section
	if len(related) < limit && sectionID.Valid && sectionID.String != "" {
		remaining := limit - len(related)
		query := fmt.Sprintf(`
			SELECT %s
			FROM articles
			WHERE primary_section_id = $1 AND id != $2 AND status = 'published' AND deleted_at IS NULL
			ORDER BY published_at DESC
			LIMIT $3;
		`, articleColumns)

		rows, err := r.db.QueryContext(ctx, query, sectionID.String, articleID, remaining)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				a, err := r.scanRow(rows)
				if err == nil && !seenIDs[a.ID] {
					related = append(related, *a)
					seenIDs[a.ID] = true
				}
			}
		}
	}

	if related == nil {
		related = []content.Article{}
	}
	return related, nil
}

// ListFeed returns published articles ordered by published_at DESC, id DESC with keyset pagination.
func (r *SQLArticleRepository) ListFeed(ctx context.Context, filter ports.ArticleFilter) ([]content.Article, *string, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var conditions []string
	var args []any
	argIdx := 1

	// Feed always returns only published and non-deleted articles
	conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
	args = append(args, string(content.StatusPublished))
	argIdx++

	conditions = append(conditions, "deleted_at IS NULL")

	if filter.ArticleType != nil {
		conditions = append(conditions, fmt.Sprintf("article_type = $%d", argIdx))
		args = append(args, string(*filter.ArticleType))
		argIdx++
	}

	if filter.Category != nil && *filter.Category != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, *filter.Category)
		argIdx++
	}

	if filter.SectionID != nil && *filter.SectionID != "" {
		conditions = append(conditions, fmt.Sprintf("primary_section_id = $%d", argIdx))
		args = append(args, *filter.SectionID)
		argIdx++
	}

	if filter.SectionSlug != nil && *filter.SectionSlug != "" {
		conditions = append(conditions, fmt.Sprintf("primary_section_id = (SELECT id FROM sections WHERE slug = $%d)", argIdx))
		args = append(args, *filter.SectionSlug)
		argIdx++
	}

	if filter.IsTopStory != nil {
		conditions = append(conditions, fmt.Sprintf("is_top_story = $%d", argIdx))
		args = append(args, *filter.IsTopStory)
		argIdx++
	}

	if filter.Tag != nil && *filter.Tag != "" {
		tagJSON, _ := json.Marshal([]string{*filter.Tag})
		conditions = append(conditions, fmt.Sprintf("tags @> $%d", argIdx))
		args = append(args, string(tagJSON))
		argIdx++
	}

	if filter.CourtCaseID != nil && *filter.CourtCaseID != "" {
		conditions = append(conditions, fmt.Sprintf("court_case_id = $%d", argIdx))
		args = append(args, *filter.CourtCaseID)
		argIdx++
	}

	if filter.Cursor != nil && *filter.Cursor != "" {
		cursorTime, cursorID, err := ports.DecodeCursor(*filter.Cursor)
		if err == nil {
			conditions = append(conditions, fmt.Sprintf("(published_at < $%d OR (published_at = $%d AND id < $%d))", argIdx, argIdx, argIdx+1))
			args = append(args, cursorTime, cursorID)
			argIdx += 2
		}
	}

	whereClause := strings.Join(conditions, " AND ")
	query := fmt.Sprintf(`
		SELECT %s
		FROM articles
		WHERE %s
		ORDER BY published_at DESC, id DESC
		LIMIT $%d;
	`, articleColumns, whereClause, argIdx)

	args = append(args, limit+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed querying articles feed: %w", err)
	}
	defer rows.Close()

	var articles []content.Article
	for rows.Next() {
		a, err := r.scanRow(rows)
		if err != nil {
			return nil, nil, err
		}
		articles = append(articles, *a)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating article rows: %w", err)
	}

	var nextCursor *string
	if len(articles) > limit {
		lastItem := articles[limit-1]
		if lastItem.PublishedAt != nil {
			c := ports.EncodeCursor(*lastItem.PublishedAt, lastItem.ID)
			nextCursor = &c
		}
		articles = articles[:limit]
	}

	if articles == nil {
		articles = []content.Article{}
	}

	return articles, nextCursor, nil
}

// ListCMS queries articles for editorial back-office with status/author filtering.
func (r *SQLArticleRepository) ListCMS(ctx context.Context, filter ports.ArticleFilter) ([]content.Article, int, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var conditions []string
	var args []any
	argIdx := 1

	if !filter.IncludeDeleted {
		conditions = append(conditions, "deleted_at IS NULL")
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*filter.Status))
		argIdx++
	}

	if filter.ArticleType != nil {
		conditions = append(conditions, fmt.Sprintf("article_type = $%d", argIdx))
		args = append(args, string(*filter.ArticleType))
		argIdx++
	}

	if filter.AuthorID != nil {
		conditions = append(conditions, fmt.Sprintf("author_id = $%d", argIdx))
		args = append(args, *filter.AuthorID)
		argIdx++
	}

	if filter.SectionID != nil && *filter.SectionID != "" {
		conditions = append(conditions, fmt.Sprintf("primary_section_id = $%d", argIdx))
		args = append(args, *filter.SectionID)
		argIdx++
	}

	if filter.Category != nil && *filter.Category != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, *filter.Category)
		argIdx++
	}

	if filter.CourtCaseID != nil && *filter.CourtCaseID != "" {
		conditions = append(conditions, fmt.Sprintf("court_case_id = $%d", argIdx))
		args = append(args, *filter.CourtCaseID)
		argIdx++
	}

	if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
		searchTerm := "%" + strings.TrimSpace(*filter.Search) + "%"
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR summary ILIKE $%d)", argIdx, argIdx))
		args = append(args, searchTerm)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM articles %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed counting articles: %w", err)
	}

	offset := 0
	if filter.Cursor != nil && *filter.Cursor != "" {
		var off int
		if _, err := fmt.Sscanf(*filter.Cursor, "%d", &off); err == nil && off >= 0 {
			offset = off
		}
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM articles
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d;
	`, articleColumns, whereClause, argIdx, argIdx+1)

	queryArgs := append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed querying CMS articles: %w", err)
	}
	defer rows.Close()

	var articles []content.Article
	for rows.Next() {
		a, err := r.scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		articles = append(articles, *a)
	}

	if articles == nil {
		articles = []content.Article{}
	}

	return articles, total, nil
}

// LinkJudgmentToCase links a judgment summary article to a court case.
func (r *SQLArticleRepository) LinkJudgmentToCase(ctx context.Context, caseID string, articleID string) error {
	query := `UPDATE articles SET court_case_id = $1, updated_at = $2 WHERE id = $3;`
	_, err := r.db.ExecContext(ctx, query, caseID, time.Now().UTC(), articleID)
	if err != nil {
		return fmt.Errorf("failed linking judgment to case: %w", err)
	}
	return nil
}

// GetJudgmentByCaseID retrieves the judgment summary linked to a court case.
func (r *SQLArticleRepository) GetJudgmentByCaseID(ctx context.Context, caseID string) (*content.Article, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM articles
		WHERE court_case_id = $1 AND article_type = 'judgment_summary' AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1;
	`, articleColumns)

	row := r.db.QueryRowContext(ctx, query, caseID)
	return r.scanArticle(row)
}

// SlugExists checks if an article with the given slug already exists, excluding a specific ID.
func (r *SQLArticleRepository) SlugExists(ctx context.Context, slug string, excludeID string) (bool, error) {
	query := `SELECT COUNT(*) FROM articles WHERE slug = $1 AND id != $2;`
	var count int
	err := r.db.QueryRowContext(ctx, query, slug, excludeID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed checking slug existence: %w", err)
	}
	return count > 0, nil
}

func (r *SQLArticleRepository) scanArticle(row *sql.Row) (*content.Article, error) {
	var a content.Article
	var rawType, rawAccess, rawStatus string
	var tagsJSON, relatedJSON []byte

	err := row.Scan(
		&a.ID,
		&a.Slug,
		&a.Title,
		&a.Subtitle,
		&a.Summary,
		&a.BodyBlocks,
		&rawType,
		&rawAccess,
		&rawStatus,
		&a.AuthorID,
		&a.EditorID,
		&a.CourtCaseID,
		&a.Category,
		&tagsJSON,
		&a.CourtName,
		&a.Citation,
		&a.PDFStorageKey,
		&a.PDFSizeBytes,
		&a.PDFChecksum,
		&a.PublishedAt,
		&a.Version,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.PrimarySectionID,
		&a.WordCount,
		&a.ReadingTimeMinutes,
		&a.FeaturedImageKey,
		&a.FeaturedImageCaption,
		&a.FeaturedImageAlt,
		&a.MetaTitle,
		&a.MetaDescription,
		&a.CanonicalURL,
		&a.SocialImageKey,
		&a.IsTopStory,
		&a.TopStoryOrder,
		&a.ScheduledAt,
		&a.DeletedAt,
		&relatedJSON,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ports.ErrArticleNotFound
		}
		return nil, fmt.Errorf("failed scanning article: %w", err)
	}

	a.ArticleType = content.ArticleType(rawType)
	a.AccessLevel = content.AccessLevel(rawAccess)
	a.Status = content.ArticleStatus(rawStatus)
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &a.Tags)
	}
	if a.Tags == nil {
		a.Tags = []string{}
	}

	if len(relatedJSON) > 0 {
		_ = json.Unmarshal(relatedJSON, &a.RelatedArticleIDs)
	}
	if a.RelatedArticleIDs == nil {
		a.RelatedArticleIDs = []string{}
	}

	return &a, nil
}

func (r *SQLArticleRepository) scanRow(rows *sql.Rows) (*content.Article, error) {
	var a content.Article
	var rawType, rawAccess, rawStatus string
	var tagsJSON, relatedJSON []byte

	err := rows.Scan(
		&a.ID,
		&a.Slug,
		&a.Title,
		&a.Subtitle,
		&a.Summary,
		&a.BodyBlocks,
		&rawType,
		&rawAccess,
		&rawStatus,
		&a.AuthorID,
		&a.EditorID,
		&a.CourtCaseID,
		&a.Category,
		&tagsJSON,
		&a.CourtName,
		&a.Citation,
		&a.PDFStorageKey,
		&a.PDFSizeBytes,
		&a.PDFChecksum,
		&a.PublishedAt,
		&a.Version,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.PrimarySectionID,
		&a.WordCount,
		&a.ReadingTimeMinutes,
		&a.FeaturedImageKey,
		&a.FeaturedImageCaption,
		&a.FeaturedImageAlt,
		&a.MetaTitle,
		&a.MetaDescription,
		&a.CanonicalURL,
		&a.SocialImageKey,
		&a.IsTopStory,
		&a.TopStoryOrder,
		&a.ScheduledAt,
		&a.DeletedAt,
		&relatedJSON,
	)

	if err != nil {
		return nil, fmt.Errorf("failed scanning article row: %w", err)
	}

	a.ArticleType = content.ArticleType(rawType)
	a.AccessLevel = content.AccessLevel(rawAccess)
	a.Status = content.ArticleStatus(rawStatus)
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &a.Tags)
	}
	if a.Tags == nil {
		a.Tags = []string{}
	}

	if len(relatedJSON) > 0 {
		_ = json.Unmarshal(relatedJSON, &a.RelatedArticleIDs)
	}
	if a.RelatedArticleIDs == nil {
		a.RelatedArticleIDs = []string{}
	}

	return &a, nil
}
