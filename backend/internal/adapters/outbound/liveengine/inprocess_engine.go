package liveengine

import (
	"context"

	"github.com/livelaw/backend/internal/domain/livecase"
)

// InProcessLiveEngine implements ports.LiveEnginePort directly using MemoryLiveEngine.
type InProcessLiveEngine struct {
	memEngine *MemoryLiveEngine
}

// NewInProcessLiveEngine creates a new InProcessLiveEngine.
func NewInProcessLiveEngine(memEngine *MemoryLiveEngine) *InProcessLiveEngine {
	return &InProcessLiveEngine{memEngine: memEngine}
}

// PublishUpdate notifies listeners and stores update in memory.
func (e *InProcessLiveEngine) PublishUpdate(ctx context.Context, update livecase.LiveUpdate) error {
	return e.memEngine.PublishUpdate(ctx, update)
}

// GetUpdatesAfter retrieves all updates for a case with sequence_no > afterSequence.
func (e *InProcessLiveEngine) GetUpdatesAfter(ctx context.Context, caseID string, afterSequence int64, limit int) ([]livecase.LiveUpdate, error) {
	return e.memEngine.GetUpdatesAfter(ctx, caseID, afterSequence, limit)
}

// GetLatestUpdates retrieves the most recent updates for a case.
func (e *InProcessLiveEngine) GetLatestUpdates(ctx context.Context, caseID string, limit int) ([]livecase.LiveUpdate, error) {
	return e.memEngine.GetLatestUpdates(ctx, caseID, limit)
}

// Subscribe opens a real-time event channel with historical replay from afterSequence.
func (e *InProcessLiveEngine) Subscribe(ctx context.Context, caseID string, afterSequence int64) (<-chan livecase.LiveUpdate, error) {
	return e.memEngine.Subscribe(ctx, caseID, afterSequence)
}

// MemoryEngine returns the underlying memory engine.
func (e *InProcessLiveEngine) MemoryEngine() *MemoryLiveEngine {
	return e.memEngine
}
