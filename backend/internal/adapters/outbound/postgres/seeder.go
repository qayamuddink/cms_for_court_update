package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/identity"
)

// SeedUser holds definition for a seeded default user.
type SeedUser struct {
	Email       string
	Password    string
	FullName    string
	DisplayRole identity.DisplayRole
	SystemRole  identity.SystemRole
}

// DefaultSeedUsers defines the canonical four seeded users (one per role).
var DefaultSeedUsers = []SeedUser{
	{
		Email:       "reader@livelaw.in",
		Password:    "ReaderPass123!",
		FullName:    "Standard Reader",
		DisplayRole: identity.DisplayPublic,
		SystemRole:  identity.RoleReader,
	},
	{
		Email:       "reporter@livelaw.in",
		Password:    "ReporterPass123!",
		FullName:    "Court Reporter",
		DisplayRole: identity.DisplayLawyer,
		SystemRole:  identity.RoleReporter,
	},
	{
		Email:       "editor@livelaw.in",
		Password:    "EditorPass123!",
		FullName:    "Chief Editor",
		DisplayRole: identity.DisplayLawyer,
		SystemRole:  identity.RoleEditor,
	},
	{
		Email:       "admin@livelaw.in",
		Password:    "AdminPass123!",
		FullName:    "Platform Admin",
		DisplayRole: identity.DisplayLawyer,
		SystemRole:  identity.RoleAdmin,
	},
}

// Seeder provisions baseline database records.
type Seeder struct {
	userRepo ports.UserRepository
	hasher   ports.PasswordHasher
	idGen    ports.IDGenerator
	clock    ports.Clock
	logger   *slog.Logger
}

// NewSeeder creates a new Seeder.
func NewSeeder(
	userRepo ports.UserRepository,
	hasher ports.PasswordHasher,
	idGen ports.IDGenerator,
	clock ports.Clock,
	logger *slog.Logger,
) *Seeder {
	return &Seeder{
		userRepo: userRepo,
		hasher:   hasher,
		idGen:    idGen,
		clock:    clock,
		logger:   logger,
	}
}

// SeedUsers provisions the default accounts if they do not already exist.
func (s *Seeder) SeedUsers(ctx context.Context) error {
	now := s.clock.Now()

	for _, su := range DefaultSeedUsers {
		normEmail := identity.NormalizeEmail(su.Email)
		existing, err := s.userRepo.GetByEmail(ctx, normEmail)
		if err != nil {
			return fmt.Errorf("failed checking existing user %s: %w", normEmail, err)
		}
		if existing != nil {
			s.logger.Info("seed user already exists, skipping", slog.String("email", normEmail))
			continue
		}

		hash, err := s.hasher.Hash(su.Password)
		if err != nil {
			return fmt.Errorf("failed hashing password for seed user %s: %w", normEmail, err)
		}

		user := &identity.User{
			ID:              s.idGen.NewID(),
			Email:           normEmail,
			PasswordHash:    &hash,
			AuthProvider:    "local",
			FullName:        su.FullName,
			DisplayRole:     su.DisplayRole,
			SystemRole:      su.SystemRole,
			EmailVerifiedAt: &now,
			Status:          identity.StatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			return fmt.Errorf("failed creating seed user %s: %w", normEmail, err)
		}

		s.logger.Info("seeded user created successfully",
			slog.String("email", normEmail),
			slog.String("role", string(su.SystemRole)),
		)
	}

	return nil
}
