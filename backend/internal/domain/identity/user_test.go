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

func TestUserCanLogin(t *testing.T) {
	activeUser := &User{Status: StatusActive}
	if err := activeUser.CanLogin(); err != nil {
		t.Errorf("expected active user to be able to login, got err: %v", err)
	}

	suspendedUser := &User{Status: StatusSuspended}
	if err := suspendedUser.CanLogin(); err == nil {
		t.Errorf("expected suspended user login to fail")
	}

	deletedUser := &User{Status: StatusDeleted}
	if err := deletedUser.CanLogin(); err == nil {
		t.Errorf("expected deleted user login to fail")
	}
}
