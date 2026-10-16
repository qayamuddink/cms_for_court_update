package commands

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/identity"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserSuspended      = errors.New("user account is suspended")
	ErrUserDeleted        = errors.New("user account has been deleted")
	ErrEmailAlreadyExists = errors.New("email is already registered")
	ErrInvalidToken       = errors.New("invalid or expired refresh token")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden: insufficient permissions")
	ErrUserNotFound       = errors.New("user not found")
)

// AuthResult holds the authenticated user details and active token pair.
type AuthResult struct {
	User         *identity.User `json:"user"`
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	ExpiresAt    time.Time      `json:"expires_at"`
}

// AuthUseCases handles user authentication and staff administration operations.
type AuthUseCases struct {
	userRepo     ports.UserRepository
	tokenRepo    ports.RefreshTokenRepository
	hasher       ports.PasswordHasher
	tokenService ports.TokenService
	idGen        ports.IDGenerator
	clock        ports.Clock
	subRepo      ports.SubscriptionRepository
}

// NewAuthUseCases creates a new AuthUseCases instance.
func NewAuthUseCases(
	userRepo ports.UserRepository,
	tokenRepo ports.RefreshTokenRepository,
	hasher ports.PasswordHasher,
	tokenService ports.TokenService,
	idGen ports.IDGenerator,
	clock ports.Clock,
	subRepo ...ports.SubscriptionRepository,
) *AuthUseCases {
	var sr ports.SubscriptionRepository
	if len(subRepo) > 0 {
		sr = subRepo[0]
	}
	return &AuthUseCases{
		userRepo:     userRepo,
		tokenRepo:    tokenRepo,
		hasher:       hasher,
		tokenService: tokenService,
		idGen:        idGen,
		clock:        clock,
		subRepo:      sr,
	}
}

// RegisterCommand carries input for reader registration.
type RegisterCommand struct {
	Email            string
	Password         string
	FullName         string
	DisplayRole      identity.DisplayRole
	BarCouncilNumber *string
	Phone            *string
}

// Register registers a new reader account.
func (uc *AuthUseCases) Register(ctx context.Context, cmd RegisterCommand) (*AuthResult, error) {
	normEmail := identity.NormalizeEmail(cmd.Email)
	if normEmail == "" {
		return nil, errors.New("email is required")
	}
	if cmd.FullName == "" {
		return nil, errors.New("full name is required")
	}

	if err := identity.ValidatePassword(cmd.Password, identity.RoleReader); err != nil {
		return nil, err
	}

	existing, err := uc.userRepo.GetByEmail(ctx, normEmail)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	hash, err := uc.hasher.Hash(cmd.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := uc.clock.Now()
	displayRole := cmd.DisplayRole
	if displayRole == "" {
		displayRole = identity.DisplayPublic
	}

	user := &identity.User{
		ID:               uc.idGen.NewID(),
		Email:            normEmail,
		PasswordHash:     &hash,
		AuthProvider:     "local",
		FullName:         cmd.FullName,
		DisplayRole:      displayRole,
		SystemRole:       identity.RoleReader,
		BarCouncilNumber: cmd.BarCouncilNumber,
		Phone:            cmd.Phone,
		Status:           identity.StatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return uc.generateTokenPair(ctx, user)
}

// LoginCommand carries credentials for authentication.
type LoginCommand struct {
	Email    string
	Password string
}

// Login authenticates a user and returns access and refresh tokens.
func (uc *AuthUseCases) Login(ctx context.Context, cmd LoginCommand) (*AuthResult, error) {
	normEmail := identity.NormalizeEmail(cmd.Email)
	user, err := uc.userRepo.GetByEmail(ctx, normEmail)
	if err != nil {
		return nil, err
	}
	if user == nil || user.PasswordHash == nil {
		return nil, ErrInvalidCredentials
	}

	if err := uc.hasher.Compare(*user.PasswordHash, cmd.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := user.CanLogin(); err != nil {
		if user.Status == identity.StatusSuspended {
			return nil, ErrUserSuspended
		}
		return nil, ErrUserDeleted
	}

	now := uc.clock.Now()
	_ = uc.userRepo.UpdateLastLogin(ctx, user.ID, now)
	user.LastLoginAt = &now

	return uc.generateTokenPair(ctx, user)
}

// RefreshCommand carries the raw refresh token for rotation.
type RefreshCommand struct {
	RefreshToken string
}

// RefreshToken validates and rotates a refresh token, issuing a new token pair.
func (uc *AuthUseCases) RefreshToken(ctx context.Context, cmd RefreshCommand) (*AuthResult, error) {
	if cmd.RefreshToken == "" {
		return nil, ErrInvalidToken
	}

	tokenHash := uc.tokenService.HashRefreshToken(cmd.RefreshToken)
	stored, err := uc.tokenRepo.GetByHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, ErrInvalidToken
	}

	now := uc.clock.Now()
	if stored.RevokedAt != nil || now.After(stored.ExpiresAt) {
		return nil, ErrInvalidToken
	}

	user, err := uc.userRepo.GetByID(ctx, stored.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	if err := user.CanRefresh(); err != nil {
		if user.Status == identity.StatusSuspended {
			return nil, ErrUserSuspended
		}
		return nil, ErrUserDeleted
	}

	// Revoke the old token (rotation)
	if err := uc.tokenRepo.Revoke(ctx, tokenHash, now); err != nil {
		return nil, err
	}

	// Generate and return new token pair
	return uc.generateTokenPair(ctx, user)
}

// LogoutCommand carries the refresh token to revoke.
type LogoutCommand struct {
	RefreshToken string
}

// Logout revokes the provided refresh token.
func (uc *AuthUseCases) Logout(ctx context.Context, cmd LogoutCommand) error {
	if cmd.RefreshToken == "" {
		return nil
	}
	tokenHash := uc.tokenService.HashRefreshToken(cmd.RefreshToken)
	return uc.tokenRepo.Revoke(ctx, tokenHash, uc.clock.Now())
}

// AdminCreateStaffCommand carries fields for creating staff accounts.
type AdminCreateStaffCommand struct {
	AdminUserID      string
	Email            string
	Password         string
	FullName         string
	SystemRole       identity.SystemRole
	DisplayRole      identity.DisplayRole
	BarCouncilNumber *string
	Phone            *string
}

// AdminCreateStaff allows administrators to provision staff accounts.
func (uc *AuthUseCases) AdminCreateStaff(ctx context.Context, cmd AdminCreateStaffCommand) (*identity.User, error) {
	admin, err := uc.userRepo.GetByID(ctx, cmd.AdminUserID)
	if err != nil {
		return nil, err
	}
	if admin == nil || admin.SystemRole != identity.RoleAdmin {
		return nil, ErrForbidden
	}

	if err := identity.AuthorizeStaffCreation(admin.SystemRole, cmd.SystemRole); err != nil {
		return nil, err
	}

	if err := identity.ValidatePassword(cmd.Password, cmd.SystemRole); err != nil {
		return nil, err
	}

	normEmail := identity.NormalizeEmail(cmd.Email)
	existing, err := uc.userRepo.GetByEmail(ctx, normEmail)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	hash, err := uc.hasher.Hash(cmd.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash staff password: %w", err)
	}

	now := uc.clock.Now()
	displayRole := cmd.DisplayRole
	if displayRole == "" {
		displayRole = identity.DisplayLawyer
	}

	staff := &identity.User{
		ID:               uc.idGen.NewID(),
		Email:            normEmail,
		PasswordHash:     &hash,
		AuthProvider:     "local",
		FullName:         cmd.FullName,
		DisplayRole:      displayRole,
		SystemRole:       cmd.SystemRole,
		BarCouncilNumber: cmd.BarCouncilNumber,
		Phone:            cmd.Phone,
		Status:           identity.StatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := uc.userRepo.Create(ctx, staff); err != nil {
		return nil, err
	}

	return staff, nil
}

// AdminUpdateUserCommand carries updates for user role or status.
type AdminUpdateUserCommand struct {
	AdminUserID  string
	TargetUserID string
	SystemRole   *identity.SystemRole
	Status       *identity.UserStatus
}

// AdminUpdateUser allows administrators to change user roles or suspend accounts.
func (uc *AuthUseCases) AdminUpdateUser(ctx context.Context, cmd AdminUpdateUserCommand) (*identity.User, error) {
	admin, err := uc.userRepo.GetByID(ctx, cmd.AdminUserID)
	if err != nil {
		return nil, err
	}
	if admin == nil || admin.SystemRole != identity.RoleAdmin {
		return nil, ErrForbidden
	}

	target, err := uc.userRepo.GetByID(ctx, cmd.TargetUserID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, ErrUserNotFound
	}

	now := uc.clock.Now()
	target.UpdatedAt = now

	if cmd.SystemRole != nil {
		if err := identity.AuthorizeRoleChange(admin.SystemRole, *cmd.SystemRole); err != nil {
			return nil, err
		}
		target.SystemRole = *cmd.SystemRole
	}

	if cmd.Status != nil {
		target.Status = *cmd.Status
		// If user is being suspended, revoke all active refresh tokens immediately
		if *cmd.Status == identity.StatusSuspended || *cmd.Status == identity.StatusDeleted {
			_ = uc.tokenRepo.RevokeAllForUser(ctx, target.ID, now)
		}
	}

	if err := uc.userRepo.Update(ctx, target); err != nil {
		return nil, err
	}

	return target, nil
}

func (uc *AuthUseCases) generateTokenPair(ctx context.Context, user *identity.User) (*AuthResult, error) {
	now := uc.clock.Now()
	entitlement := ports.EntitlementClaim{
		Tier:      "free",
		ExpiresAt: 0,
	}

	// Staff members get long-lived staff entitlement
	if user.SystemRole == identity.RoleAdmin ||
		user.SystemRole == identity.RoleEditor ||
		user.SystemRole == identity.RoleReporter {
		entitlement = ports.EntitlementClaim{
			Tier:      "staff",
			ExpiresAt: now.Add(365 * 24 * time.Hour).Unix(),
		}
	} else if uc.subRepo != nil {
		// Readers: check active subscription
		sub, err := uc.subRepo.GetActiveByUserID(ctx, user.ID)
		if err == nil && sub != nil && sub.IsEntitled(now) {
			entitlement = ports.EntitlementClaim{
				Tier:      string(sub.Tier),
				ExpiresAt: sub.CurrentPeriodEnd.Unix(),
			}
		}
	}

	accessToken, accessExpiresAt, err := uc.tokenService.GenerateAccessToken(user, entitlement)
	if err != nil {
		return nil, err
	}

	rawRefresh, tokenHash, refreshExpiresAt, err := uc.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	tokenRecord := &ports.RefreshToken{
		ID:        uc.idGen.NewID(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: refreshExpiresAt,
		CreatedAt: now,
	}

	if err := uc.tokenRepo.Create(ctx, tokenRecord); err != nil {
		return nil, err
	}

	return &AuthResult{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    accessExpiresAt,
	}, nil
}
