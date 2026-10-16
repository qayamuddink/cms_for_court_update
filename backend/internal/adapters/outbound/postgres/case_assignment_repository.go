package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/livelaw/backend/internal/domain/livecase"
)

// SQLCaseAssignmentRepository implements ports.CaseAssignmentRepository using PostgreSQL.
type SQLCaseAssignmentRepository struct {
	db *sql.DB
}

// NewSQLCaseAssignmentRepository creates a new SQLCaseAssignmentRepository.
func NewSQLCaseAssignmentRepository(db *sql.DB) *SQLCaseAssignmentRepository {
	return &SQLCaseAssignmentRepository{db: db}
}

// Assign maps a reporter to a court case.
func (r *SQLCaseAssignmentRepository) Assign(ctx context.Context, assignment *livecase.CaseAssignment) error {
	query := `
		INSERT INTO case_assignments (id, court_case_id, user_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (court_case_id, user_id) DO NOTHING;
	`

	_, err := r.db.ExecContext(ctx, query,
		assignment.ID,
		assignment.CourtCaseID,
		assignment.UserID,
		assignment.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed assigning reporter to case: %w", err)
	}

	return nil
}

// Unassign removes a reporter's assignment to a court case.
func (r *SQLCaseAssignmentRepository) Unassign(ctx context.Context, caseID, userID string) error {
	query := `DELETE FROM case_assignments WHERE court_case_id = $1 AND user_id = $2;`
	_, err := r.db.ExecContext(ctx, query, caseID, userID)
	if err != nil {
		return fmt.Errorf("failed removing reporter assignment: %w", err)
	}
	return nil
}

// IsReporterAssigned checks if a given user is currently assigned to report on a case.
func (r *SQLCaseAssignmentRepository) IsReporterAssigned(ctx context.Context, caseID, userID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM case_assignments
			WHERE court_case_id = $1 AND user_id = $2
		);
	`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, caseID, userID).Scan(&exists)
	return exists, err
}

// ListAssignedReporters returns all user IDs assigned to report on a case.
func (r *SQLCaseAssignmentRepository) ListAssignedReporters(ctx context.Context, caseID string) ([]string, error) {
	query := `SELECT user_id FROM case_assignments WHERE court_case_id = $1;`
	rows, err := r.db.QueryContext(ctx, query, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed querying assigned reporters: %w", err)
	}
	defer rows.Close()

	var reporterIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		reporterIDs = append(reporterIDs, uid)
	}

	return reporterIDs, rows.Err()
}
