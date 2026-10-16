package identity

import (
	"errors"
	"strings"
	"time"
)

// SystemRole defines the system authorization level.
type SystemRole string

const (
	RoleReader   SystemRole = "reader"
	RoleReporter SystemRole = "reporter"
	RoleEditor   SystemRole = "editor"
	RoleAdmin    SystemRole = "admin"
)

// DisplayRole defines the public or professional persona.
type DisplayRole string

const (
	DisplayPublic  DisplayRole = "public"
	DisplayStudent DisplayRole = "student"
	DisplayLawyer  DisplayRole = "lawyer"
	DisplayJudge   DisplayRole = "judge"
)

// UserStatus represents user account lifecycle state.
type UserStatus string

const (
	StatusActive    UserStatus = "active"
	StatusSuspended UserStatus = "suspended"
	StatusDeleted   UserStatus = "deleted"
)

// User represents the identity domain entity.
type User struct {
	ID                string
	Email             string
	PasswordHash      *string
	AuthProvider      string
	FullName          string
	DisplayRole       DisplayRole
	SystemRole        SystemRole
	BarCouncilNumber  *string
	Phone             *string
	EmailVerifiedAt   *time.Time
	Status            UserStatus
	LastLoginAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NormalizeEmail converts email to lowercase and trims whitespace.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// CanLogin checks if the user account is allowed to authenticate.
func (u *User) CanLogin() error {
	if u.Status == StatusSuspended {
		return errors.New("user account is suspended")
	}
	if u.Status == StatusDeleted {
		return errors.New("user account has been deleted")
	}
	return nil
}
