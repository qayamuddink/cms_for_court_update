package extractor_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/livelaw/backend/internal/adapters/outbound/extractor"
)

func TestStandardPDFExtractor_ExtractPages(t *testing.T) {
	e := extractor.NewStandardPDFExtractor(nil)

	// Minimal mock PDF with 2 pages and stream
	pdfData := []byte(`%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R >>
stream
BT
/F1 12 Tf
(Supreme Court Judgment Page 1 Content) Tj
ET
endstream
endobj
4 0 obj
<< /Type /Page /Parent 2 0 R >>
stream
BT
/F1 12 Tf
(Supreme Court Judgment Page 2 Analysis) Tj
ET
endstream
endobj
%%EOF`)

	pages, err := e.ExtractPages(context.Background(), bytes.NewReader(pdfData), int64(len(pdfData)))
	if err != nil {
		t.Fatalf("ExtractPages failed: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}

	if pages[0].PageNumber != 1 || pages[1].PageNumber != 2 {
		t.Errorf("expected page numbers 1 and 2, got %d and %d", pages[0].PageNumber, pages[1].PageNumber)
	}

	thumb, prev, pageCount, err := e.GenerateThumbnailAndPreview(context.Background(), bytes.NewReader(pdfData), int64(len(pdfData)))
	if err != nil {
		t.Fatalf("GenerateThumbnailAndPreview failed: %v", err)
	}
	if pageCount != 2 {
		t.Errorf("expected page count 2, got %d", pageCount)
	}
	if len(thumb) == 0 || len(prev) == 0 {
		t.Errorf("expected non-empty thumbnail and preview")
	}
}

func TestStandardPDFExtractor_InvalidMagicBytes(t *testing.T) {
	e := extractor.NewStandardPDFExtractor(nil)
	fakePDF := []byte("NOT A REAL PDF FILE")

	_, err := e.ExtractPages(context.Background(), bytes.NewReader(fakePDF), int64(len(fakePDF)))
	if err == nil {
		t.Errorf("expected error for invalid magic bytes, got nil")
	}
}
