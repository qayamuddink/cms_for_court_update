package search_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adapters "github.com/livelaw/backend/internal/adapters/outbound/search"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/search"
)

func TestMemorySearchIndex_IndexingAndQuerying(t *testing.T) {
	idx := adapters.NewMemorySearchIndex()
	ctx := context.Background()

	// 1. Index Article
	now := time.Now().UTC()
	err := idx.IndexArticle(ctx, ports.ArticleSearchDoc{
		ID:          "art-1",
		Title:       "Supreme Court Lays Down Bail Principles",
		Summary:     "Important bail guidelines for trial courts",
		BodyText:    "Jail is the exception and bail is the rule according to the Constitution.",
		ArticleType: "news",
		SectionName: "Supreme Court",
		IsPremium:   false,
		PublishedAt: now,
	})
	if err != nil {
		t.Fatalf("failed to index article: %v", err)
	}

	// 2. Index Judgment with PDF pages and statutes
	cit := "2026 LiveLaw (SC) 42"
	year := 2026
	err = idx.IndexJudgment(ctx, ports.JudgmentSearchDoc{
		ID:                "jdg-1",
		CaseTitle:         "M/s ABC Enterprises v. State of Maharashtra",
		CaseNumber:        "Crl.A. 100/2026",
		Citation:          cit,
		CitationYear:      year,
		CitationCourtCode: "SC",
		CitationNumber:    42,
		Court:             "Supreme Court of India",
		CourtCode:         "SC",
		Bench:             []string{"D.Y. Chandrachud, CJI"},
		Appellant:         "M/s ABC Enterprises",
		Respondent:        "State of Maharashtra",
		Statutes: []ports.JudgmentStatuteSearchDoc{
			{
				StatuteName:   "Negotiable Instruments Act",
				SectionsCited: []string{"Section 138"},
			},
		},
		Held:         "Statutory notice mandatory.",
		AccessLevel:  "free",
		JudgmentDate: now,
		PublishedAt:  now,
		Pages: []ports.JudgmentPageSearchDoc{
			{
				PageNumber: 7,
				Text:       "Section 138 requires strict adherence to notice delivery within 15 days.",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to index judgment: %v", err)
	}

	// 3. Search query matching judgment and page number
	res, err := idx.Search(ctx, search.SearchQuery{
		Query: "Section 138",
		Limit: 10,
	}, true)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if res.TotalHits != 1 {
		t.Fatalf("expected 1 hit, got %d", res.TotalHits)
	}
	hit := res.Hits[0]
	if hit.ID != "jdg-1" {
		t.Errorf("expected hit jdg-1, got %s", hit.ID)
	}
	if hit.MatchedPageNumber == nil || *hit.MatchedPageNumber != 7 {
		t.Errorf("expected matched page 7, got %v", hit.MatchedPageNumber)
	}
	if !strings.Contains(hit.Snippet, "<mark>Section 138</mark>") {
		t.Errorf("expected snippet with mark tag, got %s", hit.Snippet)
	}
	if !strings.Contains(hit.Snippet, "(Page 7)") {
		t.Errorf("expected page attribution in snippet, got %s", hit.Snippet)
	}

	// 4. Exact citation search
	citQuery := search.SearchQuery{
		Query: "2026 LiveLaw (SC) 42",
		Limit: 10,
	}
	citRes, err := idx.Search(ctx, citQuery, true)
	if err != nil {
		t.Fatalf("citation search failed: %v", err)
	}
	if citRes.ExactCitationMatch == nil || citRes.ExactCitationMatch.Citation != cit {
		t.Fatalf("expected exact citation match %s", cit)
	}

	// 5. Autosuggest
	suggs, err := idx.Autosuggest(ctx, "ABC", 5)
	if err != nil {
		t.Fatalf("autosuggest failed: %v", err)
	}
	if len(suggs.CaseNames) == 0 {
		t.Fatalf("expected autosuggest results for 'ABC'")
	}

	// 6. Stats & Clear
	stats, err := idx.GetStats(ctx)
	if err != nil {
		t.Fatalf("get stats failed: %v", err)
	}
	if stats.JudgmentCount != 1 || stats.ArticleCount != 1 {
		t.Errorf("unexpected stats: %+v", stats)
	}

	if err := idx.Reset(ctx); err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	statsAfter, _ := idx.GetStats(ctx)
	if statsAfter.TotalDocuments != 0 {
		t.Errorf("expected 0 documents after reset, got %d", statsAfter.TotalDocuments)
	}
}

func TestMeilisearchAdapter_FallbackAndHealth(t *testing.T) {
	// Mock Meilisearch server returning 200 OK for health check
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"available"}`))
			return
		}
		if r.URL.Path == "/stats" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"databaseSize": 1024}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	adapter := adapters.NewMeilisearchAdapter(adapters.Config{
		BaseURL:   srv.URL,
		MasterKey: "test-key",
		Timeout:   time.Second,
	}, nil)

	ctx := context.Background()
	// Test indexing to adapter
	err := adapter.IndexArticle(ctx, ports.ArticleSearchDoc{
		ID:          "art-fallback-1",
		Title:       "Test Meili Title",
		Summary:     "Test Summary",
		ArticleType: "news",
		PublishedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("indexing failed: %v", err)
	}

	stats, err := adapter.GetStats(ctx)
	if err != nil {
		t.Fatalf("get stats failed: %v", err)
	}
	if stats.ArticleCount != 1 {
		t.Errorf("expected 1 article in fallback stats, got %d", stats.ArticleCount)
	}
}

