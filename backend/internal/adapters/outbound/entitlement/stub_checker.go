package entitlement

import (
	"context"
	"database/sql"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
)

// SQLSubscriptionChecker implements ports.EntitlementChecker checking subscriptions and staff roles.
// It prioritizes zero-DB fast-path checks using JWT AuthClaims before falling back to DB queries.
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

	// 1. FAST PATH: Zero-DB check via verified JWT AuthClaims in context
	if claims := ports.AuthClaimsFromContext(ctx); claims != nil && claims.UserID == *userID {
		// Staff always have access
		if claims.SystemRole == "admin" ||
			claims.SystemRole == "editor" ||
			claims.SystemRole == "editor_in_chief" ||
			claims.SystemRole == "senior_editor" ||
			claims.SystemRole == "reporter" ||
			claims.Entitlement.Tier == "staff" {
			return true, nil
		}

		// Valid active subscription claim inside JWT
		if claims.Entitlement.Tier != "free" && claims.Entitlement.Tier != "" && claims.Entitlement.ExpiresAt > time.Now().Unix() {
			return true, nil
		}
	}

	// 2. Fallback check: user account role from database
	if c.userRepo != nil {
		u, err := c.userRepo.GetByID(ctx, *userID)
		if err == nil && u != nil {
			if u.SystemRole == "admin" ||
				u.SystemRole == "editor" ||
				u.SystemRole == "editor_in_chief" ||
				u.SystemRole == "senior_editor" ||
				u.SystemRole == "reporter" {
				return true, nil
			}
		}
	}

	// 3. Fallback check: active subscription in PostgreSQL
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
