package ports

import (
	"context"

	"github.com/livelaw/backend/internal/domain/content"
)

// EntitlementChecker defines the boundary for checking reader access to premium content.
type EntitlementChecker interface {
	// HasArticleAccess returns true if the user is authorized to read the full body of the article.
	HasArticleAccess(ctx context.Context, userID *string, article *content.Article) (bool, error)
}
