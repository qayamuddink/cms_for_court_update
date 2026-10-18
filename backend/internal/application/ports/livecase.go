package ports

import (
	"context"
	"errors"
	"time"

	"github.com/livelaw/backend/internal/domain/livecase"
)

var (
	ErrCaseNotFound = errors.New("court case not found")
)

// Tx defines the transaction boundary abstraction for atomic operations.
type Tx interface {
	Commit() error
	Rollback() error
}

// TransactionManager coordinates database transactions across repository operations.
type TransactionManager interface {
	Begin(ctx context.Context) (Tx, error)
	// LockCaseForUpdate acquires an exclusive row lock on the court case within an active transaction.
	LockCaseForUpdate(ctx context.Context, tx Tx, caseID string) (*livecase.CourtCase, error)
	// UpdateCaseSequence updates the last_sequence_no of the case within the transaction.
	UpdateCaseSequence(ctx context.Context, tx Tx, caseID string, nextSeq int64, updatedAt time.Time) error
}

// CourtCaseRepository defines persistence operations for CourtCase entities.
type CourtCaseRepository interface {
	Create(ctx context.Context, c *livecase.CourtCase) error
	GetByID(ctx context.Context, id string) (*livecase.CourtCase, error)
	Update(ctx context.Context, c *livecase.CourtCase) error
	SetLive(ctx context.Context, id string, isLive bool, updatedAt time.Time) error
	ListLiveCases(ctx context.Context) ([]livecase.CourtCase, error)
	ListAll(ctx context.Context, limit int) ([]livecase.CourtCase, error)
}

// LiveUpdateRepository defines persistence operations for append-only LiveUpdate rows.
type LiveUpdateRepository interface {
	CreateInTx(ctx context.Context, tx Tx, update *livecase.LiveUpdate) error
	GetByID(ctx context.Context, id string) (*livecase.LiveUpdate, error)
	GetByIdempotencyKey(ctx context.Context, caseID, idempotencyKey string) (*livecase.LiveUpdate, error)
	HasVerdict(ctx context.Context, caseID string) (bool, error)
	IsRetracted(ctx context.Context, caseID, updateID string) (bool, error)
	GetUpdatesAfter(ctx context.Context, caseID string, afterSequence int64, limit int) ([]livecase.LiveUpdate, error)
	GetLatestUpdates(ctx context.Context, caseID string, limit int) ([]livecase.LiveUpdate, error)
}

// CaseAssignmentRepository defines reporter assignments to cases.
type CaseAssignmentRepository interface {
	Assign(ctx context.Context, assignment *livecase.CaseAssignment) error
	Unassign(ctx context.Context, caseID, userID string) error
	IsReporterAssigned(ctx context.Context, caseID, userID string) (bool, error)
	ListAssignedReporters(ctx context.Context, caseID string) ([]string, error)
}

// OutboxRepository defines persistence operations for the Transactional Outbox.
type OutboxRepository interface {
	SaveInTx(ctx context.Context, tx Tx, event *OutboxEvent) error
	Save(ctx context.Context, event *OutboxEvent) error
	GetUnpublished(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkPublished(ctx context.Context, id string, publishedAt time.Time) error
}
