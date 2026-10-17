package scanner

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/livelaw/backend/internal/application/ports"
)

// Standard EICAR antivirus test signature
const eicarSignature = "X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*"

// StubScanner implements ports.VirusScanner with EICAR detection and ClamAV extensibility.
type StubScanner struct {
	clamAVAddr string
}

// NewStubScanner creates a new StubScanner instance.
func NewStubScanner(clamAVAddr string) *StubScanner {
	return &StubScanner{clamAVAddr: clamAVAddr}
}

// Scan checks the reader stream for virus signatures (e.g., EICAR test string).
func (s *StubScanner) Scan(ctx context.Context, reader io.Reader) (bool, string, error) {
	// Read beginning of payload or buffer
	buf := make([]byte, 1024*1024) // inspect up to 1MB sample for EICAR / known signatures
	n, err := io.ReadFull(reader, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, "", err
	}

	content := string(buf[:n])
	if strings.Contains(content, eicarSignature) || bytes.Contains(buf[:n], []byte("EICAR-STANDARD-ANTIVIRUS-TEST-FILE")) {
		return false, "EICAR-Test-Signature", nil
	}

	return true, "", nil
}

var _ ports.VirusScanner = (*StubScanner)(nil)
