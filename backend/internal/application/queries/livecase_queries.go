package queries

import (
	"context"
	"errors"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/livecase"
)

var (
	ErrCaseNotFound = errors.New("court case not found")
)

// LiveCaseQueries provides read operations for court cases and live updates.
type LiveCaseQueries struct {
	caseRepo       ports.CourtCaseRepository
	updateRepo     ports.LiveUpdateRepository
	assignmentRepo ports.CaseAssignmentRepository
}

// NewLiveCaseQueries creates a new LiveCaseQueries instance.
func NewLiveCaseQueries(
	caseRepo ports.CourtCaseRepository,
	updateRepo ports.LiveUpdateRepository,
	assignmentRepo ports.CaseAssignmentRepository,
) *LiveCaseQueries {
	return &LiveCaseQueries{
		caseRepo:       caseRepo,
		updateRepo:     updateRepo,
		assignmentRepo: assignmentRepo,
	}
}

// ListLiveCases returns all cases currently marked is_live = true.
func (q *LiveCaseQueries) ListLiveCases(ctx context.Context) ([]livecase.CourtCase, error) {
	return q.caseRepo.ListLiveCases(ctx)
}

// GetCase retrieves a court case by its ID.
func (q *LiveCaseQueries) GetCase(ctx context.Context, id string) (*livecase.CourtCase, error) {
	c, err := q.caseRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrCaseNotFound
	}
	return c, nil
}

// GetUpdatesAfter retrieves all updates for a case with sequence_no > afterSequence.
func (q *LiveCaseQueries) GetUpdatesAfter(ctx context.Context, caseID string, afterSequence int64, limit int) ([]livecase.LiveUpdate, error) {
	// Verify case exists
	c, err := q.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrCaseNotFound
	}

	return q.updateRepo.GetUpdatesAfter(ctx, caseID, afterSequence, limit)
}

// GetLatestUpdates retrieves the most recent updates for a case in chronological order.
func (q *LiveCaseQueries) GetLatestUpdates(ctx context.Context, caseID string, limit int) ([]livecase.LiveUpdate, error) {
	// Verify case exists
	c, err := q.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrCaseNotFound
	}

	return q.updateRepo.GetLatestUpdates(ctx, caseID, limit)
}

// ListAssignedReporters retrieves the list of reporter user IDs assigned to a case.
func (q *LiveCaseQueries) ListAssignedReporters(ctx context.Context, caseID string) ([]string, error) {
	c, err := q.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrCaseNotFound
	}

	return q.assignmentRepo.ListAssignedReporters(ctx, caseID)
}
