package judgment

import (
	"testing"
	"time"
)

func TestJudgmentCreationAndValidation(t *testing.T) {
	now := time.Now().UTC()

	// Empty title
	_, err := NewJudgment("j1", "", "CA 101/2026", "Supreme Court of India", "SC",
		[]string{"Judge A"}, now, nil, "Appellant X", "Respondent Y", nil, nil, nil,
		"", "", "", "Summary", nil, nil, nil, AccessLevelFree, "user1", now)
	if err != ErrInvalidCaseTitle {
		t.Fatalf("expected ErrInvalidCaseTitle, got %v", err)
	}

	// Valid creation
	j, err := NewJudgment("j1", "Union of India v. Association", "CA 101/2026", "Supreme Court of India", "",
		[]string{"Justice D.Y. Chandrachud", "Justice Sanjiv Khanna"}, now, nil,
		"Union of India", "Association of Democratic Reforms", []string{"Sr. Adv. Kapil Sibal"},
		[]StatuteCited{
			{ID: "s1", JudgmentID: "j1", StatuteName: "Constitution of India", SectionsCited: []string{"Article 19", "Article 21"}},
		},
		[]string{"Constitutional Law", "Electoral Bonds"}, "Whether scheme violates Right to Information",
		"Scheme struck down as unconstitutional", "Right to privacy does not extend to political funding anonymity",
		"Detailed legal summary of unanimous ruling", []string{"Electoral Bonds unconstitutional", "SBI must disclose data"},
		nil, nil, AccessLevelPremium, "user1", now)
	if err != nil {
		t.Fatalf("unexpected error creating judgment: %v", err)
	}

	if j.CourtCode != "SC" {
		t.Errorf("expected court code auto-normalized to 'SC', got %q", j.CourtCode)
	}
	if j.Status != StatusDraft {
		t.Errorf("expected StatusDraft, got %s", j.Status)
	}
	if j.AccessLevel != AccessLevelPremium {
		t.Errorf("expected AccessLevelPremium, got %s", j.AccessLevel)
	}
	if len(j.Bench) != 2 {
		t.Errorf("expected 2 bench judges, got %d", len(j.Bench))
	}
	if len(j.StatutesCited) != 1 {
		t.Errorf("expected 1 statute cited, got %d", len(j.StatutesCited))
	}
}

func TestCitationFormattingAndPublish(t *testing.T) {
	now := time.Now().UTC()
	j, err := NewJudgment("j1", "Case Title", "CA 1/2026", "Supreme Court of India", "SC",
		[]string{"Judge A"}, now, nil, "Appellant", "Respondent", nil, nil, nil,
		"Issues", "Held", "Headnote", "Summary", nil, nil, nil, AccessLevelFree, "user1", now)
	if err != nil {
		t.Fatalf("failed to create judgment: %v", err)
	}

	// FormatCitation
	cit := FormatCitation(2026, "CourtDesk", "SC", 1)
	if cit != "2026 CourtDesk (SC) 1" {
		t.Errorf("expected '2026 CourtDesk (SC) 1', got %q", cit)
	}

	// FormatCitation with custom brand prefix
	citCustom := FormatCitation(2026, "LiveLegal", "DEL", 125)
	if citCustom != "2026 LiveLegal (DEL) 125" {
		t.Errorf("expected '2026 LiveLegal (DEL) 125', got %q", citCustom)
	}

	// Publish
	err = j.Publish("CourtDesk", 2026, "SC", 42, now)
	if err != nil {
		t.Fatalf("unexpected error on publish: %v", err)
	}
	if j.Status != StatusPublished {
		t.Errorf("expected StatusPublished, got %s", j.Status)
	}
	if j.Citation == nil || *j.Citation != "2026 CourtDesk (SC) 42" {
		t.Errorf("expected citation '2026 CourtDesk (SC) 42', got %v", j.Citation)
	}

	// Duplicate publish
	err = j.Publish("CourtDesk", 2026, "SC", 43, now)
	if err != ErrAlreadyPublished {
		t.Errorf("expected ErrAlreadyPublished, got %v", err)
	}
}

func TestVerifyPDFMagicBytes(t *testing.T) {
	validPDF := []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	if !VerifyPDFMagicBytes(validPDF) {
		t.Errorf("expected valid PDF to pass magic bytes check")
	}

	invalidExe := []byte("MZ\x90\x00\x03\x00\x00\x00")
	if VerifyPDFMagicBytes(invalidExe) {
		t.Errorf("expected EXE to fail magic bytes check")
	}

	invalidText := []byte("This is a plain text file pretending to be PDF")
	if VerifyPDFMagicBytes(invalidText) {
		t.Errorf("expected plain text to fail magic bytes check")
	}

	shortBuf := []byte("%PD")
	if VerifyPDFMagicBytes(shortBuf) {
		t.Errorf("expected short buffer to fail magic bytes check")
	}
}

func TestValidatePDFSize(t *testing.T) {
	maxBytes := int64(100 * 1024 * 1024) // 100 MB

	if err := ValidatePDFSize(5*1024*1024, maxBytes); err != nil {
		t.Errorf("expected 5MB to be valid, got: %v", err)
	}

	if err := ValidatePDFSize(120*1024*1024, maxBytes); err == nil {
		t.Errorf("expected 120MB to exceed 100MB limit")
	}

	if err := ValidatePDFSize(0, maxBytes); err == nil {
		t.Errorf("expected 0 bytes to fail size check")
	}
}

func TestAttachPDFAndExtractionStatus(t *testing.T) {
	now := time.Now().UTC()
	j, _ := NewJudgment("j1", "Case", "CA 1", "Court", "SC", nil, now, nil,
		"A", "R", nil, nil, nil, "", "", "", "Summary", nil, nil, nil, AccessLevelFree, "user1", now)

	thumb := "thumbnails/j1.jpg"
	prev := "previews/j1.json"
	err := j.AttachPDF("judgments/2026/j1.pdf", "judgment.pdf", 1048576, "sha256-abcdef", 24, &thumb, &prev, now)
	if err != nil {
		t.Fatalf("unexpected error attaching PDF: %v", err)
	}

	if j.PDFPageCount == nil || *j.PDFPageCount != 24 {
		t.Errorf("expected page count 24, got %v", j.PDFPageCount)
	}
	if j.ExtractionStatus != ExtractionPending {
		t.Errorf("expected extraction status pending, got %s", j.ExtractionStatus)
	}

	j.SetExtractionStatus(ExtractionProcessing, now)
	if j.ExtractionStatus != ExtractionProcessing {
		t.Errorf("expected processing, got %s", j.ExtractionStatus)
	}

	j.SetExtractionStatus(ExtractionCompleted, now)
	if j.ExtractionStatus != ExtractionCompleted {
		t.Errorf("expected completed, got %s", j.ExtractionStatus)
	}
}
