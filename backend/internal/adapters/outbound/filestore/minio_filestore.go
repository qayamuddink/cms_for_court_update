package filestore

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOFileStore implements ports.FileStore using MinIO / S3 SDK.
type MinIOFileStore struct {
	client     *minio.Client
	bucketName string
}

// Config holds configuration parameters for S3/MinIO client.
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	UseSSL          bool
}

// NewMinIOFileStore initializes a new MinIO file storage adapter.
func NewMinIOFileStore(cfg Config) (*MinIOFileStore, error) {
	endpoint := cfg.Endpoint
	useSSL := cfg.UseSSL

	// Normalize endpoint if scheme was provided
	if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
		useSSL = false
	} else if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
		useSSL = true
	}

	endpoint = strings.TrimRight(endpoint, "/")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	bucket := cfg.BucketName
	if bucket == "" {
		bucket = "livelaw-files"
	}

	return &MinIOFileStore{
		client:     client,
		bucketName: bucket,
	}, nil
}

// EnsureBucket ensures the target bucket exists, creating it if necessary.
func (s *MinIOFileStore) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucketName)
	if err != nil {
		return fmt.Errorf("error checking bucket existence: %w", err)
	}
	if !exists {
		err = s.client.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("error creating bucket %s: %w", s.bucketName, err)
		}
	}
	return nil
}

// PresignedPutURL generates a signed PUT URL allowing the client to upload directly.
func (s *MinIOFileStore) PresignedPutURL(ctx context.Context, key, contentType string, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	u, err := s.client.PresignedPutObject(ctx, s.bucketName, key, expiry)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned PUT url: %w", err)
	}
	return u.String(), nil
}

// PresignedGetURL generates a signed GET URL allowing secure downloading.
func (s *MinIOFileStore) PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	reqParams := make(url.Values)
	u, err := s.client.PresignedGetObject(ctx, s.bucketName, key, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned GET url: %w", err)
	}
	return u.String(), nil
}

// PutObject uploads a stream directly to storage.
func (s *MinIOFileStore) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucketName, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to put object %s: %w", key, err)
	}
	return nil
}

// GetObject retrieves an object stream directly from storage.
func (s *MinIOFileStore) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %w", key, err)
	}
	// Verify readable
	_, err = obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("failed to stat object %s: %w", key, err)
	}
	return obj, nil
}

// Delete removes an object from storage.
func (s *MinIOFileStore) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucketName, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object %s: %w", key, err)
	}
	return nil
}
