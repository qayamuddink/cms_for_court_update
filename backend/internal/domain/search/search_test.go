package search_test

import (
	"strings"
	"testing"
	"time"

	"github.com/livelaw/backend/internal/domain/search"
)

func TestSearchQueryValidation(t *testing.T) {
	q := search.SearchQuery{
		Query: "  Arbitration Act  ",
	}
	if err := q.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Query != "Arbitration Act" {
		t.Errorf("expected trimmed query, got %q", q.Query)
	}
	if q.Limit != 20 {
		t.Errorf("expected default limit 20, got %d", q.Limit)
	}
	if q.DocumentType != search.DocTypeAll {
		t.Errorf("expected default DocTypeAll, got %v", q.DocumentType)
	}

	// Test invalid date range
	now := time.Now()
	before := now.Add(-24 * time.Hour)
	invalidQ := search.SearchQuery{
		DateFrom: &now,
		DateTo:   &before,
	}
	if err := invalidQ.Validate(); err == nil {
		t.Errorf("expected error for date_from after date_to, got nil")
	}
}

func TestSearchQuery_HasAdvancedFilters(t *testing.T) {
	basic := search.SearchQuery{Query: "Constitution"}
	if basic.HasAdvancedFilters() {
		t.Errorf("basic query should not have advanced filters")
	}

	withStatute := search.SearchQuery{Query: "Arbitration", Statute: "Arbitration and Conciliation Act"}
	if !withStatute.HasAdvancedFilters() {
		t.Errorf("query with statute should have advanced filters")
	}

	withJudge := search.SearchQuery{Query: "Bail", Judge: "Chandrachud"}
	if !withJudge.HasAdvancedFilters() {
		t.Errorf("query with judge should have advanced filters")
	}

	withYear := search.SearchQuery{Query: "Verdict", Year: 2026}
	if !withYear.HasAdvancedFilters() {
		t.Errorf("query with year should have advanced filters")
	}
}

func TestSearchQuery_IsExactCitation(t *testing.T) {
	validCitations := []string{
		"2026 LiveLaw (SC) 1",
		"2025 CourtDesk (Del) 42",
		"2024 LIVELAW (BOM) 123",
	}
	for _, c := range validCitations {
		q := search.SearchQuery{Query: c}
		if !q.IsExactCitation() {
			t.Errorf("expected %q to be recognized as exact citation", c)
		}
	}

	invalidCitations := []string{
		"Arbitration Act 1996",
		"Section 138 NI Act",
		"2026",
	}
	for _, c := range invalidCitations {
		q := search.SearchQuery{Query: c}
		if q.IsExactCitation() {
			t.Errorf("expected %q NOT to be recognized as exact citation", c)
		}
	}
}

func TestHighlightSnippet(t *testing.T) {
	text := "In this milestone judgment, the Supreme Court held that Section 138 of the Negotiable Instruments Act requires strict compliance with notice provisions before any criminal liability can attach."
	snippet := search.HighlightSnippet(text, "Section 138", 80)
	if !strings.Contains(snippet, "<mark>Section 138</mark>") {
		t.Errorf("expected snippet to contain '<mark>Section 138</mark>', got: %s", snippet)
	}

	emptySnippet := search.HighlightSnippet("", "hello", 50)
	if emptySnippet != "" {
		t.Errorf("expected empty string for empty text, got %q", emptySnippet)
	}
}
