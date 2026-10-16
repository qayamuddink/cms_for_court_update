package liveengine

import (
	"context"
	"log/slog"
	"sync"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/livecase"
)

// InProcessLiveEngine implements ports.LiveEnginePort on top of PostgreSQL and in-memory event channels.
type InProcessLiveEngine struct {
	updateRepo ports.LiveUpdateRepository
	logger     *slog.Logger
	mu         sync.RWMutex
	listeners  map[string][]chan livecase.LiveUpdate
}

// NewInProcessLiveEngine creates a new InProcessLiveEngine.
func NewInProcessLiveEngine(updateRepo ports.LiveUpdateRepository, logger *slog.Logger) *InProcessLiveEngine {
	return &InProcessLiveEngine{
		updateRepo: updateRepo,
		logger:     logger,
		listeners:  make(map[string][]chan livecase.LiveUpdate),
	}
}

// PublishUpdate notifies any active in-process listeners of a newly committed live update.
func (e *InProcessLiveEngine) PublishUpdate(ctx context.Context, update livecase.LiveUpdate) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	channels, ok := e.listeners[update.CourtCaseID]
	if !ok {
		return nil
	}

	for _, ch := range channels {
		select {
		case ch <- update:
		default:
			e.logger.Warn("live update channel full, dropping real-time frame for slow consumer",
				slog.String("case_id", update.CourtCaseID),
				slog.Int64("sequence_no", update.SequenceNo),
			)
		}
	}

	return nil
}

// GetUpdatesAfter fetches all updates after a sequence number.
func (e *InProcessLiveEngine) GetUpdatesAfter(ctx context.Context, caseID string, afterSequence int64, limit int) ([]livecase.LiveUpdate, error) {
	return e.updateRepo.GetUpdatesAfter(ctx, caseID, afterSequence, limit)
}

// GetLatestUpdates fetches the latest N updates for a case.
func (e *InProcessLiveEngine) GetLatestUpdates(ctx context.Context, caseID string, limit int) ([]livecase.LiveUpdate, error) {
	return e.updateRepo.GetLatestUpdates(ctx, caseID, limit)
}
