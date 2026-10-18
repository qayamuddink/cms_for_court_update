package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/digest"
)

var (
	ErrDigestNotFound = errors.New("digest not found")
)

// SQLDigestRepository implements ports.DigestRepository using PostgreSQL.
type SQLDigestRepository struct {
	db *sql.DB
}

// NewSQLDigestRepository creates a new SQLDigestRepository.
func NewSQLDigestRepository(db *sql.DB) *SQLDigestRepository {
	return &SQLDigestRepository{db: db}
}

// Create inserts a new digest and any accompanying items.
func (r *SQLDigestRepository) Create(ctx context.Context, d *digest.Digest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO digests (
			id, title, slug, court, court_code, digest_type, period,
			period_start, period_end, citation_range_start, citation_range_end,
			citation_prefix, is_part, part_number, total_parts,
			intro, conclusion, article_id, status, access_level,
			created_by, published_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18, $19, $20,
			$21, $22, $23, $24
		)
	`
	_, err = tx.ExecContext(ctx, query,
		d.ID, d.Title, d.Slug, d.Court, d.CourtCode, string(d.DigestType), string(d.Period),
		d.PeriodStart, d.PeriodEnd, d.CitationRangeStart, d.CitationRangeEnd,
		d.CitationPrefix, d.IsPart, d.PartNumber, d.TotalParts,
		d.Intro, d.Conclusion, d.ArticleID, string(d.Status), string(d.AccessLevel),
		d.CreatedBy, d.PublishedAt, d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert digest: %w", err)
	}

	if len(d.Items) > 0 {
		itemQuery := `
			INSERT INTO digest_items (
				id, digest_id, judgment_id, display_order, custom_title, blurb, commentary, subject_tag, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`
		for _, item := range d.Items {
			_, err = tx.ExecContext(ctx, itemQuery,
				item.ID, d.ID, item.JudgmentID, item.DisplayOrder,
				item.CustomTitle, item.Blurb, item.Commentary, item.SubjectTag, item.CreatedAt,
			)
			if err != nil {
				return fmt.Errorf("failed to insert digest item: %w", err)
			}
		}
	}

	return tx.Commit()
}

// GetByID retrieves a digest with its items by ID.
func (r *SQLDigestRepository) GetByID(ctx context.Context, id string) (*digest.Digest, error) {
	query := `
		SELECT
			id, title, slug, court, court_code, digest_type, period,
			period_start, period_end, citation_range_start, citation_range_end,
			citation_prefix, is_part, part_number, total_parts,
			intro, conclusion, article_id, status, access_level,
			created_by, published_at, created_at, updated_at
		FROM digests
		WHERE id = $1
	`
	d, err := r.scanDigest(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDigestNotFound
		}
		return nil, fmt.Errorf("failed to query digest: %w", err)
	}

	items, err := r.GetItems(ctx, d.ID)
	if err == nil {
		d.Items = items
	}

	return d, nil
}

// GetBySlug retrieves a digest with its items by slug.
func (r *SQLDigestRepository) GetBySlug(ctx context.Context, slug string) (*digest.Digest, error) {
	query := `
		SELECT
			id, title, slug, court, court_code, digest_type, period,
			period_start, period_end, citation_range_start, citation_range_end,
			citation_prefix, is_part, part_number, total_parts,
			intro, conclusion, article_id, status, access_level,
			created_by, published_at, created_at, updated_at
		FROM digests
		WHERE slug = $1
	`
	d, err := r.scanDigest(r.db.QueryRowContext(ctx, query, strings.TrimSpace(slug)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDigestNotFound
		}
		return nil, fmt.Errorf("failed to query digest by slug: %w", err)
	}

	items, err := r.GetItems(ctx, d.ID)
	if err == nil {
		d.Items = items
	}

	return d, nil
}

// Update updates an existing digest and optionally replaces items.
func (r *SQLDigestRepository) Update(ctx context.Context, d *digest.Digest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		UPDATE digests SET
			title = $2, slug = $3, court = $4, court_code = $5, digest_type = $6, period = $7,
			period_start = $8, period_end = $9, citation_range_start = $10, citation_range_end = $11,
			citation_prefix = $12, is_part = $13, part_number = $14, total_parts = $15,
			intro = $16, conclusion = $17, article_id = $18, status = $19, access_level = $20,
			published_at = $21, updated_at = $22
		WHERE id = $1
	`
	res, err := tx.ExecContext(ctx, query,
		d.ID, d.Title, d.Slug, d.Court, d.CourtCode, string(d.DigestType), string(d.Period),
		d.PeriodStart, d.PeriodEnd, d.CitationRangeStart, d.CitationRangeEnd,
		d.CitationPrefix, d.IsPart, d.PartNumber, d.TotalParts,
		d.Intro, d.Conclusion, d.ArticleID, string(d.Status), string(d.AccessLevel),
		d.PublishedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update digest: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrDigestNotFound
	}

	if len(d.Items) > 0 {
		_, _ = tx.ExecContext(ctx, `DELETE FROM digest_items WHERE digest_id = $1`, d.ID)
		itemQuery := `
			INSERT INTO digest_items (
				id, digest_id, judgment_id, display_order, custom_title, blurb, commentary, subject_tag, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`
		for _, item := range d.Items {
			_, err = tx.ExecContext(ctx, itemQuery,
				item.ID, d.ID, item.JudgmentID, item.DisplayOrder,
				item.CustomTitle, item.Blurb, item.Commentary, item.SubjectTag, item.CreatedAt,
			)
			if err != nil {
				return fmt.Errorf("failed to update digest items: %w", err)
			}
		}
	}

	return tx.Commit()
}

// Delete removes a digest and cascades associated items.
func (r *SQLDigestRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM digests WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete digest: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrDigestNotFound
	}
	return nil
}

// List retrieves digests matching filters.
func (r *SQLDigestRepository) List(ctx context.Context, f ports.DigestFilter) ([]digest.Digest, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if f.CourtCode != nil && *f.CourtCode != "" {
		conditions = append(conditions, fmt.Sprintf("court_code = $%d", argIdx))
		args = append(args, strings.ToUpper(*f.CourtCode))
		argIdx++
	}
	if f.Period != nil && *f.Period != "" {
		conditions = append(conditions, fmt.Sprintf("period = $%d", argIdx))
		args = append(args, string(*f.Period))
		argIdx++
	}
	if f.DigestType != nil && *f.DigestType != "" {
		conditions = append(conditions, fmt.Sprintf("digest_type = $%d", argIdx))
		args = append(args, string(*f.DigestType))
		argIdx++
	}
	if f.Status != nil && *f.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*f.Status))
		argIdx++
	}
	if f.Year != nil && *f.Year > 0 {
		conditions = append(conditions, fmt.Sprintf("EXTRACT(YEAR FROM period_start) = $%d", argIdx))
		args = append(args, *f.Year)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM digests %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count digests: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT
			id, title, slug, court, court_code, digest_type, period,
			period_start, period_end, citation_range_start, citation_range_end,
			citation_prefix, is_part, part_number, total_parts,
			intro, conclusion, article_id, status, access_level,
			created_by, published_at, created_at, updated_at
		FROM digests
		%s
		ORDER BY period_start DESC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query digests: %w", err)
	}
	defer rows.Close()

	var result []digest.Digest
	for rows.Next() {
		d, err := r.scanDigest(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan digest row: %w", err)
		}
		result = append(result, *d)
	}

	return result, total, nil
}

// SaveItems replaces the items inside a digest.
func (r *SQLDigestRepository) SaveItems(ctx context.Context, digestID string, items []digest.DigestItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `DELETE FROM digest_items WHERE digest_id = $1`, digestID)
	if err != nil {
		return fmt.Errorf("failed to clear old items: %w", err)
	}

	query := `
		INSERT INTO digest_items (
			id, digest_id, judgment_id, display_order, custom_title, blurb, commentary, subject_tag, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	for _, item := range items {
		_, err = tx.ExecContext(ctx, query,
			item.ID, digestID, item.JudgmentID, item.DisplayOrder,
			item.CustomTitle, item.Blurb, item.Commentary, item.SubjectTag, item.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert digest item: %w", err)
		}
	}

	return tx.Commit()
}

// GetItems retrieves the full list of items for a digest, populated with judgment details.
func (r *SQLDigestRepository) GetItems(ctx context.Context, digestID string) ([]digest.DigestItem, error) {
	query := `
		SELECT
			di.id, di.digest_id, di.judgment_id, di.display_order,
			di.custom_title, di.blurb, di.commentary, di.subject_tag, di.created_at,
			j.case_title, j.citation, j.citation_number, j.judgment_date, j.bench, j.headnote, j.held
		FROM digest_items di
		JOIN judgments j ON di.judgment_id = j.id
		WHERE di.digest_id = $1
		ORDER BY di.display_order ASC, di.created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, digestID)
	if err != nil {
		return nil, fmt.Errorf("failed to query digest items: %w", err)
	}
	defer rows.Close()

	var items []digest.DigestItem
	for rows.Next() {
		var it digest.DigestItem
		var bench []string
		var customTitle, blurb, commentary, subjectTag sql.NullString
		var citation sql.NullString
		var citationNumber sql.NullInt64
		var judgmentDate time.Time

		err := rows.Scan(
			&it.ID, &it.DigestID, &it.JudgmentID, &it.DisplayOrder,
			&customTitle, &blurb, &commentary, &subjectTag, &it.CreatedAt,
			&it.CaseTitle, &citation, &citationNumber, &judgmentDate, pq.Array(&bench), &it.Headnote, &it.Held,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan digest item: %w", err)
		}

		if customTitle.Valid {
			it.CustomTitle = customTitle.String
		}
		if blurb.Valid {
			it.Blurb = blurb.String
		}
		if commentary.Valid {
			it.Commentary = commentary.String
		}
		if subjectTag.Valid {
			it.SubjectTag = subjectTag.String
		}
		if citation.Valid {
			it.Citation = citation.String
		}
		if citationNumber.Valid {
			num := int(citationNumber.Int64)
			it.CitationNumber = &num
		}
		it.JudgmentDate = judgmentDate.Format("2006-01-02")
		it.Bench = bench

		// Also fetch statutes cited for each judgment
		statutesQuery := `SELECT statute_name, sections_cited FROM judgment_statutes WHERE judgment_id = $1`
		statuteRows, sErr := r.db.QueryContext(ctx, statutesQuery, it.JudgmentID)
		if sErr == nil {
			var statutes []string
			for statuteRows.Next() {
				var name string
				var sections []string
				if err := statuteRows.Scan(&name, pq.Array(&sections)); err == nil {
					if len(sections) > 0 {
						for _, sec := range sections {
							statutes = append(statutes, fmt.Sprintf("%s - %s", name, sec))
						}
					} else {
						statutes = append(statutes, name)
					}
				}
			}
			statuteRows.Close()
			it.StatutesCited = statutes
		}

		items = append(items, it)
	}

	return items, nil
}

type digestScanner interface {
	Scan(dest ...any) error
}

func (r *SQLDigestRepository) scanDigest(scanner digestScanner) (*digest.Digest, error) {
	var d digest.Digest
	var citationPrefix, intro, conclusion, articleID, createdBy sql.NullString
	var citationRangeStart, citationRangeEnd sql.NullInt64
	var publishedAt sql.NullTime

	err := scanner.Scan(
		&d.ID, &d.Title, &d.Slug, &d.Court, &d.CourtCode, &d.DigestType, &d.Period,
		&d.PeriodStart, &d.PeriodEnd, &citationRangeStart, &citationRangeEnd,
		&citationPrefix, &d.IsPart, &d.PartNumber, &d.TotalParts,
		&intro, &conclusion, &articleID, &d.Status, &d.AccessLevel,
		&createdBy, &publishedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if citationPrefix.Valid {
		d.CitationPrefix = citationPrefix.String
	}
	if intro.Valid {
		d.Intro = intro.String
	}
	if conclusion.Valid {
		d.Conclusion = conclusion.String
	}
	if articleID.Valid {
		d.ArticleID = &articleID.String
	}
	if createdBy.Valid {
		d.CreatedBy = &createdBy.String
	}
	if citationRangeStart.Valid {
		val := int(citationRangeStart.Int64)
		d.CitationRangeStart = &val
	}
	if citationRangeEnd.Valid {
		val := int(citationRangeEnd.Int64)
		d.CitationRangeEnd = &val
	}
	if publishedAt.Valid {
		t := publishedAt.Time.UTC()
		d.PublishedAt = &t
	}

	return &d, nil
}
