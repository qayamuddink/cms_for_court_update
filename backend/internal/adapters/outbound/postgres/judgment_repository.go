package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/judgment"
)

var (
	ErrJudgmentNotFound = errors.New("judgment not found")
)

// SQLJudgmentRepository implements ports.JudgmentRepository with PostgreSQL.
type SQLJudgmentRepository struct {
	db *sql.DB
}

// NewSQLJudgmentRepository creates a new SQLJudgmentRepository.
func NewSQLJudgmentRepository(db *sql.DB) *SQLJudgmentRepository {
	return &SQLJudgmentRepository{db: db}
}

// Create inserts a new judgment and its statutes in a transaction.
func (r *SQLJudgmentRepository) Create(ctx context.Context, j *judgment.Judgment) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO judgments (
			id, citation, citation_year, citation_court_code, citation_number,
			case_title, case_number, court, court_code, bench,
			judgment_date, decision_date, appellant, respondent, advocates,
			subject_tags, issues, held, headnote, summary, key_takeaways,
			court_case_id, article_id, access_level, status,
			pdf_storage_key, pdf_filename, pdf_size_bytes, pdf_checksum,
			pdf_page_count, pdf_thumbnail_key, pdf_preview_key,
			extraction_status, created_by, published_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21,
			$22, $23, $24, $25,
			$26, $27, $28, $29,
			$30, $31, $32,
			$33, $34, $35, $36, $37
		)
	`

	bench := j.Bench
	if bench == nil {
		bench = []string{}
	}
	advocates := j.Advocates
	if advocates == nil {
		advocates = []string{}
	}
	subjectTags := j.SubjectTags
	if subjectTags == nil {
		subjectTags = []string{}
	}
	keyTakeaways := j.KeyTakeaways
	if keyTakeaways == nil {
		keyTakeaways = []string{}
	}

	_, err = tx.ExecContext(ctx, query,
		j.ID, j.Citation, j.CitationYear, j.CitationCourtCode, j.CitationNumber,
		j.CaseTitle, j.CaseNumber, j.Court, j.CourtCode, pq.Array(bench),
		j.JudgmentDate, j.DecisionDate, j.Appellant, j.Respondent, pq.Array(advocates),
		pq.Array(subjectTags), j.Issues, j.Held, j.Headnote, j.Summary, pq.Array(keyTakeaways),
		j.CourtCaseID, j.ArticleID, string(j.AccessLevel), string(j.Status),
		j.PDFStorageKey, j.PDFFilename, j.PDFSizeBytes, j.PDFChecksum,
		j.PDFPageCount, j.PDFThumbnailKey, j.PDFPreviewKey,
		string(j.ExtractionStatus), j.CreatedBy, j.PublishedAt, j.CreatedAt, j.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert judgment: %w", err)
	}

	if len(j.StatutesCited) > 0 {
		statuteQuery := `
			INSERT INTO judgment_statutes (id, judgment_id, statute_name, sections_cited, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`
		for _, s := range j.StatutesCited {
			_, err = tx.ExecContext(ctx, statuteQuery, s.ID, j.ID, s.StatuteName, pq.Array(s.SectionsCited), j.CreatedAt)
			if err != nil {
				return fmt.Errorf("failed to insert judgment statute: %w", err)
			}
		}
	}

	return tx.Commit()
}

// GetByID retrieves a judgment with statutes by ID.
func (r *SQLJudgmentRepository) GetByID(ctx context.Context, id string) (*judgment.Judgment, error) {
	query := `
		SELECT
			id, citation, citation_year, citation_court_code, citation_number,
			case_title, case_number, court, court_code, bench,
			judgment_date, decision_date, appellant, respondent, advocates,
			subject_tags, issues, held, headnote, summary, key_takeaways,
			court_case_id, article_id, access_level, status,
			pdf_storage_key, pdf_filename, pdf_size_bytes, pdf_checksum,
			pdf_page_count, pdf_thumbnail_key, pdf_preview_key,
			extraction_status, created_by, published_at, created_at, updated_at
		FROM judgments
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	j, err := r.scanJudgment(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJudgmentNotFound
		}
		return nil, fmt.Errorf("failed to query judgment: %w", err)
	}

	statutes, err := r.GetStatutes(ctx, j.ID)
	if err == nil {
		j.StatutesCited = statutes
	}

	return j, nil
}

// GetByCitation retrieves a judgment by its official unique citation.
func (r *SQLJudgmentRepository) GetByCitation(ctx context.Context, citation string) (*judgment.Judgment, error) {
	query := `
		SELECT
			id, citation, citation_year, citation_court_code, citation_number,
			case_title, case_number, court, court_code, bench,
			judgment_date, decision_date, appellant, respondent, advocates,
			subject_tags, issues, held, headnote, summary, key_takeaways,
			court_case_id, article_id, access_level, status,
			pdf_storage_key, pdf_filename, pdf_size_bytes, pdf_checksum,
			pdf_page_count, pdf_thumbnail_key, pdf_preview_key,
			extraction_status, created_by, published_at, created_at, updated_at
		FROM judgments
		WHERE LOWER(citation) = LOWER($1)
	`
	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(citation))
	j, err := r.scanJudgment(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJudgmentNotFound
		}
		return nil, fmt.Errorf("failed to query judgment by citation: %w", err)
	}

	statutes, err := r.GetStatutes(ctx, j.ID)
	if err == nil {
		j.StatutesCited = statutes
	}

	return j, nil
}

// GetByCaseID retrieves the judgment linked to a court case.
func (r *SQLJudgmentRepository) GetByCaseID(ctx context.Context, caseID string) (*judgment.Judgment, error) {
	query := `
		SELECT
			id, citation, citation_year, citation_court_code, citation_number,
			case_title, case_number, court, court_code, bench,
			judgment_date, decision_date, appellant, respondent, advocates,
			subject_tags, issues, held, headnote, summary, key_takeaways,
			court_case_id, article_id, access_level, status,
			pdf_storage_key, pdf_filename, pdf_size_bytes, pdf_checksum,
			pdf_page_count, pdf_thumbnail_key, pdf_preview_key,
			extraction_status, created_by, published_at, created_at, updated_at
		FROM judgments
		WHERE court_case_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, caseID)
	j, err := r.scanJudgment(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJudgmentNotFound
		}
		return nil, fmt.Errorf("failed to query judgment by case: %w", err)
	}

	statutes, err := r.GetStatutes(ctx, j.ID)
	if err == nil {
		j.StatutesCited = statutes
	}

	return j, nil
}

// GetByChecksum finds an existing judgment by its PDF SHA-256 checksum.
func (r *SQLJudgmentRepository) GetByChecksum(ctx context.Context, checksum string) (*judgment.Judgment, error) {
	query := `
		SELECT
			id, citation, citation_year, citation_court_code, citation_number,
			case_title, case_number, court, court_code, bench,
			judgment_date, decision_date, appellant, respondent, advocates,
			subject_tags, issues, held, headnote, summary, key_takeaways,
			court_case_id, article_id, access_level, status,
			pdf_storage_key, pdf_filename, pdf_size_bytes, pdf_checksum,
			pdf_page_count, pdf_thumbnail_key, pdf_preview_key,
			extraction_status, created_by, published_at, created_at, updated_at
		FROM judgments
		WHERE pdf_checksum = $1
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(checksum))
	j, err := r.scanJudgment(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // none found
		}
		return nil, fmt.Errorf("failed to query judgment by checksum: %w", err)
	}
	return j, nil
}

// Update updates an existing judgment and replaces statutes.
func (r *SQLJudgmentRepository) Update(ctx context.Context, j *judgment.Judgment) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		UPDATE judgments SET
			citation = $2, citation_year = $3, citation_court_code = $4, citation_number = $5,
			case_title = $6, case_number = $7, court = $8, court_code = $9, bench = $10,
			judgment_date = $11, decision_date = $12, appellant = $13, respondent = $14, advocates = $15,
			subject_tags = $16, issues = $17, held = $18, headnote = $19, summary = $20, key_takeaways = $21,
			court_case_id = $22, article_id = $23, access_level = $24, status = $25,
			pdf_storage_key = $26, pdf_filename = $27, pdf_size_bytes = $28, pdf_checksum = $29,
			pdf_page_count = $30, pdf_thumbnail_key = $31, pdf_preview_key = $32,
			extraction_status = $33, published_at = $34, updated_at = $35
		WHERE id = $1
	`

	bench := j.Bench
	if bench == nil {
		bench = []string{}
	}
	advocates := j.Advocates
	if advocates == nil {
		advocates = []string{}
	}
	subjectTags := j.SubjectTags
	if subjectTags == nil {
		subjectTags = []string{}
	}
	keyTakeaways := j.KeyTakeaways
	if keyTakeaways == nil {
		keyTakeaways = []string{}
	}

	res, err := tx.ExecContext(ctx, query,
		j.ID, j.Citation, j.CitationYear, j.CitationCourtCode, j.CitationNumber,
		j.CaseTitle, j.CaseNumber, j.Court, j.CourtCode, pq.Array(bench),
		j.JudgmentDate, j.DecisionDate, j.Appellant, j.Respondent, pq.Array(advocates),
		pq.Array(subjectTags), j.Issues, j.Held, j.Headnote, j.Summary, pq.Array(keyTakeaways),
		j.CourtCaseID, j.ArticleID, string(j.AccessLevel), string(j.Status),
		j.PDFStorageKey, j.PDFFilename, j.PDFSizeBytes, j.PDFChecksum,
		j.PDFPageCount, j.PDFThumbnailKey, j.PDFPreviewKey,
		string(j.ExtractionStatus), j.PublishedAt, j.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update judgment: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrJudgmentNotFound
	}

	// Update statutes
	_, _ = tx.ExecContext(ctx, `DELETE FROM judgment_statutes WHERE judgment_id = $1`, j.ID)
	if len(j.StatutesCited) > 0 {
		statuteQuery := `
			INSERT INTO judgment_statutes (id, judgment_id, statute_name, sections_cited, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`
		for _, s := range j.StatutesCited {
			_, err = tx.ExecContext(ctx, statuteQuery, s.ID, j.ID, s.StatuteName, pq.Array(s.SectionsCited), j.UpdatedAt)
			if err != nil {
				return fmt.Errorf("failed to update judgment statute: %w", err)
			}
		}
	}

	return tx.Commit()
}

// List queries judgments matching filters with total count.
func (r *SQLJudgmentRepository) List(ctx context.Context, f ports.JudgmentFilter) ([]judgment.Judgment, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if f.CourtCode != nil && *f.CourtCode != "" {
		conditions = append(conditions, fmt.Sprintf("court_code = $%d", argIdx))
		args = append(args, strings.ToUpper(*f.CourtCode))
		argIdx++
	}
	if f.Year != nil && *f.Year > 0 {
		conditions = append(conditions, fmt.Sprintf("(citation_year = $%d OR EXTRACT(YEAR FROM judgment_date) = $%d)", argIdx, argIdx))
		args = append(args, *f.Year)
		argIdx++
	}
	if f.Status != nil && *f.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*f.Status))
		argIdx++
	}
	if f.AccessLevel != nil && *f.AccessLevel != "" {
		conditions = append(conditions, fmt.Sprintf("access_level = $%d", argIdx))
		args = append(args, string(*f.AccessLevel))
		argIdx++
	}
	if f.CaseID != nil && *f.CaseID != "" {
		conditions = append(conditions, fmt.Sprintf("court_case_id = $%d", argIdx))
		args = append(args, *f.CaseID)
		argIdx++
	}
	if f.SearchQuery != nil && *f.SearchQuery != "" {
		pattern := "%" + strings.TrimSpace(*f.SearchQuery) + "%"
		conditions = append(conditions, fmt.Sprintf("(case_title ILIKE $%d OR case_number ILIKE $%d OR citation ILIKE $%d OR summary ILIKE $%d)", argIdx, argIdx, argIdx, argIdx))
		args = append(args, pattern)
		argIdx++
	}
	if f.StatuteName != nil && *f.StatuteName != "" {
		conditions = append(conditions, fmt.Sprintf("id IN (SELECT judgment_id FROM judgment_statutes WHERE statute_name ILIKE $%d)", argIdx))
		args = append(args, "%"+*f.StatuteName+"%")
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM judgments %s", whereClause)
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count judgments: %w", err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	listQuery := fmt.Sprintf(`
		SELECT
			id, citation, citation_year, citation_court_code, citation_number,
			case_title, case_number, court, court_code, bench,
			judgment_date, decision_date, appellant, respondent, advocates,
			subject_tags, issues, held, headnote, summary, key_takeaways,
			court_case_id, article_id, access_level, status,
			pdf_storage_key, pdf_filename, pdf_size_bytes, pdf_checksum,
			pdf_page_count, pdf_thumbnail_key, pdf_preview_key,
			extraction_status, created_by, published_at, created_at, updated_at
		FROM judgments
		%s
		ORDER BY judgment_date DESC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list judgments: %w", err)
	}
	defer rows.Close()

	var results []judgment.Judgment
	for rows.Next() {
		j, err := r.scanJudgment(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, *j)
	}

	return results, total, nil
}

// AllocateCitation atomically increments and reserves a unique sequential citation number per court per year.
// PostgreSQL row-level locks on ON CONFLICT DO UPDATE guarantee gapless uniqueness even under parallel load.
func (r *SQLJudgmentRepository) AllocateCitation(
	ctx context.Context,
	courtCode string,
	year int,
	brandPrefix string,
) (string, int, error) {
	courtCode = strings.ToUpper(strings.TrimSpace(courtCode))
	if courtCode == "" {
		courtCode = "SC"
	}
	if year <= 0 {
		year = time.Now().UTC().Year()
	}

	query := `
		INSERT INTO citation_counters (court_code, year, current_number, updated_at)
		VALUES ($1, $2, 1, NOW())
		ON CONFLICT (court_code, year) DO UPDATE
		SET current_number = citation_counters.current_number + 1,
		    updated_at = NOW()
		RETURNING current_number;
	`

	var currentNumber int
	err := r.db.QueryRowContext(ctx, query, courtCode, year).Scan(&currentNumber)
	if err != nil {
		return "", 0, fmt.Errorf("failed to allocate atomic citation number: %w", err)
	}

	citation := judgment.FormatCitation(year, brandPrefix, courtCode, currentNumber)
	return citation, currentNumber, nil
}

// AddStatutes adds structured statutes cited to a judgment.
func (r *SQLJudgmentRepository) AddStatutes(ctx context.Context, judgmentID string, statutes []judgment.StatuteCited) error {
	query := `
		INSERT INTO judgment_statutes (id, judgment_id, statute_name, sections_cited, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`
	for _, s := range statutes {
		_, err := r.db.ExecContext(ctx, query, s.ID, judgmentID, s.StatuteName, pq.Array(s.SectionsCited))
		if err != nil {
			return fmt.Errorf("failed to insert statute: %w", err)
		}
	}
	return nil
}

// GetStatutes loads structured statutes cited for a judgment.
func (r *SQLJudgmentRepository) GetStatutes(ctx context.Context, judgmentID string) ([]judgment.StatuteCited, error) {
	query := `
		SELECT id, judgment_id, statute_name, sections_cited
		FROM judgment_statutes
		WHERE judgment_id = $1
		ORDER BY statute_name ASC
	`
	rows, err := r.db.QueryContext(ctx, query, judgmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query statutes: %w", err)
	}
	defer rows.Close()

	var res []judgment.StatuteCited
	for rows.Next() {
		var s judgment.StatuteCited
		if err := rows.Scan(&s.ID, &s.JudgmentID, &s.StatuteName, pq.Array(&s.SectionsCited)); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	return res, nil
}

// SavePages inserts or updates extracted text per page for search indexing.
func (r *SQLJudgmentRepository) SavePages(ctx context.Context, judgmentID string, pages []judgment.JudgmentPage) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO judgment_pages (id, judgment_id, page_number, text_content, has_ocr, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (judgment_id, page_number) DO UPDATE
		SET text_content = EXCLUDED.text_content,
		    has_ocr = EXCLUDED.has_ocr,
		    created_at = EXCLUDED.created_at
	`
	for _, p := range pages {
		_, err := tx.ExecContext(ctx, query, p.ID, judgmentID, p.PageNumber, p.TextContent, p.HasOCR, p.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to save judgment page %d: %w", p.PageNumber, err)
		}
	}

	return tx.Commit()
}

// GetPages retrieves all extracted pages for a judgment.
func (r *SQLJudgmentRepository) GetPages(ctx context.Context, judgmentID string) ([]judgment.JudgmentPage, error) {
	query := `
		SELECT id, judgment_id, page_number, text_content, has_ocr, created_at
		FROM judgment_pages
		WHERE judgment_id = $1
		ORDER BY page_number ASC
	`
	rows, err := r.db.QueryContext(ctx, query, judgmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query judgment pages: %w", err)
	}
	defer rows.Close()

	var pages []judgment.JudgmentPage
	for rows.Next() {
		var p judgment.JudgmentPage
		if err := rows.Scan(&p.ID, &p.JudgmentID, &p.PageNumber, &p.TextContent, &p.HasOCR, &p.CreatedAt); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, nil
}

// LogDownload logs a download event to the audit table.
func (r *SQLJudgmentRepository) LogDownload(ctx context.Context, d judgment.JudgmentDownload) error {
	id := d.ID
	if id == "" {
		id = uuid.New().String()
	}
	query := `
		INSERT INTO judgment_downloads (id, judgment_id, user_id, ip_address, user_agent, downloaded_at, watermarked)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query, id, d.JudgmentID, d.UserID, d.IPAddress, d.UserAgent, d.DownloadedAt, d.Watermarked)
	if err != nil {
		return fmt.Errorf("failed to log judgment download: %w", err)
	}
	return nil
}

// ListPendingExtractions returns judgments awaiting PDF extraction / OCR.
func (r *SQLJudgmentRepository) ListPendingExtractions(ctx context.Context, limit int) ([]judgment.Judgment, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
		SELECT
			id, citation, citation_year, citation_court_code, citation_number,
			case_title, case_number, court, court_code, bench,
			judgment_date, decision_date, appellant, respondent, advocates,
			subject_tags, issues, held, headnote, summary, key_takeaways,
			court_case_id, article_id, access_level, status,
			pdf_storage_key, pdf_filename, pdf_size_bytes, pdf_checksum,
			pdf_page_count, pdf_thumbnail_key, pdf_preview_key,
			extraction_status, created_by, published_at, created_at, updated_at
		FROM judgments
		WHERE extraction_status = 'pending' AND pdf_storage_key IS NOT NULL
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending extractions: %w", err)
	}
	defer rows.Close()

	var results []judgment.Judgment
	for rows.Next() {
		j, err := r.scanJudgment(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *j)
	}
	return results, nil
}

// UpdateExtractionStatus updates the document extraction status for a judgment.
func (r *SQLJudgmentRepository) UpdateExtractionStatus(
	ctx context.Context,
	id string,
	status judgment.ExtractionStatus,
	now time.Time,
) error {
	query := `
		UPDATE judgments
		SET extraction_status = $2, updated_at = $3
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, string(status), now)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *SQLJudgmentRepository) scanJudgment(s rowScanner) (*judgment.Judgment, error) {
	var j judgment.Judgment
	var bench, advocates, tags, takeaways []string
	var accessLevel, status, extractionStatus string

	err := s.Scan(
		&j.ID, &j.Citation, &j.CitationYear, &j.CitationCourtCode, &j.CitationNumber,
		&j.CaseTitle, &j.CaseNumber, &j.Court, &j.CourtCode, pq.Array(&bench),
		&j.JudgmentDate, &j.DecisionDate, &j.Appellant, &j.Respondent, pq.Array(&advocates),
		pq.Array(&tags), &j.Issues, &j.Held, &j.Headnote, &j.Summary, pq.Array(&takeaways),
		&j.CourtCaseID, &j.ArticleID, &accessLevel, &status,
		&j.PDFStorageKey, &j.PDFFilename, &j.PDFSizeBytes, &j.PDFChecksum,
		&j.PDFPageCount, &j.PDFThumbnailKey, &j.PDFPreviewKey,
		&extractionStatus, &j.CreatedBy, &j.PublishedAt, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	j.Bench = bench
	j.Advocates = advocates
	j.SubjectTags = tags
	j.KeyTakeaways = takeaways
	j.AccessLevel = judgment.AccessLevel(accessLevel)
	j.Status = judgment.JudgmentStatus(status)
	j.ExtractionStatus = judgment.ExtractionStatus(extractionStatus)
	return &j, nil
}
