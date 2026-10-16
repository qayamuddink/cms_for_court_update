
package queries

import (
	"context"

	"github.com/livelaw/backend/internal/application/ports"
)

// FollowQueries handles read queries for case follow subscriptions.
type FollowQueries struct {
	followRepo ports.CaseFollowRepository
}

// NewFollowQueries creates a new FollowQueries instance.
func NewFollowQueries(followRepo ports.CaseFollowRepository) *FollowQueries {
	return &FollowQueries{followRepo: followRepo}
}

// CaseFollowStatusResponse describes the user's follow state for a case.
type CaseFollowStatusResponse struct {
	IsFollowing   bool     `json:"is_following"`
	NotifyOn      []string `json:"notify_on"`
	FollowerCount int      `json:"follower_count"`
}

// GetCaseFollowStatus retrieves the follow status and follower count for a case.
func (q *FollowQueries) GetCaseFollowStatus(ctx context.Context, userID, caseID string) (*CaseFollowStatusResponse, error) {
	count, err := q.followRepo.CountFollowers(ctx, caseID)
	if err != nil {
		return nil, err
	}

	if userID == "" {
		return &CaseFollowStatusResponse{
			IsFollowing:   false,
			NotifyOn:      []string{},
			FollowerCount: count,
		}, nil
	}

	follow, err := q.followRepo.GetFollow(ctx, userID, caseID)
	if err != nil {
		return nil, err
	}

	if follow == nil {
		return &CaseFollowStatusResponse{
			IsFollowing:   false,
			NotifyOn:      []string{},
			FollowerCount: count,
		}, nil
	}

	return &CaseFollowStatusResponse{
		IsFollowing:   true,
		NotifyOn:      follow.NotifyOn,
		FollowerCount: count,
	}, nil
}
