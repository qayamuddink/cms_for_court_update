package liveengine

import (
	"context"
	"log/slog"
	"sync"

	"github.com/livelaw/backend/internal/domain/livecase"
)

// MemoryLiveEngine implements an in-memory high-throughput ticker broadcast engine.
// It never touches the primary database and relies entirely on event ingestion.
type MemoryLiveEngine struct {
	logger      *slog.Logger
	mu          sync.RWMutex
	caseUpdates map[string][]livecase.LiveUpdate
	subscribers map[string][]chan livecase.LiveUpdate
}

// NewMemoryLiveEngine creates a new MemoryLiveEngine instance.
func NewMemoryLiveEngine(logger *slog.Logger) *MemoryLiveEngine {
	return &MemoryLiveEngine{
		logger:      logger,
		caseUpdates: make(map[string][]livecase.LiveUpdate),
		subscribers: make(map[string][]chan livecase.LiveUpdate),
	}
}

// PublishUpdate ingests a new live update into the memory engine and fans out to all subscribers.
func (e *MemoryLiveEngine) PublishUpdate(ctx context.Context, update livecase.LiveUpdate) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	caseID := update.CourtCaseID
	existing := e.caseUpdates[caseID]

	// Check if already ingested (idempotency by sequence number)
	alreadyPresent := false
	for _, u := range existing {
		if u.SequenceNo == update.SequenceNo {
			alreadyPresent = true
			break
		}
	}

	if !alreadyPresent {
		e.caseUpdates[caseID] = append(existing, update)
	}

	// Fan out to active subscribers
	subs := e.subscribers[caseID]
	for _, ch := range subs {
		select {
		case ch <- update:
		default:
			e.logger.Warn("subscriber channel buffer full, dropping frame for slow consumer",
				slog.String("case_id", caseID),
				slog.Int64("sequence_no", update.SequenceNo),
			)
		}
	}

	return nil
}

// Subscribe returns a channel streaming updates for caseID strictly after afterSequence.
func (e *MemoryLiveEngine) Subscribe(ctx context.Context, caseID string, afterSequence int64) (<-chan livecase.LiveUpdate, error) {
	e.mu.Lock()
	ch := make(chan livecase.LiveUpdate, 200)

	// Replay any buffered historical updates after afterSequence
	existing := e.caseUpdates[caseID]
	for _, u := range existing {
		if u.SequenceNo > afterSequence {
			select {
			case ch <- u:
			default:
			}
		}
	}

	// Register subscriber for live events
	e.subscribers[caseID] = append(e.subscribers[caseID], ch)
	e.mu.Unlock()

	// Clean up registration on context cancellation
	go func() {
		<-ctx.Done()
		e.mu.Lock()
		defer e.mu.Unlock()

		subs := e.subscribers[caseID]
		for i, sub := range subs {
			if sub == ch {
				e.subscribers[caseID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
	}()

	return ch, nil
}

// GetUpdatesAfter retrieves all updates for a case with sequence_no > afterSequence up to limit.
func (e *MemoryLiveEngine) GetUpdatesAfter(ctx context.Context, caseID string, afterSequence int64, limit int) ([]livecase.LiveUpdate, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	existing := e.caseUpdates[caseID]
	var results []livecase.LiveUpdate
	for _, u := range existing {
		if u.SequenceNo > afterSequence {
			results = append(results, u)
			if len(results) >= limit {
				break
			}
		}
	}

	if results == nil {
		results = []livecase.LiveUpdate{}
	}

	return results, nil
}

// GetLatestUpdates retrieves the most recent N updates for a case in chronological order.
func (e *MemoryLiveEngine) GetLatestUpdates(ctx context.Context, caseID string, limit int) ([]livecase.LiveUpdate, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	existing := e.caseUpdates[caseID]
	total := len(existing)
	if total == 0 {
		return []livecase.LiveUpdate{}, nil
	}

	start := total - limit
	if start < 0 {
		start = 0
	}

	result := make([]livecase.LiveUpdate, total-start)
	copy(result, existing[start:])
	return result, nil
}

// Snapshot returns the highest sequence number and latest N updates (or updates after afterSequence) for a case.
func (e *MemoryLiveEngine) Snapshot(caseID string, limit int, afterSequence *int64) (int64, []livecase.LiveUpdate) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	existing := e.caseUpdates[caseID]
	total := len(existing)
	if total == 0 {
		return 0, []livecase.LiveUpdate{}
	}

	highestSeq := existing[total-1].SequenceNo

	if afterSequence != nil {
		var results []livecase.LiveUpdate
		for _, u := range existing {
			if u.SequenceNo > *afterSequence {
				results = append(results, u)
				if len(results) >= limit {
					break
				}
			}
		}
		if results == nil {
			results = []livecase.LiveUpdate{}
		}
		return highestSeq, results
	}

	start := total - limit
	if start < 0 {
		start = 0
	}

	result := make([]livecase.LiveUpdate, total-start)
	copy(result, existing[start:])
	return highestSeq, result
}
