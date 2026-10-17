package ports

import (
	"context"
	"time"

	"github.com/livelaw/backend/internal/domain/judgment"
)

// JudgmentFilter specifies search and listing filters for judgments.
type JudgmentFilter struct {
	CourtCode   *string
	Year        *int
	Status      *judgment.JudgmentStatus
	AccessLevel *judgment.AccessLevel
	SearchQuery *string
	StatuteName *string
	CaseID      *string
	Limit       int
	Offset      int
}

// JudgmentRepository defines the database persistence port for judgments, citations, pages, and audit logs.
type JudgmentRepository interface {
	Create(ctx context.Context, j *judgment.Judgment) error
	GetByID(ctx context.Context, id string) (*judgment.Judgment, error)
	GetByCitation(ctx context.Context, citation string) (*judgment.Judgment, error)
	GetByCaseID(ctx context.Context, caseID string) (*judgment.Judgment, error)
	GetByChecksum(ctx context.Context, checksum string) (*judgment.Judgment, error)
	Update(ctx context.Context, j *judgment.Judgment) error
	List(ctx context.Context, filter JudgmentFilter) ([]judgment.Judgment, int, error)

	// AllocateCitation atomically increments the sequential counter per court and year using row-level locking.
	AllocateCitation(ctx context.Context, courtCode string, year int, brandPrefix string) (citation string, number int, err error)

	// Statutes
	AddStatutes(ctx context.Context, judgmentID string, statutes []judgment.StatuteCited) error
	GetStatutes(ctx context.Context, judgmentID string) ([]judgment.StatuteCited, error)

	// Pages & Extracted text
	SavePages(ctx context.Context, judgmentID string, pages []judgment.JudgmentPage) error
	GetPages(ctx context.Context, judgmentID string) ([]judgment.JudgmentPage, error)

	// Audit Logging
	LogDownload(ctx context.Context, download judgment.JudgmentDownload) error

	// Background extraction worker support
	ListPendingExtractions(ctx context.Context, limit int) ([]judgment.Judgment, error)
	UpdateExtractionStatus(ctx context.Context, id string, status judgment.ExtractionStatus, now time.Time) error
}
