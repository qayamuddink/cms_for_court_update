package search

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/search"
)

// MemorySearchIndex provides an in-memory, thread-safe SearchIndex implementation.
type MemorySearchIndex struct {
	mu         sync.RWMutex
	articles   map[string]ports.ArticleSearchDoc
	judgments  map[string]ports.JudgmentSearchDoc
	cases      map[string]ports.CaseSearchDoc
	lastSyncAt time.Time
}

// NewMemorySearchIndex creates a new MemorySearchIndex.
func NewMemorySearchIndex() *MemorySearchIndex {
	return &MemorySearchIndex{
		articles:   make(map[string]ports.ArticleSearchDoc),
		judgments:  make(map[string]ports.JudgmentSearchDoc),
		cases:      make(map[string]ports.CaseSearchDoc),
		lastSyncAt: time.Now().UTC(),
	}
}

func (m *MemorySearchIndex) IndexArticle(ctx context.Context, doc ports.ArticleSearchDoc) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.articles[doc.ID] = doc
	m.lastSyncAt = time.Now().UTC()
	return nil
}

func (m *MemorySearchIndex) DeleteArticle(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.articles, id)
	m.lastSyncAt = time.Now().UTC()
	return nil
}

func (m *MemorySearchIndex) IndexJudgment(ctx context.Context, doc ports.JudgmentSearchDoc) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.judgments[doc.ID] = doc
	m.lastSyncAt = time.Now().UTC()
	return nil
}

func (m *MemorySearchIndex) DeleteJudgment(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.judgments, id)
	m.lastSyncAt = time.Now().UTC()
	return nil
}

func (m *MemorySearchIndex) IndexCourtCase(ctx context.Context, doc ports.CaseSearchDoc) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cases[doc.ID] = doc
	m.lastSyncAt = time.Now().UTC()
	return nil
}

func (m *MemorySearchIndex) DeleteCourtCase(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cases, id)
	m.lastSyncAt = time.Now().UTC()
	return nil
}

func (m *MemorySearchIndex) Reset(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.articles = make(map[string]ports.ArticleSearchDoc)
	m.judgments = make(map[string]ports.JudgmentSearchDoc)
	m.cases = make(map[string]ports.CaseSearchDoc)
	m.lastSyncAt = time.Now().UTC()
	return nil
}

func (m *MemorySearchIndex) GetStats(ctx context.Context) (*ports.SearchIndexStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := int64(len(m.articles) + len(m.judgments) + len(m.cases))
	syncCopy := m.lastSyncAt
	return &ports.SearchIndexStats{
		TotalDocuments: total,
		ArticleCount:   int64(len(m.articles)),
		JudgmentCount:  int64(len(m.judgments)),
		CaseCount:      int64(len(m.cases)),
		Engine:         "in-memory",
		Status:         "healthy",
		LastSyncAt:     &syncCopy,
	}, nil
}

func (m *MemorySearchIndex) Search(ctx context.Context, query search.SearchQuery, isEntitled bool) (*search.SearchResult, error) {
	start := time.Now()
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := query.Validate(); err != nil {
		return nil, err
	}

	result := &search.SearchResult{
		Query: query.Query,
		Hits:  make([]search.SearchResultItem, 0),
	}

	// Gating: If user is not entitled but uses advanced filters, flag upgrade required
	if !isEntitled && query.HasAdvancedFilters() {
		result.IsUpgradeRequired = true
		result.UpgradeMessage = "Advanced legal filters and statutory search require a LiveLaw Premium subscription."
	}

	rawQuery := strings.ToLower(strings.TrimSpace(query.Query))
	exactCitation := ""
	if query.IsExactCitation() {
		exactCitation = strings.ToLower(strings.TrimSpace(query.Query))
	}

	var allHits []search.SearchResultItem

	// 1. Search Judgments
	if query.DocumentType == search.DocTypeAll || query.DocumentType == search.DocTypeJudgment {
		for _, j := range m.judgments {
			if !m.matchJudgmentFilters(j, query) {
				continue
			}

			score, snippet, pageNum, matched := m.scoreJudgment(j, rawQuery, exactCitation, query.SectionCited)
			if !matched && rawQuery != "" && exactCitation == "" && query.SectionCited == "" {
				continue
			}

			isPremium := j.AccessLevel == "premium"
			upgradeReq := false
			if isPremium && !isEntitled {
				upgradeReq = true
				if snippet != "" {
					snippet = "[Premium content: subscribe to view full holding]"
				}
			}

			statuteNames := make([]string, 0, len(j.Statutes))
			for _, st := range j.Statutes {
				statuteNames = append(statuteNames, st.StatuteName)
			}

			var pNumPtr *int
			if pageNum > 0 {
				pNumCopy := pageNum
				pNumPtr = &pNumCopy
			}

			hit := search.SearchResultItem{
				ID:                j.ID,
				DocumentType:      search.DocTypeJudgment,
				Title:             j.CaseTitle,
				Snippet:           snippet,
				Citation:          j.Citation,
				Court:             j.Court,
				Date:              j.JudgmentDate.Format("2006-01-02"),
				StatutesCited:     statuteNames,
				MatchedPageNumber: pNumPtr,
				Score:             score,
				IsPremium:         isPremium,
				IsUpgradeRequired: upgradeReq,
			}

			// Check exact citation match
			if exactCitation != "" && strings.EqualFold(strings.TrimSpace(j.Citation), strings.TrimSpace(query.Query)) {
				hitCopy := hit
				result.ExactCitationMatch = &hitCopy
			}

			allHits = append(allHits, hit)
		}
	}

	// 2. Search Articles
	if query.DocumentType == search.DocTypeAll || query.DocumentType == search.DocTypeArticle {
		for _, a := range m.articles {
			if !m.matchArticleFilters(a, query) {
				continue
			}

			score, snippet, matched := m.scoreArticle(a, rawQuery)
			if !matched && rawQuery != "" {
				continue
			}

			upgradeReq := false
			if a.IsPremium && !isEntitled {
				upgradeReq = true
				if snippet != "" {
					snippet = "[Premium article: subscribe to read full story]"
				}
			}

			hit := search.SearchResultItem{
				ID:                a.ID,
				DocumentType:      search.DocTypeArticle,
				Title:             a.Title,
				Snippet:           snippet,
				Date:              a.PublishedAt.Format("2006-01-02"),
				ArticleType:       a.ArticleType,
				SectionSlug:       a.SectionSlug,
				Score:             score,
				IsPremium:         a.IsPremium,
				IsUpgradeRequired: upgradeReq,
			}
			allHits = append(allHits, hit)
		}
	}

	// 3. Search Court Cases
	if query.DocumentType == search.DocTypeAll || query.DocumentType == search.DocTypeCourtCase {
		for _, c := range m.cases {
			if !m.matchCaseFilters(c, query) {
				continue
			}

			score, snippet, matched := m.scoreCase(c, rawQuery)
			if !matched && rawQuery != "" {
				continue
			}

			hit := search.SearchResultItem{
				ID:           c.ID,
				DocumentType: search.DocTypeCourtCase,
				Title:        c.Title,
				Snippet:      snippet,
				Court:        c.CourtName,
				Date:         c.UpdatedAt.Format("2006-01-02"),
				Score:        score,
			}
			allHits = append(allHits, hit)
		}
	}

	// Sort hits
	if query.SortBy == "date_desc" {
		sort.Slice(allHits, func(i, j int) bool {
			return allHits[i].Date > allHits[j].Date
		})
	} else if query.SortBy == "date_asc" {
		sort.Slice(allHits, func(i, j int) bool {
			return allHits[i].Date < allHits[j].Date
		})
	} else {
		// Default: relevance score descending
		sort.Slice(allHits, func(i, j int) bool {
			if allHits[i].Score == allHits[j].Score {
				return allHits[i].Date > allHits[j].Date
			}
			return allHits[i].Score > allHits[j].Score
		})
	}

	result.TotalHits = len(allHits)

	// Apply pagination bounds
	offset := query.Offset
	if offset > len(allHits) {
		offset = len(allHits)
	}
	end := offset + query.Limit
	if end > len(allHits) {
		end = len(allHits)
	}
	result.Hits = allHits[offset:end]
	result.ProcessingTimeMs = time.Since(start).Milliseconds()

	return result, nil
}

func (m *MemorySearchIndex) matchJudgmentFilters(j ports.JudgmentSearchDoc, q search.SearchQuery) bool {
	if q.Court != "" {
		courtLow := strings.ToLower(q.Court)
		if !strings.Contains(strings.ToLower(j.Court), courtLow) && !strings.Contains(strings.ToLower(j.CourtCode), courtLow) {
			return false
		}
	}
	if q.Year > 0 {
		if j.CitationYear != q.Year && j.JudgmentDate.Year() != q.Year {
			return false
		}
	}
	if q.Judge != "" {
		judgeLow := strings.ToLower(q.Judge)
		matchedJudge := false
		for _, b := range j.Bench {
			if strings.Contains(strings.ToLower(b), judgeLow) {
				matchedJudge = true
				break
			}
		}
		if !matchedJudge {
			return false
		}
	}
	if q.Statute != "" {
		statuteLow := strings.ToLower(q.Statute)
		matchedStatute := false
		for _, st := range j.Statutes {
			if strings.Contains(strings.ToLower(st.StatuteName), statuteLow) {
				matchedStatute = true
				break
			}
		}
		if !matchedStatute {
			return false
		}
	}
	if q.SectionCited != "" {
		secLow := strings.ToLower(q.SectionCited)
		matchedSec := false
		for _, st := range j.Statutes {
			for _, s := range st.SectionsCited {
				if strings.Contains(strings.ToLower(s), secLow) || strings.EqualFold(s, q.SectionCited) {
					matchedSec = true
					break
				}
			}
			if matchedSec {
				break
			}
		}
		if !matchedSec {
			// Also check if section number appears inside pages
			for _, p := range j.Pages {
				if strings.Contains(strings.ToLower(p.Text), secLow) {
					matchedSec = true
					break
				}
			}
		}
		if !matchedSec {
			return false
		}
	}
	if q.DateFrom != nil && j.JudgmentDate.Before(*q.DateFrom) {
		return false
	}
	if q.DateTo != nil && j.JudgmentDate.After(*q.DateTo) {
		return false
	}
	return true
}

func (m *MemorySearchIndex) scoreJudgment(j ports.JudgmentSearchDoc, rawQuery, exactCitation, sectionCited string) (float64, string, int, bool) {
	score := 0.0
	matched := false
	var bestSnippet string
	matchedPage := 0

	// Exact citation match
	if exactCitation != "" && strings.EqualFold(j.Citation, exactCitation) {
		return 100.0, fmt.Sprintf("Judgment Citation: %s (%s)", j.Citation, j.CaseTitle), 0, true
	}

	// Case title
	if rawQuery != "" && strings.Contains(strings.ToLower(j.CaseTitle), rawQuery) {
		score += 50.0
		matched = true
		bestSnippet = search.HighlightSnippet(j.CaseTitle, rawQuery, 140)
	}

	// Headnote / Held / Summary
	if rawQuery != "" {
		if strings.Contains(strings.ToLower(j.Headnote), rawQuery) {
			score += 30.0
			matched = true
			if bestSnippet == "" {
				bestSnippet = search.HighlightSnippet(j.Headnote, rawQuery, 180)
			}
		}
		if strings.Contains(strings.ToLower(j.Held), rawQuery) {
			score += 25.0
			matched = true
			if bestSnippet == "" {
				bestSnippet = search.HighlightSnippet(j.Held, rawQuery, 180)
			}
		}
		if strings.Contains(strings.ToLower(j.Summary), rawQuery) {
			score += 20.0
			matched = true
			if bestSnippet == "" {
				bestSnippet = search.HighlightSnippet(j.Summary, rawQuery, 180)
			}
		}
	}

	// Statutes cited
	for _, st := range j.Statutes {
		if rawQuery != "" && strings.Contains(strings.ToLower(st.StatuteName), rawQuery) {
			score += 20.0
			matched = true
			if bestSnippet == "" {
				bestSnippet = search.HighlightSnippet("Statute: "+st.StatuteName, rawQuery, 140)
			}
		}
		for _, sec := range st.SectionsCited {
			if (rawQuery != "" && strings.Contains(strings.ToLower(sec), rawQuery)) ||
				(sectionCited != "" && (strings.EqualFold(sec, sectionCited) || strings.Contains(strings.ToLower(sec), strings.ToLower(sectionCited)))) {
				score += 35.0
				matched = true
				if bestSnippet == "" {
					highlightTerm := rawQuery
					if highlightTerm == "" {
						highlightTerm = sectionCited
					}
					bestSnippet = search.HighlightSnippet(fmt.Sprintf("%s, %s", st.StatuteName, sec), highlightTerm, 140)
				}
			}
		}
	}

	// PDF text pages (deep extraction search)
	termToSearchInPDF := rawQuery
	if termToSearchInPDF == "" {
		termToSearchInPDF = sectionCited
	}
	if termToSearchInPDF != "" {
		for _, page := range j.Pages {
			if strings.Contains(strings.ToLower(page.Text), strings.ToLower(termToSearchInPDF)) {
				score += 15.0
				matched = true
				if bestSnippet == "" || matchedPage == 0 {
					matchedPage = page.PageNumber
					snip := search.HighlightSnippet(page.Text, termToSearchInPDF, 160)
					bestSnippet = fmt.Sprintf("%s (Page %d)", snip, page.PageNumber)
				}
			}
		}
	}

	if bestSnippet == "" {
		if j.Summary != "" {
			bestSnippet = search.HighlightSnippet(j.Summary, rawQuery, 140)
		} else {
			bestSnippet = j.CaseTitle
		}
	}

	return score, bestSnippet, matchedPage, matched
}

func (m *MemorySearchIndex) matchArticleFilters(a ports.ArticleSearchDoc, q search.SearchQuery) bool {
	if q.Section != "" {
		secLow := strings.ToLower(q.Section)
		if !strings.EqualFold(a.SectionSlug, secLow) && !strings.Contains(strings.ToLower(a.SectionName), secLow) {
			return false
		}
	}
	if q.ArticleType != "" && !strings.EqualFold(a.ArticleType, q.ArticleType) {
		return false
	}
	if q.DateFrom != nil && a.PublishedAt.Before(*q.DateFrom) {
		return false
	}
	if q.DateTo != nil && a.PublishedAt.After(*q.DateTo) {
		return false
	}
	return true
}

func (m *MemorySearchIndex) scoreArticle(a ports.ArticleSearchDoc, rawQuery string) (float64, string, bool) {
	score := 0.0
	matched := false
	var bestSnippet string

	if rawQuery == "" {
		return 1.0, a.Summary, true
	}

	// Title match (high weight)
	if strings.Contains(strings.ToLower(a.Title), rawQuery) {
		score += 50.0
		matched = true
		bestSnippet = search.HighlightSnippet(a.Title, rawQuery, 140)
	}

	// Summary match
	if strings.Contains(strings.ToLower(a.Summary), rawQuery) {
		score += 30.0
		matched = true
		if bestSnippet == "" {
			bestSnippet = search.HighlightSnippet(a.Summary, rawQuery, 160)
		}
	}

	// Body text match
	if strings.Contains(strings.ToLower(a.BodyText), rawQuery) {
		score += 20.0
		matched = true
		if bestSnippet == "" {
			bestSnippet = search.HighlightSnippet(a.BodyText, rawQuery, 160)
		}
	}

	if bestSnippet == "" {
		bestSnippet = search.HighlightSnippet(a.Summary, rawQuery, 140)
	}

	return score, bestSnippet, matched
}

func (m *MemorySearchIndex) matchCaseFilters(c ports.CaseSearchDoc, q search.SearchQuery) bool {
	if q.Court != "" && !strings.Contains(strings.ToLower(c.CourtName), strings.ToLower(q.Court)) {
		return false
	}
	if q.Judge != "" {
		judgeLow := strings.ToLower(q.Judge)
		matched := strings.Contains(strings.ToLower(c.PresidingJudge), judgeLow)
		if !matched {
			for _, b := range c.BenchJudges {
				if strings.Contains(strings.ToLower(b), judgeLow) {
					matched = true
					break
				}
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func (m *MemorySearchIndex) scoreCase(c ports.CaseSearchDoc, rawQuery string) (float64, string, bool) {
	score := 0.0
	matched := false
	var bestSnippet string

	if rawQuery == "" {
		return 1.0, fmt.Sprintf("%s - %s", c.CaseNumber, c.CourtName), true
	}

	if strings.Contains(strings.ToLower(c.Title), rawQuery) {
		score += 40.0
		matched = true
		bestSnippet = search.HighlightSnippet(c.Title, rawQuery, 140)
	}
	if strings.Contains(strings.ToLower(c.CaseNumber), rawQuery) {
		score += 50.0
		matched = true
		if bestSnippet == "" {
			bestSnippet = fmt.Sprintf("Case %s in %s", c.CaseNumber, c.CourtName)
		}
	}
	for _, p := range c.PetitionerParties {
		if strings.Contains(strings.ToLower(p), rawQuery) {
			score += 20.0
			matched = true
			if bestSnippet == "" {
				bestSnippet = search.HighlightSnippet(p, rawQuery, 120)
			}
		}
	}

	if bestSnippet == "" {
		bestSnippet = fmt.Sprintf("%s (%s)", c.Title, c.CourtName)
	}

	return score, bestSnippet, matched
}

func (m *MemorySearchIndex) Autosuggest(ctx context.Context, prefix string, limit int) (*search.AutosuggestResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}
	pLow := strings.ToLower(strings.TrimSpace(prefix))

	caseSet := make(map[string]struct{})
	statuteSet := make(map[string]struct{})
	citationSet := make(map[string]struct{})

	// Check judgments
	for _, j := range m.judgments {
		if pLow == "" || strings.Contains(strings.ToLower(j.CaseTitle), pLow) {
			caseSet[j.CaseTitle] = struct{}{}
		}
		if j.Citation != "" && (pLow == "" || strings.Contains(strings.ToLower(j.Citation), pLow)) {
			citationSet[j.Citation] = struct{}{}
		}
		for _, st := range j.Statutes {
			if pLow == "" || strings.Contains(strings.ToLower(st.StatuteName), pLow) {
				statuteSet[st.StatuteName] = struct{}{}
			}
			for _, sec := range st.SectionsCited {
				full := fmt.Sprintf("%s (%s)", st.StatuteName, sec)
				if pLow == "" || strings.Contains(strings.ToLower(full), pLow) {
					statuteSet[full] = struct{}{}
				}
			}
		}
	}

	// Check court cases
	for _, c := range m.cases {
		if pLow == "" || strings.Contains(strings.ToLower(c.Title), pLow) {
			caseSet[c.Title] = struct{}{}
		}
	}

	res := &search.AutosuggestResult{
		CaseNames: make([]string, 0, limit),
		Statutes:  make([]string, 0, limit),
		Citations: make([]string, 0, limit),
	}

	for k := range caseSet {
		if len(res.CaseNames) < limit {
			res.CaseNames = append(res.CaseNames, k)
		}
	}
	sort.Strings(res.CaseNames)

	for k := range statuteSet {
		if len(res.Statutes) < limit {
			res.Statutes = append(res.Statutes, k)
		}
	}
	sort.Strings(res.Statutes)

	for k := range citationSet {
		if len(res.Citations) < limit {
			res.Citations = append(res.Citations, k)
		}
	}
	sort.Strings(res.Citations)

	return res, nil
}
