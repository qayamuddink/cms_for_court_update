package ports

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/livelaw/backend/internal/domain/content"
)

var (
	ErrArticleNotFound = errors.New("article not found")
	ErrSlugAlreadyExists = errors.New("an article with this slug already exists")
)

// ArticleFilter provides search, filtering, and keyset cursor options for querying articles.
type ArticleFilter struct {
	Status      *content.ArticleStatus
	ArticleType *content.ArticleType
	Category    *string
	Tag         *string
	AuthorID    *string
	CourtCaseID *string
	Search      *string
	Limit       int
	Cursor      *string
}

// ArticleRepository defines persistence operations for articles and judgment summaries.
type ArticleRepository interface {
	Create(ctx context.Context, article *content.Article) error
	GetByID(ctx context.Context, id string) (*content.Article, error)
	GetBySlug(ctx context.Context, slug string) (*content.Article, error)
	Update(ctx context.Context, article *content.Article, expectedVersion int) error
	Delete(ctx context.Context, id string) error
	ListFeed(ctx context.Context, filter ArticleFilter) ([]content.Article, *string, error)
	ListCMS(ctx context.Context, filter ArticleFilter) ([]content.Article, int, error)
	LinkJudgmentToCase(ctx context.Context, caseID string, articleID string) error
	GetJudgmentByCaseID(ctx context.Context, caseID string) (*content.Article, error)
	SlugExists(ctx context.Context, slug string, excludeID string) (bool, error)
}

// EncodeCursor creates an opaque, base64-encoded keyset pagination cursor from a timestamp and UUID.
func EncodeCursor(t time.Time, id string) string {
	raw := fmt.Sprintf("%s|%s", t.UTC().Format(time.RFC3339Nano), id)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses an opaque base64 keyset cursor into its timestamp and ID components.
func DecodeCursor(cursor string) (time.Time, string, error) {
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor encoding: %w", err)
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("invalid cursor format")
	}

	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		// Fallback to standard RFC3339
		t, err = time.Parse(time.RFC3339, parts[0])
		if err != nil {
			return time.Time{}, "", fmt.Errorf("invalid cursor timestamp: %w", err)
		}
	}

	return t.UTC(), parts[1], nil
}
