package ports

import (
	"context"
	"errors"

	"github.com/livelaw/backend/internal/domain/content"
)

var (
	ErrSectionNotFound    = errors.New("section not found")
	ErrSectionSlugExists  = errors.New("a section with this slug already exists")
)

// SectionRepository defines persistence operations for taxonomy sections.
type SectionRepository interface {
	Create(ctx context.Context, s *content.Section) error
	GetByID(ctx context.Context, id string) (*content.Section, error)
	GetBySlug(ctx context.Context, slug string) (*content.Section, error)
	Update(ctx context.Context, s *content.Section) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, activeOnly bool) ([]content.Section, error)
}
