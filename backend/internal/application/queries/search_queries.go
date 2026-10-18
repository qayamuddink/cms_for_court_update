package queries

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/search"
)

// SearchQueries handles all search query and analytics read operations.
type SearchQueries struct {
	searchIndex   ports.SearchIndex
	analyticsRepo ports.SearchAnalyticsRepository
}

// NewSearchQueries creates a new SearchQueries instance.
func NewSearchQueries(searchIndex ports.SearchIndex, analyticsRepo ports.SearchAnalyticsRepository) *SearchQueries {
	return &SearchQueries{
		searchIndex:   searchIndex,
		analyticsRepo: analyticsRepo,
	}
}

// Search executes a filterable search query, enforcing entitlement gating and logging analytics.
func (q *SearchQueries) Search(ctx context.Context, query search.SearchQuery, clientIP string, isEntitled bool) (*search.SearchResult, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	result, err := q.searchIndex.Search(ctx, query, isEntitled)
	if err != nil {
		return nil, err
	}

	// Record search analytics asynchronously
	if q.analyticsRepo != nil && strings.TrimSpace(query.Query) != "" {
		ipHash := hashIP(clientIP)
		hasFilters := query.HasAdvancedFilters() || query.Section != "" || query.Court != "" || query.ArticleType != ""
		_ = q.analyticsRepo.RecordQuery(ctx, query.Query, result.TotalHits, ipHash, hasFilters)
	}

	return result, nil
}

// Autosuggest returns prefix-matched suggestions for case names, statutes, and citations.
func (q *SearchQueries) Autosuggest(ctx context.Context, prefix string, limit int) (*search.AutosuggestResult, error) {
	return q.searchIndex.Autosuggest(ctx, prefix, limit)
}

// GetHealth returns index size and document statistics for the CMS admin dashboard.
func (q *SearchQueries) GetHealth(ctx context.Context) (*ports.SearchIndexStats, error) {
	return q.searchIndex.GetStats(ctx)
}

// GetAnalytics returns top search queries, zero-result queries, and total volume.
func (q *SearchQueries) GetAnalytics(ctx context.Context, limit int) ([]ports.QueryCount, []ports.ZeroResultQuery, int64, error) {
	if q.analyticsRepo == nil {
		return nil, nil, 0, nil
	}

	top, err := q.analyticsRepo.GetTopQueries(ctx, limit)
	if err != nil {
		return nil, nil, 0, err
	}

	zeros, err := q.analyticsRepo.GetZeroResultQueries(ctx, limit)
	if err != nil {
		return nil, nil, 0, err
	}

	total, err := q.analyticsRepo.GetTotalSearches(ctx)
	if err != nil {
		return nil, nil, 0, err
	}

	return top, zeros, total, nil
}

func hashIP(ip string) string {
	if ip == "" {
		return ""
	}
	h := sha256.Sum256([]byte(ip))
	return fmt.Sprintf("%x", h[:16])
}
