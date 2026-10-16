package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/livecase"
)

// SQLLiveUpdateRepository implements ports.LiveUpdateRepository using PostgreSQL.
type SQLLiveUpdateRepository struct {
	db *sql.DB
}

// NewSQLLiveUpdateRepository creates a new SQLLiveUpdateRepository.
func NewSQLLiveUpdateRepository(db *sql.DB) *SQLLiveUpdateRepository {
	return &SQLLiveUpdateRepository{db: db}
}

// CreateInTx inserts an append-only live_update row within an active transaction.
func (r *SQLLiveUpdateRepository) CreateInTx(ctx context.Context, tx ports.Tx, update *livecase.LiveUpdate) error {
	sqlTx, ok := tx.(*SQLTx)
	if !ok {
		return errors.New("invalid transaction type for live update insert")
	}

	query := `
		INSERT INTO live_updates (
			id, court_case_id, sequence_no, update_type, speaker_label,
			content, reporter_id, status, corrects_update_id, event_time,
			created_at, idempotency_key
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12
		);
	`

	_, err := sqlTx.tx.ExecContext(ctx, query,
		update.ID,
		update.CourtCaseID,
		update.SequenceNo,
		string(update.UpdateType),
		update.SpeakerLabel,
		update.Content,
		update.ReporterID,
		string(update.Status),
		update.CorrectsUpdateID,
		update.EventTime,
		update.CreatedAt,
		update.IdempotencyKey,
	)

	if err != nil {
		return fmt.Errorf("failed inserting live update: %w", err)
	}

	return nil
}

// GetByID retrieves a live update by ID.
func (r *SQLLiveUpdateRepository) GetByID(ctx context.Context, id string) (*livecase.LiveUpdate, error) {
	query := `
		SELECT
			id, court_case_id, sequence_no, update_type, speaker_label,
			content, reporter_id, status, corrects_update_id, event_time,
			created_at, idempotency_key
		FROM live_updates
		WHERE id = $1;
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanUpdate(row)
}

// GetByIdempotencyKey retrieves an update previously posted with this idempotency key for the case.
func (r *SQLLiveUpdateRepository) GetByIdempotencyKey(ctx context.Context, caseID, idempotencyKey string) (*livecase.LiveUpdate, error) {
	query := `
		SELECT
			id, court_case_id, sequence_no, update_type, speaker_label,
			content, reporter_id, status, corrects_update_id, event_time,
			created_at, idempotency_key
		FROM live_updates
		WHERE court_case_id = $1 AND idempotency_key = $2;
	`

	row := r.db.QueryRowContext(ctx, query, caseID, idempotencyKey)
	return r.scanUpdate(row)
}

// HasVerdict checks whether an active (non-retracted) verdict has already been posted to the case.
func (r *SQLLiveUpdateRepository) HasVerdict(ctx context.Context, caseID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM live_updates
			WHERE court_case_id = $1
			  AND update_type = 'verdict'
			  AND status != 'retracted'
		);
	`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, caseID).Scan(&exists)
	return exists, err
}

// IsRetracted checks if an update or any retraction referencing it has been posted.
func (r *SQLLiveUpdateRepository) IsRetracted(ctx context.Context, caseID, updateID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM live_updates
			WHERE court_case_id = $1 AND (
				(id = $2 AND status = 'retracted')
				OR
				(corrects_update_id = $2 AND status = 'retracted')
			)
		);
	`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, caseID, updateID).Scan(&exists)
	return exists, err
}

// GetUpdatesAfter retrieves all updates for a case with sequence_no > afterSequence in ascending order (ticker replay).
func (r *SQLLiveUpdateRepository) GetUpdatesAfter(ctx context.Context, caseID string, afterSequence int64, limit int) ([]livecase.LiveUpdate, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT
			id, court_case_id, sequence_no, update_type, speaker_label,
			content, reporter_id, status, corrects_update_id, event_time,
			created_at, idempotency_key
		FROM live_updates
		WHERE court_case_id = $1 AND sequence_no > $2
		ORDER BY sequence_no ASC
		LIMIT $3;
	`

	rows, err := r.db.QueryContext(ctx, query, caseID, afterSequence, limit)
	if err != nil {
		return nil, fmt.Errorf("failed querying updates after sequence: %w", err)
	}
	defer rows.Close()

	var updates []livecase.LiveUpdate
	for rows.Next() {
		u, err := r.scanUpdate(rows)
		if err != nil {
			return nil, err
		}
		updates = append(updates, *u)
	}

	return updates, rows.Err()
}

// GetLatestUpdates retrieves the most recent updates for a case in ascending chronological order.
func (r *SQLLiveUpdateRepository) GetLatestUpdates(ctx context.Context, caseID string, limit int) ([]livecase.LiveUpdate, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Use subquery with idx_live_updates_case_seq_desc for blazing fast index scan, then reorder ASC
	query := `
		SELECT
			id, court_case_id, sequence_no, update_type, speaker_label,
			content, reporter_id, status, corrects_update_id, event_time,
			created_at, idempotency_key
		FROM (
			SELECT
				id, court_case_id, sequence_no, update_type, speaker_label,
				content, reporter_id, status, corrects_update_id, event_time,
				created_at, idempotency_key
			FROM live_updates
			WHERE court_case_id = $1
			ORDER BY sequence_no DESC
			LIMIT $2
		) sub
		ORDER BY sequence_no ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, caseID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed querying latest updates: %w", err)
	}
	defer rows.Close()

	var updates []livecase.LiveUpdate
	for rows.Next() {
		u, err := r.scanUpdate(rows)
		if err != nil {
			return nil, err
		}
		updates = append(updates, *u)
	}

	return updates, rows.Err()
}

func (r *SQLLiveUpdateRepository) scanUpdate(s scannable) (*livecase.LiveUpdate, error) {
	var u livecase.LiveUpdate
	var typeStr, statusStr string

	err := s.Scan(
		&u.ID,
		&u.CourtCaseID,
		&u.SequenceNo,
		&typeStr,
		&u.SpeakerLabel,
		&u.Content,
		&u.ReporterID,
		&statusStr,
		&u.CorrectsUpdateID,
		&u.EventTime,
		&u.CreatedAt,
		&u.IdempotencyKey,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed scanning live update: %w", err)
	}

	u.UpdateType = livecase.UpdateType(typeStr)
	u.Status = livecase.UpdateStatus(statusStr)

	return &u, nil
}
