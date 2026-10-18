package search

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Citation regex pattern: e.g. "2026 LiveLaw (SC) 1" or "2024 CourtDesk (Del) 42"
var citationPattern = regexp.MustCompile(`^(?i)(\d{4})\s+([A-Za-z0-9_-]+)\s+\(([A-Za-z0-9_-]+)\)\s+(\d+)$`)

// DocumentType represents the type of searchable document.
type DocumentType string

const (
	DocTypeArticle   DocumentType = "article"
	DocTypeJudgment  DocumentType = "judgment"
	DocTypeCourtCase DocumentType = "court_case"
	DocTypeAll       DocumentType = "all"
)

// SearchQuery models an incoming search request.
type SearchQuery struct {
	Query              string
	DocumentType       DocumentType
	Section            string
	Court              string
	Year               int
	Judge              string
	Statute            string
	SectionCited       string
	ArticleType        string
	DateFrom           *time.Time
	DateTo             *time.Time
	SortBy             string // "relevance", "date_desc", "date_asc"
	Limit              int
	Offset             int
	IncludeFullPDFText bool
}

// Validate sanitizes and enforces boundary constraints on a search query.
func (q *SearchQuery) Validate() error {
	q.Query = strings.TrimSpace(q.Query)
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.SortBy == "" {
		q.SortBy = "relevance"
	}
	if q.DocumentType == "" {
		q.DocumentType = DocTypeAll
	}
	if q.DateFrom != nil && q.DateTo != nil && q.DateFrom.After(*q.DateTo) {
		return fmt.Errorf("date_from cannot be after date_to")
	}
	return nil
}

// HasAdvancedFilters returns true if the search query uses advanced filters that require entitlement.
func (q *SearchQuery) HasAdvancedFilters() bool {
	return q.Statute != "" ||
		q.SectionCited != "" ||
		q.Judge != "" ||
		q.Year > 0 ||
		q.DateFrom != nil ||
		q.DateTo != nil
}

// IsExactCitation returns true if query matches citation format.
func (q *SearchQuery) IsExactCitation() bool {
	return citationPattern.MatchString(strings.TrimSpace(q.Query))
}

// SearchResultItem represents an individual hit in search results.
type SearchResultItem struct {
	ID                string       `json:"id"`
	DocumentType      DocumentType `json:"document_type"`
	Title             string       `json:"title"`
	Snippet           string       `json:"snippet"`
	Citation          string       `json:"citation,omitempty"`
	Court             string       `json:"court,omitempty"`
	Date              string       `json:"date,omitempty"`
	ArticleType       string       `json:"article_type,omitempty"`
	SectionSlug       string       `json:"section_slug,omitempty"`
	StatutesCited     []string     `json:"statutes_cited,omitempty"`
	MatchedPageNumber *int         `json:"matched_page_number,omitempty"`
	Score             float64      `json:"score,omitempty"`
	IsPremium         bool         `json:"is_premium,omitempty"`
	IsUpgradeRequired bool         `json:"is_upgrade_required,omitempty"`
}

// SearchResult represents the full output of a search query execution.
type SearchResult struct {
	Query              string             `json:"query"`
	TotalHits          int                `json:"total_hits"`
	Hits               []SearchResultItem `json:"hits"`
	ExactCitationMatch *SearchResultItem  `json:"exact_citation_match,omitempty"`
	IsUpgradeRequired  bool               `json:"is_upgrade_required"`
	UpgradeMessage     string             `json:"upgrade_message,omitempty"`
	ProcessingTimeMs   int64              `json:"processing_time_ms"`
}

// AutosuggestResult models typeahead suggestions.
type AutosuggestResult struct {
	CaseNames []string `json:"case_names"`
	Statutes  []string `json:"statutes"`
	Citations []string `json:"citations"`
}

// HighlightSnippet generates a text snippet centered around the matched terms with <mark> tags.
func HighlightSnippet(text string, query string, maxLen int) string {
	if text == "" {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 180
	}
	textClean := strings.TrimSpace(text)
	query = strings.TrimSpace(query)
	if query == "" {
		if len(textClean) > maxLen {
			return textClean[:maxLen] + "..."
		}
		return textClean
	}

	terms := []string{query}
	terms = append(terms, strings.Fields(query)...)
	lowestIdx := -1
	matchedTerm := ""
	for _, term := range terms {
		if len(term) < 2 {
			continue
		}
		idx := strings.Index(strings.ToLower(textClean), strings.ToLower(term))
		if idx != -1 {
			if lowestIdx == -1 || idx < lowestIdx || (idx == lowestIdx && len(term) > len(matchedTerm)) {
				lowestIdx = idx
				matchedTerm = term
			}
		}
	}

	if lowestIdx == -1 {
		if len(textClean) > maxLen {
			return textClean[:maxLen] + "..."
		}
		return textClean
	}

	start := lowestIdx - (maxLen / 3)
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(textClean) {
		end = len(textClean)
	}

	snippet := textClean[start:end]
	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(textClean) {
		suffix = "..."
	}

	// Insert <mark> tags case-insensitively around matched term
	re := regexp.MustCompile("(?i)(" + regexp.QuoteMeta(matchedTerm) + ")")
	highlighted := re.ReplaceAllString(snippet, "<mark>$1</mark>")

	return prefix + highlighted + suffix
}
