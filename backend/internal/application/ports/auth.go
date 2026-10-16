package ports

import (
	"context"
	"time"

	"github.com/livelaw/backend/internal/domain/identity"
)

// PasswordHasher provides secure password hashing and verification.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

// EntitlementClaim represents the subscription status attached to the access token.
type EntitlementClaim struct {
	Tier      string `json:"tier"`
	ExpiresAt int64  `json:"expires_at"`
}

// AuthClaims represents decoded JWT access token claims.
type AuthClaims struct {
	UserID      string           `json:"user_id"`
	Email       string           `json:"email"`
	SystemRole  string           `json:"system_role"`
	Entitlement EntitlementClaim `json:"entitlement"`
}

// TokenService issues and verifies access and refresh tokens.
type TokenService interface {
	GenerateAccessToken(user *identity.User, entitlement EntitlementClaim) (string, time.Time, error)
	GenerateRefreshToken() (string, string, time.Time, error)
	ValidateAccessToken(tokenString string) (*AuthClaims, error)
	HashRefreshToken(rawToken string) string
}

// RefreshToken model representing active or revoked refresh tokens.
type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// UserRepository defines persistence operations for User entities.
type UserRepository interface {
	Create(ctx context.Context, user *identity.User) error
	GetByID(ctx context.Context, id string) (*identity.User, error)
	GetByEmail(ctx context.Context, email string) (*identity.User, error)
	Update(ctx context.Context, user *identity.User) error
	UpdateLastLogin(ctx context.Context, id string, lastLogin time.Time) error
	List(ctx context.Context, limit int, beforeCreatedAt *time.Time, beforeID *string) ([]identity.User, error)
}

// RefreshTokenRepository defines persistence operations for RefreshToken entities.
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)
	Revoke(ctx context.Context, hash string, revokedAt time.Time) error
	RevokeAllForUser(ctx context.Context, userID string, revokedAt time.Time) error
}
