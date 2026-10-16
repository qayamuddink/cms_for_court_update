package identity

import "testing"

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"TEST@Example.Com", "test@example.com"},
		{"  reporter@LiveLaw.in  ", "reporter@livelaw.in"},
		{"user@Domain.org", "user@domain.org"},
	}

	for _, tt := range tests {
		actual := NormalizeEmail(tt.input)
		if actual != tt.expected {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, actual, tt.expected)
		}
	}
}

func TestUserCanLoginAndRefresh(t *testing.T) {
	activeUser := &User{Status: StatusActive}
	if err := activeUser.CanLogin(); err != nil {
		t.Errorf("expected active user to be able to login, got: %v", err)
	}
	if err := activeUser.CanRefresh(); err != nil {
		t.Errorf("expected active user to be able to refresh, got: %v", err)
	}

	suspendedUser := &User{Status: StatusSuspended}
	if err := suspendedUser.CanLogin(); err == nil {
		t.Errorf("expected suspended user login to fail")
	}
	if err := suspendedUser.CanRefresh(); err == nil {
		t.Errorf("expected suspended user refresh to fail")
	}

	deletedUser := &User{Status: StatusDeleted}
	if err := deletedUser.CanLogin(); err == nil {
		t.Errorf("expected deleted user login to fail")
	}
	if err := deletedUser.CanRefresh(); err == nil {
		t.Errorf("expected deleted user refresh to fail")
	}
}

func TestValidatePassword_Reader(t *testing.T) {
	// Reader passwords: minimum 8 characters
	if err := ValidatePassword("short", RoleReader); err == nil {
		t.Errorf("expected error for reader password < 8 chars")
	}
	if err := ValidatePassword("simple123", RoleReader); err != nil {
		t.Errorf("expected valid reader password to pass, got: %v", err)
	}
}

func TestValidatePassword_Staff(t *testing.T) {
	roles := []SystemRole{RoleReporter, RoleEditor, RoleAdmin}

	for _, r := range roles {
		// Too short (< 12)
		if err := ValidatePassword("Pass123!", r); err == nil {
			t.Errorf("role %s: expected error for staff password < 12 chars", r)
		}

		// Missing uppercase
		if err := ValidatePassword("password123!", r); err == nil {
			t.Errorf("role %s: expected error for missing uppercase", r)
		}

		// Missing lowercase
		if err := ValidatePassword("PASSWORD123!", r); err == nil {
			t.Errorf("role %s: expected error for missing lowercase", r)
		}

		// Missing digit
		if err := ValidatePassword("PasswordSpecial!", r); err == nil {
			t.Errorf("role %s: expected error for missing digit", r)
		}

		// Missing special character
		if err := ValidatePassword("Password123456", r); err == nil {
			t.Errorf("role %s: expected error for missing special character", r)
		}

		// Valid staff password
		if err := ValidatePassword("StrongStaffPass123!", r); err != nil {
			t.Errorf("role %s: expected valid staff password to pass, got: %v", r, err)
		}
	}
}

func TestAuthorizeAdminActions(t *testing.T) {
	if err := AuthorizeAdminAction(RoleAdmin); err != nil {
		t.Errorf("expected admin to be authorized, got: %v", err)
	}
	if err := AuthorizeAdminAction(RoleEditor); err == nil {
		t.Errorf("expected editor to be rejected for admin action")
	}
	if err := AuthorizeAdminAction(RoleReporter); err == nil {
		t.Errorf("expected reporter to be rejected for admin action")
	}
	if err := AuthorizeAdminAction(RoleReader); err == nil {
		t.Errorf("expected reader to be rejected for admin action")
	}

	// Create staff
	if err := AuthorizeStaffCreation(RoleAdmin, RoleReporter); err != nil {
		t.Errorf("expected admin to be able to create reporter, got: %v", err)
	}
	if err := AuthorizeStaffCreation(RoleAdmin, RoleReader); err == nil {
		t.Errorf("expected admin staff creation to reject non-staff role reader")
	}

	// Change role
	if err := AuthorizeRoleChange(RoleAdmin, RoleEditor); err != nil {
		t.Errorf("expected admin to be able to change role, got: %v", err)
	}
	if err := AuthorizeRoleChange(RoleAdmin, "superhero"); err == nil {
		t.Errorf("expected invalid target role to be rejected")
	}
}
