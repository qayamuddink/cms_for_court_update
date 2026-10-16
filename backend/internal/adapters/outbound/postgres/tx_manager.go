package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/livecase"
)

// SQLTx wraps *sql.Tx implementing ports.Tx.
type SQLTx struct {
	tx *sql.Tx
}

func (t *SQLTx) Commit() error {
	return t.tx.Commit()
}

func (t *SQLTx) Rollback() error {
	return t.tx.Rollback()
}

func (t *SQLTx) Underlying() *sql.Tx {
	return t.tx
}

// SQLTransactionManager manages SQL transactions and case row locking.
type SQLTransactionManager struct {
	db *sql.DB
}

// NewSQLTransactionManager creates a new SQLTransactionManager.
func NewSQLTransactionManager(db *sql.DB) *SQLTransactionManager {
	return &SQLTransactionManager{db: db}
}

// Begin starts a new transaction.
func (m *SQLTransactionManager) Begin(ctx context.Context) (ports.Tx, error) {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &SQLTx{tx: tx}, nil
}

// LockCaseForUpdate acquires an exclusive row-level lock on the court case within the transaction.
func (m *SQLTransactionManager) LockCaseForUpdate(ctx context.Context, tx ports.Tx, caseID string) (*livecase.CourtCase, error) {
	sqlTx, ok := tx.(*SQLTx)
	if !ok {
		return nil, errors.New("invalid transaction type")
	}

	query := `
		SELECT
			id, court_name, case_number, case_type, title,
			bench_judges, presiding_judge, petitioner_parties, respondent_parties,
			status, next_hearing_at, last_sequence_no, is_live,
			judgment_article_id, created_at, updated_at
		FROM court_cases
		WHERE id = $1
		FOR UPDATE;
	`

	row := sqlTx.tx.QueryRowContext(ctx, query, caseID)

	var c livecase.CourtCase
	var statusStr string
	var benchJudgesJSON, petJSON, respJSON []byte

	err := row.Scan(
		&c.ID,
		&c.CourtName,
		&c.CaseNumber,
		&c.CaseType,
		&c.Title,
		&benchJudgesJSON,
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
			return nil, nil // Case not found
		}
		return nil, fmt.Errorf("failed to lock case for update: %w", err)
	}

	c.Status = livecase.CaseStatus(statusStr)
	_ = json.Unmarshal(benchJudgesJSON, &c.BenchJudges)
	_ = json.Unmarshal(petJSON, &c.PetitionerParties)
	_ = json.Unmarshal(respJSON, &c.RespondentParties)

	return &c, nil
}

// UpdateCaseSequence updates the sequence counter and updated_at on the case row.
func (m *SQLTransactionManager) UpdateCaseSequence(ctx context.Context, tx ports.Tx, caseID string, nextSeq int64, updatedAt time.Time) error {
	sqlTx, ok := tx.(*SQLTx)
	if !ok {
		return errors.New("invalid transaction type")
	}

	query := `UPDATE court_cases SET last_sequence_no = $1, updated_at = $2 WHERE id = $3;`
	res, err := sqlTx.tx.ExecContext(ctx, query, nextSeq, updatedAt, caseID)
	if err != nil {
		return fmt.Errorf("failed updating case sequence counter: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("case not found during sequence update")
	}

	return nil
}
