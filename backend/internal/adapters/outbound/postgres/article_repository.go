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

// SQLArticleRepository implements ports.ArticleRepository using PostgreSQL.
type SQLArticleRepository struct {
	db *sql.DB
}

// NewSQLArticleRepository creates a new SQLArticleRepository instance.
func NewSQLArticleRepository(db *sql.DB) *SQLArticleRepository {
	return &SQLArticleRepository{db: db}
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

	query := `
		INSERT INTO articles (
			id, slug, title, subtitle, summary, body_blocks,
			article_type, access_level, status, author_id, editor_id,
			court_case_id, category, tags, court_name, citation,
			pdf_storage_key, pdf_size_bytes, pdf_checksum, published_at,
			version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, $20,
			$21, $22, $23
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
	query := `
		SELECT
			id, slug, title, subtitle, summary, body_blocks,
			article_type, access_level, status, author_id, editor_id,
			court_case_id, category, tags, court_name, citation,
			pdf_storage_key, pdf_size_bytes, pdf_checksum, published_at,
			version, created_at, updated_at
		FROM articles
		WHERE id = $1;
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanArticle(row)
}

// GetBySlug retrieves an article by its URL-friendly slug.
func (r *SQLArticleRepository) GetBySlug(ctx context.Context, slug string) (*content.Article, error) {
	query := `
		SELECT
			id, slug, title, subtitle, summary, body_blocks,
			article_type, access_level, status, author_id, editor_id,
			court_case_id, category, tags, court_name, citation,
			pdf_storage_key, pdf_size_bytes, pdf_checksum, published_at,
			version, created_at, updated_at
		FROM articles
		WHERE slug = $1;
	`

	row := r.db.QueryRowContext(ctx, query, slug)
	return r.scanArticle(row)
}

// Update updates an article with optimistic locking check against expectedVersion.
func (r *SQLArticleRepository) Update(ctx context.Context, a *content.Article, expectedVersion int) error {
	bodyBlocks := a.BodyBlocks
	if len(bodyBlocks) == 0 {
		bodyBlocks = []byte("[]")
	}

	tagsJSON, err := json.Marshal(a.Tags)
	if err != nil {
		tagsJSON = []byte("[]")
	}

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
			version = version + 1,
			updated_at = $19
		WHERE id = $20 AND version = $21;
	`

	res, err := r.db.ExecContext(ctx, query,
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
		// Verify if article exists to distinguish 404 from optimistic lock conflict
		var currentVersion int
		checkErr := r.db.QueryRowContext(ctx, "SELECT version FROM articles WHERE id = $1", a.ID).Scan(&currentVersion)
		if errors.Is(checkErr, sql.ErrNoRows) {
			return ports.ErrArticleNotFound
		}
		return content.ErrOptimisticLockConflict
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

// ListFeed returns published articles ordered by published_at DESC, id DESC with keyset pagination.
func (r *SQLArticleRepository) ListFeed(ctx context.Context, filter ports.ArticleFilter) ([]content.Article, *string, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var conditions []string
	var args []any
	argIdx := 1

	// Feed always returns only published articles
	conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
	args = append(args, string(content.StatusPublished))
	argIdx++

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
		SELECT
			id, slug, title, subtitle, summary, body_blocks,
			article_type, access_level, status, author_id, editor_id,
			court_case_id, category, tags, court_name, citation,
			pdf_storage_key, pdf_size_bytes, pdf_checksum, published_at,
			version, created_at, updated_at
		FROM articles
		WHERE %s
		ORDER BY published_at DESC, id DESC
		LIMIT $%d;
	`, whereClause, argIdx)

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

	if filter.AuthorID != nil && *filter.AuthorID != "" {
		conditions = append(conditions, fmt.Sprintf("author_id = $%d", argIdx))
		args = append(args, *filter.AuthorID)
		argIdx++
	}

	if filter.Category != nil && *filter.Category != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, *filter.Category)
		argIdx++
	}

	if filter.Search != nil && *filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR summary ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+*filter.Search+"%")
		argIdx++
	}

	whereClause := "1=1"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM articles WHERE %s;", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed counting cms articles: %w", err)
	}

	// Select items
	query := fmt.Sprintf(`
		SELECT
			id, slug, title, subtitle, summary, body_blocks,
			article_type, access_level, status, author_id, editor_id,
			court_case_id, category, tags, court_name, citation,
			pdf_storage_key, pdf_size_bytes, pdf_checksum, published_at,
			version, created_at, updated_at
		FROM articles
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d;
	`, whereClause, argIdx)

	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed querying cms articles: %w", err)
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

// LinkJudgmentToCase links a judgment summary article to a court case in a single transaction.
func (r *SQLArticleRepository) LinkJudgmentToCase(ctx context.Context, caseID, articleID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Update article
	_, err = tx.ExecContext(ctx, "UPDATE articles SET court_case_id = $1, article_type = 'judgment_summary' WHERE id = $2", caseID, articleID)
	if err != nil {
		return fmt.Errorf("failed updating article court_case_id: %w", err)
	}

	// Update court_case
	_, err = tx.ExecContext(ctx, "UPDATE court_cases SET judgment_article_id = $1 WHERE id = $2", articleID, caseID)
	if err != nil {
		return fmt.Errorf("failed updating court_case judgment_article_id: %w", err)
	}

	return tx.Commit()
}

// GetJudgmentByCaseID retrieves the judgment summary article linked to a court case.
func (r *SQLArticleRepository) GetJudgmentByCaseID(ctx context.Context, caseID string) (*content.Article, error) {
	query := `
		SELECT
			a.id, a.slug, a.title, a.subtitle, a.summary, a.body_blocks,
			a.article_type, a.access_level, a.status, a.author_id, a.editor_id,
			a.court_case_id, a.category, a.tags, a.court_name, a.citation,
			a.pdf_storage_key, a.pdf_size_bytes, a.pdf_checksum, a.published_at,
			a.version, a.created_at, a.updated_at
		FROM articles a
		INNER JOIN court_cases c ON c.judgment_article_id = a.id
		WHERE c.id = $1;
	`

	row := r.db.QueryRowContext(ctx, query, caseID)
	return r.scanArticle(row)
}

// SlugExists checks whether an article with the slug already exists (excluding excludeID).
func (r *SQLArticleRepository) SlugExists(ctx context.Context, slug string, excludeID string) (bool, error) {
	query := "SELECT EXISTS(SELECT 1 FROM articles WHERE slug = $1 AND id != $2);"
	var exists bool
	err := r.db.QueryRowContext(ctx, query, slug, excludeID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *SQLArticleRepository) scanArticle(row *sql.Row) (*content.Article, error) {
	var a content.Article
	var rawType, rawAccess, rawStatus string
	var tagsJSON []byte

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

	return &a, nil
}

func (r *SQLArticleRepository) scanRow(rows *sql.Rows) (*content.Article, error) {
	var a content.Article
	var rawType, rawAccess, rawStatus string
	var tagsJSON []byte

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

	return &a, nil
}
