package ports

import (
	"context"
	"io"
)

// ExtractedPage holds page-level text extraction data and OCR determination.
type ExtractedPage struct {
	PageNumber int
	Text       string
	HasOCR     bool
}

// PDFExtractor defines the port for extracting text pages and previews from legal PDFs.
type PDFExtractor interface {
	ExtractPages(ctx context.Context, reader io.ReaderAt, size int64) ([]ExtractedPage, error)
	GenerateThumbnailAndPreview(ctx context.Context, reader io.ReaderAt, size int64) (thumbnail []byte, preview []byte, pageCount int, err error)
}

// OCREngine defines the port for running optical character recognition on scanned PDF pages.
type OCREngine interface {
	RecognizeText(ctx context.Context, pageImage []byte) (string, error)
}
