package identity

import (
	"errors"
	"strings"
	"time"
	"unicode"
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
	ID               string
	Email            string
	PasswordHash     *string
	AuthProvider     string
	FullName         string
	DisplayRole      DisplayRole
	SystemRole       SystemRole
	BarCouncilNumber *string
	Phone            *string
	EmailVerifiedAt  *time.Time
	Status           UserStatus
	LastLoginAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NormalizeEmail converts email to lowercase and trims whitespace.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// IsValidSystemRole checks if the role string is a recognized system role.
func IsValidSystemRole(role SystemRole) bool {
	switch role {
	case RoleReader, RoleReporter, RoleEditor, RoleAdmin:
		return true
	default:
		return false
	}
}

// IsStaffRole checks whether the role has staff privileges (reporter, editor, admin).
func IsStaffRole(role SystemRole) bool {
	switch role {
	case RoleReporter, RoleEditor, RoleAdmin:
		return true
	default:
		return false
	}
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

// CanRefresh checks if the user account is allowed to refresh tokens.
func (u *User) CanRefresh() error {
	return u.CanLogin()
}

// ValidatePassword enforces password strength policies based on role.
// Readers require at least 8 characters.
// Staff (reporters, editors, admins) require at least 12 characters, uppercase, lowercase, digit, and symbol.
func ValidatePassword(password string, role SystemRole) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	if IsStaffRole(role) {
		if len(password) < 12 {
			return errors.New("staff passwords must be at least 12 characters long")
		}

		var hasUpper, hasLower, hasDigit, hasSpecial bool
		for _, r := range password {
			switch {
			case unicode.IsUpper(r):
				hasUpper = true
			case unicode.IsLower(r):
				hasLower = true
			case unicode.IsDigit(r):
				hasDigit = true
			case unicode.IsPunct(r) || unicode.IsSymbol(r):
				hasSpecial = true
			}
		}

		if !hasUpper {
			return errors.New("staff password must contain at least one uppercase letter")
		}
		if !hasLower {
			return errors.New("staff password must contain at least one lowercase letter")
		}
		if !hasDigit {
			return errors.New("staff password must contain at least one digit")
		}
		if !hasSpecial {
			return errors.New("staff password must contain at least one special character")
		}
	}

	return nil
}

// AuthorizeAdminAction verifies that the acting user has administrative privileges.
func AuthorizeAdminAction(actorRole SystemRole) error {
	if actorRole != RoleAdmin {
		return errors.New("only administrators can perform this action")
	}
	return nil
}

// AuthorizeStaffCreation verifies that the actor can create an account with the specified role.
func AuthorizeStaffCreation(actorRole SystemRole, targetRole SystemRole) error {
	if err := AuthorizeAdminAction(actorRole); err != nil {
		return err
	}
	if !IsStaffRole(targetRole) {
		return errors.New("can only create staff roles (reporter, editor, admin)")
	}
	return nil
}

// AuthorizeRoleChange verifies that the actor can assign targetRole.
func AuthorizeRoleChange(actorRole SystemRole, targetRole SystemRole) error {
	if err := AuthorizeAdminAction(actorRole); err != nil {
		return err
	}
	if !IsValidSystemRole(targetRole) {
		return errors.New("invalid target system role")
	}
	return nil
}
