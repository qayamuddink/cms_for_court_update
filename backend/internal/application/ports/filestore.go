package ports

import (
	"context"
	"io"
	"time"
)

// FileStore defines object storage interactions (S3/MinIO).
type FileStore interface {
	// PresignedPutURL generates a temporary signed URL allowing a client to upload directly to S3/MinIO.
	PresignedPutURL(ctx context.Context, key, contentType string, expiry time.Duration) (string, error)

	// PresignedGetURL generates a temporary signed URL allowing secure downloading of private documents.
	PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error)

	// PutObject stores a stream directly into object storage.
	PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error

	// GetObject retrieves a file stream directly from object storage.
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes an object from storage.
	Delete(ctx context.Context, key string) error
}
