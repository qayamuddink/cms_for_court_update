package ports

import (
	"context"
	"io"
)

// VirusScanner defines the security port to verify uploaded PDF documents are clean before storage.
type VirusScanner interface {
	Scan(ctx context.Context, reader io.Reader) (isClean bool, virusName string, err error)
}
