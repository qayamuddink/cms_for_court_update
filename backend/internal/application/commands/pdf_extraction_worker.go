package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/judgment"
)

// PDFExtractionWorker runs periodically in the background to process uploaded judgment PDFs,
// extracting text page-by-page and running OCR fallback where text layers are absent.
type PDFExtractionWorker struct {
	judgmentRepo ports.JudgmentRepository
	fileStore    ports.FileStore
	extractor    ports.PDFExtractor
	idGen        ports.IDGenerator
	clock        ports.Clock
	interval     time.Duration
	stopCh       chan struct{}
}

// NewPDFExtractionWorker creates a new PDFExtractionWorker instance.
func NewPDFExtractionWorker(
	judgmentRepo ports.JudgmentRepository,
	fileStore ports.FileStore,
	extractor ports.PDFExtractor,
	idGen ports.IDGenerator,
	clock ports.Clock,
	interval time.Duration,
) *PDFExtractionWorker {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	return &PDFExtractionWorker{
		judgmentRepo: judgmentRepo,
		fileStore:    fileStore,
		extractor:    extractor,
		idGen:        idGen,
		clock:        clock,
		interval:     interval,
		stopCh:       make(chan struct{}),
	}
}

// Start launches the background extraction worker loop.
func (w *PDFExtractionWorker) Start(ctx context.Context) {
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
				count, err := w.ProcessPendingExtractions(ctx)
				if err != nil {
					log.Printf("[PDFExtractionWorker] Error processing judgment PDFs: %v", err)
				} else if count > 0 {
					log.Printf("[PDFExtractionWorker] Processed and indexed text for %d judgment(s)", count)
				}
			}
		}
	}()
}

// Stop cleanly stops the background worker.
func (w *PDFExtractionWorker) Stop() {
	close(w.stopCh)
}

// ProcessPendingExtractions finds pending judgments and extracts text page-by-page.
func (w *PDFExtractionWorker) ProcessPendingExtractions(ctx context.Context) (int, error) {
	pending, err := w.judgmentRepo.ListPendingExtractions(ctx, 10)
	if err != nil {
		return 0, fmt.Errorf("failed to query pending extractions: %w", err)
	}

	processed := 0
	now := w.clock.Now().UTC()

	for _, j := range pending {
		if j.PDFStorageKey == nil || *j.PDFStorageKey == "" {
			continue
		}

		// Download PDF payload
		var pdfData []byte
		if w.fileStore != nil {
			rc, err := w.fileStore.GetObject(ctx, *j.PDFStorageKey)
			if err != nil {
				log.Printf("[PDFExtractionWorker] Failed to download pdf for judgment %s: %v", j.ID, err)
				_ = w.judgmentRepo.UpdateExtractionStatus(ctx, j.ID, judgment.ExtractionFailed, now)
				continue
			}
			data, readErr := io.ReadAll(rc)
			_ = rc.Close()
			if readErr != nil {
				log.Printf("[PDFExtractionWorker] Error reading pdf stream for judgment %s: %v", j.ID, readErr)
				_ = w.judgmentRepo.UpdateExtractionStatus(ctx, j.ID, judgment.ExtractionFailed, now)
				continue
			}
			pdfData = data
		}

		if len(pdfData) == 0 {
			continue
		}

		// Run extraction
		extractedPages, err := w.extractor.ExtractPages(ctx, bytes.NewReader(pdfData), int64(len(pdfData)))
		if err != nil {
			log.Printf("[PDFExtractionWorker] Extraction failed for judgment %s: %v", j.ID, err)
			_ = w.judgmentRepo.UpdateExtractionStatus(ctx, j.ID, judgment.ExtractionFailed, now)
			continue
		}

		var domainPages []judgment.JudgmentPage
		hasOCR := false

		for _, p := range extractedPages {
			if p.HasOCR {
				hasOCR = true
			}
			domainPages = append(domainPages, judgment.JudgmentPage{
				ID:          w.idGen.NewID(),
				JudgmentID:  j.ID,
				PageNumber:  p.PageNumber,
				TextContent: p.Text,
				HasOCR:      p.HasOCR,
				CreatedAt:   now,
			})
		}

		// Save pages
		if err := w.judgmentRepo.SavePages(ctx, j.ID, domainPages); err != nil {
			log.Printf("[PDFExtractionWorker] Failed to save pages for judgment %s: %v", j.ID, err)
			_ = w.judgmentRepo.UpdateExtractionStatus(ctx, j.ID, judgment.ExtractionFailed, now)
			continue
		}

		finalStatus := judgment.ExtractionCompleted
		if hasOCR {
			finalStatus = judgment.ExtractionOCRCompleted
		}

		if err := w.judgmentRepo.UpdateExtractionStatus(ctx, j.ID, finalStatus, now); err != nil {
			log.Printf("[PDFExtractionWorker] Failed to update extraction status for judgment %s: %v", j.ID, err)
			continue
		}

		processed++
	}

	return processed, nil
}
