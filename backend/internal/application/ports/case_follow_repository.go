package ports

import (
	"context"
	"time"
)

// CaseFollow represents a reader's subscription to live case updates and verdict notifications.
type CaseFollow struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	CourtCaseID string    `json:"court_case_id"`
	NotifyOn    []string  `json:"notify_on"` // e.g. ["verdict", "order"]
	CreatedAt   time.Time `json:"created_at"`
}

// CaseFollowRepository defines persistence operations for case follows.
type CaseFollowRepository interface {
	Follow(ctx context.Context, follow *CaseFollow) error
	Unfollow(ctx context.Context, userID, caseID string) error
	IsFollowing(ctx context.Context, userID, caseID string) (bool, error)
	GetFollow(ctx context.Context, userID, caseID string) (*CaseFollow, error)
	GetFollowersByCaseID(ctx context.Context, caseID string, eventType string) ([]string, error)
	CountFollowers(ctx context.Context, caseID string) (int, error)
}
