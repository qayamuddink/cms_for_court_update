package entitlement

import (
	"context"
	"database/sql"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
)

// SQLSubscriptionChecker implements ports.EntitlementChecker checking subscriptions and staff roles.
type SQLSubscriptionChecker struct {
	db       *sql.DB
	userRepo ports.UserRepository
}

// NewSQLSubscriptionChecker creates a new SQLSubscriptionChecker instance.
func NewSQLSubscriptionChecker(db *sql.DB, userRepo ports.UserRepository) *SQLSubscriptionChecker {
	return &SQLSubscriptionChecker{
		db:       db,
		userRepo: userRepo,
	}
}

// HasArticleAccess determines whether the given user can access the full body of the article.
func (c *SQLSubscriptionChecker) HasArticleAccess(ctx context.Context, userID *string, article *content.Article) (bool, error) {
	// Free articles are universally accessible
	if article.AccessLevel == content.AccessLevelFree {
		return true, nil
	}

	// Premium articles require authentication
	if userID == nil || *userID == "" {
		return false, nil
	}

	// Staff members (admin, editor, reporter) always have full access
	if c.userRepo != nil {
		u, err := c.userRepo.GetByID(ctx, *userID)
		if err == nil && u != nil {
			if u.SystemRole == "admin" || u.SystemRole == "editor" || u.SystemRole == "reporter" {
				return true, nil
			}
		}
	}

	// Check active subscription in database
	if c.db != nil {
		query := `
			SELECT EXISTS (
				SELECT 1 FROM subscriptions
				WHERE user_id = $1
				  AND status IN ('active', 'trialing')
				  AND current_period_end > $2
			);
		`
		var hasActiveSub bool
		err := c.db.QueryRowContext(ctx, query, *userID, time.Now().UTC()).Scan(&hasActiveSub)
		if err != nil {
			return false, err
		}
		return hasActiveSub, nil
	}

	return false, nil
}
