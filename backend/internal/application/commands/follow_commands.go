package commands

import (
	"context"
	"errors"

	"github.com/livelaw/backend/internal/application/ports"
)

var (
	ErrInvalidPlatform = errors.New("platform must be ios, android, or web")
	ErrEmptyPushToken  = errors.New("push token cannot be empty")
)

// FollowUseCases manages user follows for court cases and device token push registrations.
type FollowUseCases struct {
	followRepo ports.CaseFollowRepository
	deviceRepo ports.DeviceTokenRepository
	caseRepo   ports.CourtCaseRepository
	idGen      ports.IDGenerator
	clock      ports.Clock
}

// NewFollowUseCases creates a new FollowUseCases instance.
func NewFollowUseCases(
	followRepo ports.CaseFollowRepository,
	deviceRepo ports.DeviceTokenRepository,
	caseRepo ports.CourtCaseRepository,
	idGen ports.IDGenerator,
	clock ports.Clock,
) *FollowUseCases {
	return &FollowUseCases{
		followRepo: followRepo,
		deviceRepo: deviceRepo,
		caseRepo:   caseRepo,
		idGen:      idGen,
		clock:      clock,
	}
}

// FollowCase registers a reader to receive push alerts on case updates.
func (uc *FollowUseCases) FollowCase(ctx context.Context, userID, caseID string, notifyOn []string) error {
	c, err := uc.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return err
	}
	if c == nil {
		return ports.ErrCaseNotFound
	}

	if len(notifyOn) == 0 {
		notifyOn = []string{"verdict", "order"}
	}

	follow := &ports.CaseFollow{
		ID:          uc.idGen.NewID(),
		UserID:      userID,
		CourtCaseID: caseID,
		NotifyOn:    notifyOn,
		CreatedAt:   uc.clock.Now(),
	}

	return uc.followRepo.Follow(ctx, follow)
}

// UnfollowCase removes a reader's follow subscription from a case.
func (uc *FollowUseCases) UnfollowCase(ctx context.Context, userID, caseID string) error {
	return uc.followRepo.Unfollow(ctx, userID, caseID)
}

// RegisterDevice registers or updates a device push token for a user.
func (uc *FollowUseCases) RegisterDevice(ctx context.Context, userID, platform, pushToken string) error {
	if pushToken == "" {
		return ErrEmptyPushToken
	}

	switch platform {
	case "ios", "android", "web":
	default:
		return ErrInvalidPlatform
	}

	token := &ports.DeviceToken{
		ID:         uc.idGen.NewID(),
		UserID:     userID,
		Platform:   platform,
		PushToken:  pushToken,
		LastSeenAt: uc.clock.Now(),
	}

	return uc.deviceRepo.Upsert(ctx, token)
}

// UnregisterDevice removes a device push token.
func (uc *FollowUseCases) UnregisterDevice(ctx context.Context, userID, pushToken string) error {
	return uc.deviceRepo.Delete(ctx, userID, pushToken)
}
