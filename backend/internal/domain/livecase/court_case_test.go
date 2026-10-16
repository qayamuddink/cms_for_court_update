package livecase

import (
	"strings"
	"testing"
)

func TestCourtCaseCanPostUpdate(t *testing.T) {
	listedCase := &CourtCase{Status: CaseStatusListed}
	if err := listedCase.CanPostUpdate(); err != nil {
		t.Errorf("expected listed case to allow updates, got: %v", err)
	}

	hearingCase := &CourtCase{Status: CaseStatusHearing}
	if err := hearingCase.CanPostUpdate(); err != nil {
		t.Errorf("expected hearing case to allow updates, got: %v", err)
	}

	reservedCase := &CourtCase{Status: CaseStatusReserved}
	if err := reservedCase.CanPostUpdate(); err != nil {
		t.Errorf("expected reserved case to allow updates, got: %v", err)
	}

	decidedCase := &CourtCase{Status: CaseStatusDecided}
	if err := decidedCase.CanPostUpdate(); err == nil {
		t.Errorf("expected decided case to reject updates")
	}

	disposedCase := &CourtCase{Status: CaseStatusDisposed}
	if err := disposedCase.CanPostUpdate(); err == nil {
		t.Errorf("expected disposed case to reject updates")
	}
}

func TestSanitizeContent(t *testing.T) {
	// 1. Valid plain text
	plain := "Advocate submits that the fundamental right has been violated."
	sanitized, err := SanitizeContent(plain)
	if err != nil {
		t.Fatalf("expected valid text to pass, got err: %v", err)
	}
	if sanitized != plain {
		t.Errorf("expected sanitized text to match original, got %q", sanitized)
	}

	// 2. HTML escaping
	htmlSnippet := "<script>alert('xss')</script><b>Hearing resumed</b>"
	sanitizedHTML, err := SanitizeContent(htmlSnippet)
	if err != nil {
		t.Fatalf("expected HTML snippet to sanitize, got err: %v", err)
	}
	if strings.Contains(sanitizedHTML, "<script>") {
		t.Errorf("expected <script> to be escaped, got %q", sanitizedHTML)
	}

	// 3. Empty content
	if _, err := SanitizeContent("   "); err == nil {
		t.Errorf("expected empty content to be rejected")
	}

	// 4. Overly long content (> 5000 characters)
	longText := strings.Repeat("A", 5001)
	if _, err := SanitizeContent(longText); err == nil {
		t.Errorf("expected text over 5000 chars to be rejected")
	}

	// 5. Invalid UTF-8
	invalidUTF8 := string([]byte{0xff, 0xfe, 0xfd})
	if _, err := SanitizeContent(invalidUTF8); err == nil {
		t.Errorf("expected invalid UTF-8 to be rejected")
	}
}

func TestValidateLiveUpdateParams(t *testing.T) {
	// Valid argument
	if _, err := ValidateLiveUpdateParams("Argument presented", UpdateTypeArgument); err != nil {
		t.Errorf("expected valid update type to pass, got: %v", err)
	}

	// Invalid update type
	if _, err := ValidateLiveUpdateParams("Some text", UpdateType("unrecognized")); err == nil {
		t.Errorf("expected invalid update type to fail")
	}
}

func TestCanCorrectAndRetractUpdate(t *testing.T) {
	published := &LiveUpdate{
		ID:         "up1",
		Status:     UpdateStatusPublished,
		SequenceNo: 1,
	}

	if err := CanCorrectUpdate(published); err != nil {
		t.Errorf("expected published update to be correctable, got: %v", err)
	}
	if err := CanRetractUpdate(published); err != nil {
		t.Errorf("expected published update to be retractable, got: %v", err)
	}

	retracted := &LiveUpdate{
		ID:         "up2",
		Status:     UpdateStatusRetracted,
		SequenceNo: 2,
	}

	// No edits after retraction!
	if err := CanCorrectUpdate(retracted); err == nil {
		t.Errorf("expected error: no corrections allowed on retracted update")
	}
	if err := CanRetractUpdate(retracted); err == nil {
		t.Errorf("expected error: cannot retract already retracted update")
	}
}
