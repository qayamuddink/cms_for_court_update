package ports

import (
	"context"

	"github.com/livelaw/backend/internal/domain/livecase"
)

// LiveEnginePort defines the boundary between the monolith and the live ticker engine.
type LiveEnginePort interface {
	PublishUpdate(ctx context.Context, update livecase.LiveUpdate) error
	GetUpdatesAfter(ctx context.Context, caseID string, afterSequence int64, limit int) ([]livecase.LiveUpdate, error)
	GetLatestUpdates(ctx context.Context, caseID string, limit int) ([]livecase.LiveUpdate, error)
}
