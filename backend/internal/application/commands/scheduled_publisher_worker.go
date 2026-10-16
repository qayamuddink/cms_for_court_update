package commands

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
)

// ScheduledPublisherWorker runs periodically in the background to publish articles whose scheduled_at timestamp has arrived.
type ScheduledPublisherWorker struct {
	articleRepo ports.ArticleRepository
	clock       ports.Clock
	interval    time.Duration
	stopCh      chan struct{}
}

// NewScheduledPublisherWorker creates a new ScheduledPublisherWorker instance.
func NewScheduledPublisherWorker(
	articleRepo ports.ArticleRepository,
	clock ports.Clock,
	interval time.Duration,
) *ScheduledPublisherWorker {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &ScheduledPublisherWorker{
		articleRepo: articleRepo,
		clock:       clock,
		interval:    interval,
		stopCh:      make(chan struct{}),
	}
}

// Start launches the background scheduler loop.
func (w *ScheduledPublisherWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			case <-ticker.C:
				count, err := w.ProcessDueScheduledArticles(ctx)
				if err != nil {
					log.Printf("[ScheduledPublisherWorker] Error processing scheduled articles: %v", err)
				} else if count > 0 {
					log.Printf("[ScheduledPublisherWorker] Successfully published %d scheduled article(s)", count)
				}
			}
		}
	}()
}

// Stop terminates the worker loop.
func (w *ScheduledPublisherWorker) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
}

// ProcessDueScheduledArticles queries due scheduled articles and publishes them. Returns count published.
func (w *ScheduledPublisherWorker) ProcessDueScheduledArticles(ctx context.Context) (int, error) {
	now := w.clock.Now().UTC()
	due, err := w.articleRepo.GetScheduledDue(ctx, now, 50)
	if err != nil {
		return 0, fmt.Errorf("failed fetching due scheduled articles: %w", err)
	}

	publishedCount := 0
	for _, a := range due {
		pubTime := now
		if a.ScheduledAt != nil {
			pubTime = *a.ScheduledAt
		}
		if err := w.articleRepo.PublishScheduled(ctx, a.ID, pubTime); err != nil {
			log.Printf("[ScheduledPublisherWorker] Failed publishing article %s: %v", a.ID, err)
			continue
		}
		publishedCount++
	}

	return publishedCount, nil
}
