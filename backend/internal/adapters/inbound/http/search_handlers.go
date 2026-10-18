package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/search"
)

// SearchHandler provides public search, autosuggest, health, re-index, and analytics endpoints.
type SearchHandler struct {
	searchQueries *queries.SearchQueries
	searchIndexer *commands.SearchIndexer
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(
	searchQueries *queries.SearchQueries,
	searchIndexer *commands.SearchIndexer,
) *SearchHandler {
	return &SearchHandler{
		searchQueries: searchQueries,
		searchIndexer: searchIndexer,
	}
}

// Search handles GET /v1/search.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	qParams := r.URL.Query()

	q := qParams.Get("q")
	docType := search.DocumentType(qParams.Get("type"))
	if docType == "" {
		docType = search.DocTypeAll
	}

	year, _ := strconv.Atoi(qParams.Get("year"))
	limit, _ := strconv.Atoi(qParams.Get("limit"))
	offset, _ := strconv.Atoi(qParams.Get("offset"))

	var dateFrom, dateTo *time.Time
	if df := qParams.Get("date_from"); df != "" {
		if t, err := time.Parse("2006-01-02", df); err == nil {
			dateFrom = &t
		}
	}
	if dt := qParams.Get("date_to"); dt != "" {
		if t, err := time.Parse("2006-01-02", dt); err == nil {
			dateTo = &t
		}
	}

	searchQuery := search.SearchQuery{
		Query:              q,
		DocumentType:       docType,
		Section:            qParams.Get("section"),
		Court:              qParams.Get("court"),
		Year:               year,
		Judge:              qParams.Get("judge"),
		Statute:            qParams.Get("statute"),
		SectionCited:       qParams.Get("section_cited"),
		ArticleType:        qParams.Get("article_type"),
		DateFrom:           dateFrom,
		DateTo:             dateTo,
		SortBy:             qParams.Get("sort_by"),
		Limit:              limit,
		Offset:             offset,
		IncludeFullPDFText: true,
	}

	// Entitlement check: Subscribers and Staff have full entitlement
	claims := GetAuthClaims(r.Context())
	isEntitled := false
	if claims != nil {
		if claims.Entitlement.Tier == "premium" || claims.Entitlement.Tier == "all_access" ||
			claims.SystemRole == "admin" || claims.SystemRole == "editor" || claims.SystemRole == "reporter" {
			isEntitled = true
		}
	}

	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	result, err := h.searchQueries.Search(r.Context(), searchQuery, clientIP, isEntitled)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_SEARCH_QUERY", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Autosuggest handles GET /v1/search/autosuggest.
func (h *SearchHandler) Autosuggest(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = r.URL.Query().Get("q")
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}

	res, err := h.searchQueries.Autosuggest(r.Context(), prefix, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "AUTOSUGGEST_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// GetHealth handles GET /v1/cms/search/health.
func (h *SearchHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	stats, err := h.searchQueries.GetHealth(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "SEARCH_HEALTH_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// Reindex handles POST /v1/cms/search/reindex.
func (h *SearchHandler) Reindex(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	if h.searchIndexer == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "REINDEX_UNAVAILABLE", "Search indexer is not configured", reqID)
		return
	}

	stats, err := h.searchIndexer.ReindexAll(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "REINDEX_FAILED", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "completed",
		"message": "Full search re-index completed successfully",
		"stats":   stats,
	})
}

// GetAnalytics handles GET /v1/cms/search/analytics.
func (h *SearchHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}

	top, zeros, total, err := h.searchQueries.GetAnalytics(r.Context(), limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "ANALYTICS_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_searches":      total,
		"top_queries":         top,
		"zero_result_queries": zeros,
	})
}

