
package content

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestArticleWorkflow(t *testing.T) {
	now := time.Now().UTC()
	authorID := "author-123"
	editorID := "editor-456"

	article := &Article{
		ID:        "art-001",
		Title:     "Supreme Court Ruling on Article 21",
		Summary:   "Important constitutional bench ruling expanding personal liberty.",
		Category:  "Constitutional Law",
		Status:    StatusDraft,
		AuthorID:  authorID,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
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

	// 9. Editor schedules article for 2 hours later -> OK
	scheduledTime := now.Add(2 * time.Hour)
	if err := article.Schedule("editor", scheduledTime, now); err != nil {
		t.Fatalf("expected scheduling to succeed, got: %v", err)
	}
	if article.Status != StatusScheduled {
		t.Errorf("expected status to be scheduled, got: %s", article.Status)
	}
	if article.ScheduledAt == nil || !article.ScheduledAt.Equal(scheduledTime) {
		t.Errorf("expected scheduled_at to match scheduledTime")
	}

	// 10. Editor approves and publishes -> OK
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

	// 11. Soft delete and restore
	article.SoftDelete(now.Add(3 * time.Hour))
	if article.DeletedAt == nil {
		t.Errorf("expected deleted_at to be set after soft delete")
	}
	article.Restore(now.Add(4 * time.Hour))
	if article.DeletedAt != nil {
		t.Errorf("expected deleted_at to be nil after restore")
	}

	// 12. Editor archives article -> OK
	if err := article.Archive("editor", now.Add(5*time.Hour)); err != nil {
		t.Fatalf("expected archiving to succeed, got: %v", err)
	}
	if article.Status != StatusArchived {
		t.Errorf("expected status to be archived, got: %s", article.Status)
	}

	// 13. Archiving again fails
	if err := article.Archive("admin", now.Add(6*time.Hour)); err == nil {
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

func TestArticleTypes(t *testing.T) {
	validTypes := []ArticleType{
		ArticleTypeNews, ArticleTypeCommentary, ArticleTypeEditorial, ArticleTypeJudgmentSummary,
		ArticleTypeTopStory, ArticleTypeColumn, ArticleTypeOpinion, ArticleTypeInterview,
		ArticleTypeRoundUp, ArticleTypeDigest, ArticleTypeKnowTheLaw, ArticleTypeBookReview,
		ArticleTypeJobUpdate, ArticleTypeEvent, ArticleTypeVideo, ArticleTypeCartoon,
	}

	for _, vt := range validTypes {
		if !IsValidArticleType(vt) {
			t.Errorf("expected %s to be valid article type", vt)
		}
	}

	if IsValidArticleType("invalid_unknown_type") {
		t.Errorf("expected invalid_unknown_type to be rejected")
	}
}

func TestExtractOutlineAndTOC(t *testing.T) {
	sampleBlocks := []map[string]any{
		{
			"type": "heading",
			"attrs": map[string]any{
				"level": 2,
			},
			"content": []any{
				map[string]any{"type": "text", "text": "Background of the Dispute"},
			},
		},
		{
			"type": "paragraph",
			"content": []any{
				map[string]any{"type": "text", "text": "The appellants approached the court under Article 136."},
			},
		},
		{
			"type": "heading",
			"attrs": map[string]any{
				"level": 3,
			},
			"content": []any{
				map[string]any{"type": "text", "text": "Statutory Interpretation of Section 9"},
			},
		},
		{
			"type": "heading",
			"attrs": map[string]any{
				"level": 2,
			},
			"content": []any{
				map[string]any{"type": "text", "text": "Conclusion & Operative Order"},
			},
		},
	}

	blocksBytes, err := json.Marshal(sampleBlocks)
	if err != nil {
		t.Fatalf("failed marshaling blocks: %v", err)
	}

	outline := ExtractOutline(blocksBytes)
	if len(outline) != 3 {
		t.Fatalf("expected 3 outline items, got %d", len(outline))
	}

	if outline[0].Text != "Background of the Dispute" || outline[0].Level != 2 || outline[0].ID != "background-of-the-dispute" {
		t.Errorf("unexpected outline item 0: %+v", outline[0])
	}
	if outline[1].Text != "Statutory Interpretation of Section 9" || outline[1].Level != 3 {
		t.Errorf("unexpected outline item 1: %+v", outline[1])
	}
	if outline[2].Text != "Conclusion & Operative Order" || outline[2].Level != 2 || outline[2].ID != "conclusion-operative-order" {
		t.Errorf("unexpected outline item 2: %+v", outline[2])
	}
}

func TestLongArticle_25000Words(t *testing.T) {
	// Construct a massive 25,000 word article body with headings, quotes, and paragraphs
	numParagraphs := 250 // 250 * 100 words = 25,000 words
	blocks := make([]map[string]any, 0, numParagraphs+10)

	paragraphText := strings.Repeat("legal jurisprudence constitutional precedent statutory interpretation bench order hearing verdict ", 10) // 100 words

	for i := 1; i <= numParagraphs; i++ {
		if i%25 == 1 {
			blocks = append(blocks, map[string]any{
				"type": "heading",
				"attrs": map[string]any{
					"level": 2,
				},
				"content": []any{
					map[string]any{"type": "text", "text": fmt.Sprintf("Section %d: Comprehensive Analysis of Precedents", i/25+1)},
				},
			})
		}
		blocks = append(blocks, map[string]any{
			"type": "paragraph",
			"content": []any{
				map[string]any{"type": "text", "text": paragraphText},
			},
		})
	}

	blocksBytes, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("failed marshaling 25,000 word body: %v", err)
	}

	summary := "A monumental 25,000-word treatise reviewing fifty years of constitutional jurisprudence."

	start := time.Now()
	wordCount, readingTime := CalculateWordCountAndReadingTime(summary, blocksBytes)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("word count calculation too slow: took %v", elapsed)
	}

	if wordCount < 25000 {
		t.Errorf("expected at least 25,000 words, got: %d", wordCount)
	}

	expectedMinReadingTime := 25000 / 200 // 125 minutes
	if readingTime < expectedMinReadingTime {
		t.Errorf("expected reading time >= %d min, got: %d", expectedMinReadingTime, readingTime)
	}

	outline := ExtractOutline(blocksBytes)
	if len(outline) != 10 {
		t.Errorf("expected 10 outline items for 250 paragraphs, got: %d", len(outline))
	}
}
