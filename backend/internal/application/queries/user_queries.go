package queries

import (
	"context"
	"errors"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/identity"
)

var (
	ErrForbidden    = errors.New("forbidden: insufficient permissions")
	ErrUserNotFound = errors.New("user not found")
)

// UserQueries provides read operations for user profiles and administration.
type UserQueries struct {
	userRepo ports.UserRepository
}

// NewUserQueries creates a new UserQueries instance.
func NewUserQueries(userRepo ports.UserRepository) *UserQueries {
	return &UserQueries{userRepo: userRepo}
}

// GetMyProfile retrieves the profile for an authenticated user.
func (q *UserQueries) GetMyProfile(ctx context.Context, userID string) (*identity.User, error) {
	user, err := q.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// AdminListUsers allows administrators to list users using keyset pagination.
func (q *UserQueries) AdminListUsers(ctx context.Context, adminUserID string, limit int, beforeCreatedAt *time.Time, beforeID *string) ([]identity.User, error) {
	admin, err := q.userRepo.GetByID(ctx, adminUserID)
	if err != nil {
		return nil, err
	}
	if admin == nil || admin.SystemRole != identity.RoleAdmin {
		return nil, ErrForbidden
	}

	return q.userRepo.List(ctx, limit, beforeCreatedAt, beforeID)
}
