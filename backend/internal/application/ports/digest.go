package ports

import (
	"context"

	"github.com/livelaw/backend/internal/domain/digest"
)

// DigestFilter contains query filters for listing digests.
type DigestFilter struct {
	CourtCode  *string              `json:"court_code,omitempty"`
	Period     *digest.DigestPeriod `json:"period,omitempty"`
	DigestType *digest.DigestType   `json:"digest_type,omitempty"`
	Status     *digest.DigestStatus `json:"status,omitempty"`
	Year       *int                 `json:"year,omitempty"`
	Limit      int                  `json:"limit"`
	Offset     int                  `json:"offset"`
}

// DigestRepository defines storage operations for legal digests and roundups.
type DigestRepository interface {
	Create(ctx context.Context, d *digest.Digest) error
	GetByID(ctx context.Context, id string) (*digest.Digest, error)
	GetBySlug(ctx context.Context, slug string) (*digest.Digest, error)
	Update(ctx context.Context, d *digest.Digest) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter DigestFilter) ([]digest.Digest, int, error)
	SaveItems(ctx context.Context, digestID string, items []digest.DigestItem) error
	GetItems(ctx context.Context, digestID string) ([]digest.DigestItem, error)
}
