package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/livelaw/backend/internal/domain/livecase"
)

// SQLCourtCaseRepository implements ports.CourtCaseRepository using PostgreSQL.
type SQLCourtCaseRepository struct {
	db *sql.DB
}

// NewSQLCourtCaseRepository creates a new SQLCourtCaseRepository.
func NewSQLCourtCaseRepository(db *sql.DB) *SQLCourtCaseRepository {
	return &SQLCourtCaseRepository{db: db}
}

// Create inserts a new CourtCase.
func (r *SQLCourtCaseRepository) Create(ctx context.Context, c *livecase.CourtCase) error {
	benchJSON, _ := json.Marshal(c.BenchJudges)
	if c.BenchJudges == nil {
		benchJSON = []byte("[]")
	}
	petJSON, _ := json.Marshal(c.PetitionerParties)
	if c.PetitionerParties == nil {
		petJSON = []byte("[]")
	}
	respJSON, _ := json.Marshal(c.RespondentParties)
	if c.RespondentParties == nil {
		respJSON = []byte("[]")
	}

	query := `
		INSERT INTO court_cases (
			id, court_name, case_number, case_type, title,
			bench_judges, presiding_judge, petitioner_parties, respondent_parties,
			status, next_hearing_at, last_sequence_no, is_live,
			judgment_article_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16
		);
	`

	_, err := r.db.ExecContext(ctx, query,
		c.ID,
		c.CourtName,
		c.CaseNumber,
		c.CaseType,
		c.Title,
		benchJSON,
		c.PresidingJudge,
		petJSON,
		respJSON,
		string(c.Status),
		c.NextHearingAt,
		c.LastSequenceNo,
		c.IsLive,
		c.JudgmentArticleID,
		c.CreatedAt,
		c.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert court case: %w", err)
	}

	return nil
}

// GetByID retrieves a CourtCase by its ID.
func (r *SQLCourtCaseRepository) GetByID(ctx context.Context, id string) (*livecase.CourtCase, error) {
	query := `
		SELECT
			id, court_name, case_number, case_type, title,
			bench_judges, presiding_judge, petitioner_parties, respondent_parties,
			status, next_hearing_at, last_sequence_no, is_live,
			judgment_article_id, created_at, updated_at
		FROM court_cases
		WHERE id = $1;
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanCase(row)
}

// Update updates case attributes (title, presiding_judge, bench_judges, petitioner_parties, respondent_parties, status, next_hearing_at, updated_at).
func (r *SQLCourtCaseRepository) Update(ctx context.Context, c *livecase.CourtCase) error {
	benchJSON, _ := json.Marshal(c.BenchJudges)
	petJSON, _ := json.Marshal(c.PetitionerParties)
	respJSON, _ := json.Marshal(c.RespondentParties)

	query := `
		UPDATE court_cases SET
			court_name = $1,
			case_number = $2,
			case_type = $3,
			title = $4,
			bench_judges = $5,
			presiding_judge = $6,
			petitioner_parties = $7,
			respondent_parties = $8,
			status = $9,
			next_hearing_at = $10,
			updated_at = $11
		WHERE id = $12;
	`

	res, err := r.db.ExecContext(ctx, query,
		c.CourtName,
		c.CaseNumber,
		c.CaseType,
		c.Title,
		benchJSON,
		c.PresidingJudge,
		petJSON,
		respJSON,
		string(c.Status),
		c.NextHearingAt,
		c.UpdatedAt,
		c.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update court case: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("court case not found")
	}

	return nil
}

// SetLive toggles the is_live state of a case.
func (r *SQLCourtCaseRepository) SetLive(ctx context.Context, id string, isLive bool, updatedAt time.Time) error {
	query := `UPDATE court_cases SET is_live = $1, updated_at = $2 WHERE id = $3;`
	res, err := r.db.ExecContext(ctx, query, isLive, updatedAt, id)
	if err != nil {
		return fmt.Errorf("failed updating is_live state: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("court case not found")
	}

	return nil
}

// ListLiveCases returns all cases currently marked is_live = true, utilizing idx_court_cases_live_hearing.
func (r *SQLCourtCaseRepository) ListLiveCases(ctx context.Context) ([]livecase.CourtCase, error) {
	query := `
		SELECT
			id, court_name, case_number, case_type, title,
			bench_judges, presiding_judge, petitioner_parties, respondent_parties,
			status, next_hearing_at, last_sequence_no, is_live,
			judgment_article_id, created_at, updated_at
		FROM court_cases
		WHERE is_live = true
		ORDER BY next_hearing_at ASC NULLS LAST, updated_at DESC;
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed listing live cases: %w", err)
	}
	defer rows.Close()

	var cases []livecase.CourtCase
	for rows.Next() {
		c, err := r.scanCase(rows)
		if err != nil {
			return nil, err
		}
		cases = append(cases, *c)
	}

	return cases, rows.Err()
}

// ListAll returns court cases up to the specified limit.
func (r *SQLCourtCaseRepository) ListAll(ctx context.Context, limit int) ([]livecase.CourtCase, error) {
	if limit <= 0 {
		limit = 1000
	}
	query := `
		SELECT
			id, court_name, case_number, case_type, title,
			bench_judges, presiding_judge, petitioner_parties, respondent_parties,
			status, next_hearing_at, last_sequence_no, is_live,
			judgment_article_id, created_at, updated_at
		FROM court_cases
		ORDER BY updated_at DESC
		LIMIT $1;
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed listing all court cases: %w", err)
	}
	defer rows.Close()

	var cases []livecase.CourtCase
	for rows.Next() {
		c, err := r.scanCase(rows)
		if err != nil {
			return nil, err
		}
		cases = append(cases, *c)
	}

	return cases, rows.Err()
}

func (r *SQLCourtCaseRepository) scanCase(s scannable) (*livecase.CourtCase, error) {
	var c livecase.CourtCase
	var statusStr string
	var benchJSON, petJSON, respJSON []byte

	err := s.Scan(
		&c.ID,
		&c.CourtName,
		&c.CaseNumber,
		&c.CaseType,
		&c.Title,
		&benchJSON,
		&c.PresidingJudge,
		&petJSON,
		&respJSON,
		&statusStr,
		&c.NextHearingAt,
		&c.LastSequenceNo,
		&c.IsLive,
		&c.JudgmentArticleID,
		&c.CreatedAt,
		&c.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed scanning court case: %w", err)
	}

	c.Status = livecase.CaseStatus(statusStr)
	_ = json.Unmarshal(benchJSON, &c.BenchJudges)
	_ = json.Unmarshal(petJSON, &c.PetitionerParties)
	_ = json.Unmarshal(respJSON, &c.RespondentParties)

	return &c, nil
}
