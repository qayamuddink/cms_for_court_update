package content

import (
	"testing"
	"time"
)

func TestArticleWorkflow(t *testing.T) {
	now := time.Now().UTC()
	authorID := "author-123"
	editorID := "editor-456"

	article := &Article{
		ID:          "art-001",
		Title:       "Supreme Court Ruling on Article 21",
		Summary:     "Important constitutional bench ruling expanding personal liberty.",
		Category:    "Constitutional Law",
		Status:      StatusDraft,
		AuthorID:    authorID,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 1. Reporter tries to publish directly -> Forbidden
	if err := article.ApproveAndPublish(authorID, "reporter", now); err == nil {
		t.Errorf("expected reporter publishing to fail")
	}

	// 2. Stranger tries to submit -> Forbidden
	if err := article.SubmitForReview("stranger-999", "reporter"); err == nil {
		t.Errorf("expected non-author submission to fail")
	}

	// 3. Author submits draft for review -> OK
	if err := article.SubmitForReview(authorID, "reporter"); err != nil {
		t.Fatalf("expected author submission to succeed, got: %v", err)
	}
	if article.Status != StatusInReview {
		t.Errorf("expected status to be in_review, got: %s", article.Status)
	}

	// 4. Author tries to submit again while in_review -> Invalid status
	if err := article.SubmitForReview(authorID, "reporter"); err == nil {
		t.Errorf("expected submitting in_review article to fail")
	}

	// 5. Reporter tries to send back -> Forbidden
	if err := article.SendBack("reporter", "needs fixes"); err == nil {
		t.Errorf("expected reporter send-back to fail")
	}

	// 6. Editor sends back without comment -> Fails
	if err := article.SendBack("editor", "   "); err == nil {
		t.Errorf("expected send back without comment to fail")
	}

	// 7. Editor sends back with comment -> OK, transitions to draft
	if err := article.SendBack("editor", "Please clarify petitioner argument on page 2."); err != nil {
		t.Fatalf("expected editor send-back to succeed, got: %v", err)
	}
	if article.Status != StatusDraft {
		t.Errorf("expected status to be draft, got: %s", article.Status)
	}

	// 8. Re-submit by author
	if err := article.SubmitForReview(authorID, "reporter"); err != nil {
		t.Fatalf("failed resubmitting draft: %v", err)
	}

	// 9. Editor approves and publishes -> OK
	publishTime := now.Add(2 * time.Hour)
	if err := article.ApproveAndPublish(editorID, "editor", publishTime); err != nil {
		t.Fatalf("expected editor publish to succeed, got: %v", err)
	}
	if article.Status != StatusPublished {
		t.Errorf("expected status to be published, got: %s", article.Status)
	}
	if article.EditorID == nil || *article.EditorID != editorID {
		t.Errorf("expected editor_id to be set to %s", editorID)
	}
	if article.PublishedAt == nil || !article.PublishedAt.Equal(publishTime) {
		t.Errorf("expected published_at to match publishTime")
	}

	// 10. Editor archives article -> OK
	if err := article.Archive("editor", now.Add(3*time.Hour)); err != nil {
		t.Fatalf("expected archiving to succeed, got: %v", err)
	}
	if article.Status != StatusArchived {
		t.Errorf("expected status to be archived, got: %s", article.Status)
	}

	// 11. Archiving again fails
	if err := article.Archive("admin", now.Add(4*time.Hour)); err == nil {
		t.Errorf("expected re-archiving to fail")
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Supreme Court of India Ruling", "supreme-court-of-india-ruling"},
		{"Article 370: Special Status Abrogation & Constitutional Validity!", "article-370-special-status-abrogation-constitutional-validity"},
		{"   Lots   of    Whitespace   ", "lots-of-whitespace"},
		{"Special #$% Symbols & Punctuation!!!", "special-symbols-punctuation"},
		{"", "untitled"},
	}

	for _, tt := range tests {
		got := GenerateSlug(tt.input)
		if got != tt.expected {
			t.Errorf("GenerateSlug(%q) = %q; expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestAttachPDF(t *testing.T) {
	article := &Article{ID: "art-101"}
	now := time.Now().UTC()

	// Empty key fails
	if err := article.AttachPDF("", 1024, "checksum123", now); err == nil {
		t.Errorf("expected empty storage key to fail")
	}

	// Size <= 0 fails
	if err := article.AttachPDF("judgments/2026/sc-101.pdf", 0, "checksum123", now); err == nil {
		t.Errorf("expected zero size to fail")
	}

	// Valid attachment
	if err := article.AttachPDF("judgments/2026/sc-101.pdf", 5242880, "sha256-abcdef", now); err != nil {
		t.Fatalf("expected valid attachment to succeed, got: %v", err)
	}
	if article.PDFStorageKey == nil || *article.PDFStorageKey != "judgments/2026/sc-101.pdf" {
		t.Errorf("expected storage key to be set")
	}
	if article.PDFSizeBytes == nil || *article.PDFSizeBytes != 5242880 {
		t.Errorf("expected size bytes to be set")
	}
	if article.PDFChecksum == nil || *article.PDFChecksum != "sha256-abcdef" {
		t.Errorf("expected checksum to be set")
	}
}
