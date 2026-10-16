package ports

import (
	"context"
	"time"
)

// FileStore defines object storage interactions (S3/MinIO).
type FileStore interface {
	PresignedPutURL(ctx context.Context, key, contentType string, expiry time.Duration) (string, error)
	PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}
