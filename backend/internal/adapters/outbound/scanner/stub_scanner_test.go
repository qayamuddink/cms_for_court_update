package scanner_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/livelaw/backend/internal/adapters/outbound/scanner"
)

func TestStubScanner_CleanFile(t *testing.T) {
	s := scanner.NewStubScanner("")
	cleanPDF := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n%%EOF")

	isClean, virusName, err := s.Scan(context.Background(), bytes.NewReader(cleanPDF))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isClean {
		t.Errorf("expected clean = true, got false (virus: %s)", virusName)
	}
}

func TestStubScanner_EICARDetection(t *testing.T) {
	s := scanner.NewStubScanner("")
	eicar := []byte("X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*")

	isClean, virusName, err := s.Scan(context.Background(), bytes.NewReader(eicar))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isClean {
		t.Errorf("expected clean = false, got true")
	}
	if virusName != "EICAR-Test-Signature" {
		t.Errorf("expected virus name 'EICAR-Test-Signature', got %q", virusName)
	}
}
