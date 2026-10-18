package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/digest"
)

// DigestHandler provides public and CMS HTTP handlers for Digests & Round-ups.
type DigestHandler struct {
	commands *commands.DigestCommands
	queries  *queries.DigestQueries
}

// NewDigestHandler creates a new DigestHandler.
func NewDigestHandler(
	cmds *commands.DigestCommands,
	q *queries.DigestQueries,
) *DigestHandler {
	return &DigestHandler{
		commands: cmds,
		queries:  q,
	}
}

// ListPublicDigests handles GET /v1/digests.
func (h *DigestHandler) ListPublicDigests(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	qParams := r.URL.Query()

	statusPublished := digest.StatusPublished
	f := ports.DigestFilter{
		Status: &statusPublished,
	}

	if court := qParams.Get("court"); court != "" {
		c := strings.ToUpper(court)
		f.CourtCode = &c
	}
	if p := qParams.Get("period"); p != "" {
		period := digest.DigestPeriod(p)
		f.Period = &period
	}
	if dt := qParams.Get("type"); dt != "" {
		t := digest.DigestType(dt)
		f.DigestType = &t
	}
	if yrStr := qParams.Get("year"); yrStr != "" {
		if yr, err := strconv.Atoi(yrStr); err == nil && yr > 0 {
			f.Year = &yr
		}
	}
	if lStr := qParams.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			f.Limit = l
		}
	}
	if oStr := qParams.Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			f.Offset = o
		}
	}

	digests, total, err := h.queries.List(r.Context(), f)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list digests", reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"digests": digests,
		"total":   total,
		"limit":   f.Limit,
		"offset":  f.Offset,
	})
}

// GetPublicDigest handles GET /v1/digests/{slug}.
func (h *DigestHandler) GetPublicDigest(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	slug := r.PathValue("slug")
	if slug == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Slug is required", reqID)
		return
	}

	detail, err := h.queries.GetBySlug(r.Context(), slug)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "Digest not found", reqID)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

// ListCMSDigests handles GET /v1/cms/digests.
func (h *DigestHandler) ListCMSDigests(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	qParams := r.URL.Query()

	f := ports.DigestFilter{}
	if court := qParams.Get("court"); court != "" {
		c := strings.ToUpper(court)
		f.CourtCode = &c
	}
	if s := qParams.Get("status"); s != "" {
		status := digest.DigestStatus(s)
		f.Status = &status
	}
	if p := qParams.Get("period"); p != "" {
		period := digest.DigestPeriod(p)
		f.Period = &period
	}
	if dt := qParams.Get("type"); dt != "" {
		t := digest.DigestType(dt)
		f.DigestType = &t
	}
	if yrStr := qParams.Get("year"); yrStr != "" {
		if yr, err := strconv.Atoi(yrStr); err == nil && yr > 0 {
			f.Year = &yr
		}
	}
	if lStr := qParams.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			f.Limit = l
		}
	}
	if oStr := qParams.Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			f.Offset = o
		}
	}

	digests, total, err := h.queries.List(r.Context(), f)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list CMS digests", reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"digests": digests,
		"total":   total,
		"limit":   f.Limit,
		"offset":  f.Offset,
	})
}

// CreateDigestRequest holds input for POST /v1/cms/digests.
type CreateDigestRequest struct {
	Title              string `json:"title"`
	Court              string `json:"court"`
	CourtCode          string `json:"court_code"`
	DigestType         string `json:"digest_type"`
	Period             string `json:"period"`
	PeriodStart        string `json:"period_start"`
	PeriodEnd          string `json:"period_end"`
	CitationRangeStart *int   `json:"citation_range_start,omitempty"`
	CitationRangeEnd   *int   `json:"citation_range_end,omitempty"`
	CitationPrefix     string `json:"citation_prefix,omitempty"`
	Intro              string `json:"intro,omitempty"`
	Conclusion         string `json:"conclusion,omitempty"`
	AccessLevel        string `json:"access_level,omitempty"`
}

// CreateDigest handles POST /v1/cms/digests.
func (h *DigestHandler) CreateDigest(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	actorID := "system"
	if claims != nil && claims.UserID != "" {
		actorID = claims.UserID
	}

	var req CreateDigestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	pStart, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		pStart = time.Now().AddDate(0, -1, 0)
	}
	pEnd, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		pEnd = time.Now()
	}

	accessLevel := digest.AccessLevelPremium
	if req.AccessLevel == "free" {
		accessLevel = digest.AccessLevelFree
	}

	dType := digest.DigestTypeComprehensive
	if req.DigestType == "round_up" {
		dType = digest.DigestTypeRoundUp
	}

	period := digest.PeriodMonthly
	if req.Period != "" {
		period = digest.DigestPeriod(req.Period)
	}

	input := commands.CreateDigestInput{
		Title:              req.Title,
		Court:              req.Court,
		CourtCode:          req.CourtCode,
		DigestType:         dType,
		Period:             period,
		PeriodStart:        pStart,
		PeriodEnd:          pEnd,
		CitationRangeStart: req.CitationRangeStart,
		CitationRangeEnd:   req.CitationRangeEnd,
		CitationPrefix:     req.CitationPrefix,
		Intro:              req.Intro,
		Conclusion:         req.Conclusion,
		AccessLevel:        accessLevel,
		CreatedBy:          actorID,
	}

	d, err := h.commands.Create(r.Context(), input)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusCreated, d)
}

// GetCMSDigest handles GET /v1/cms/digests/{id}.
func (h *DigestHandler) GetCMSDigest(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Digest ID is required", reqID)
		return
	}

	detail, err := h.queries.GetByID(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "Digest not found", reqID)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

// AutoAssemble handles POST /v1/cms/digests/{id}/auto-assemble.
func (h *DigestHandler) AutoAssemble(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Digest ID is required", reqID)
		return
	}

	d, err := h.commands.AutoAssemble(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		return
	}

	// Fetch detail with newly generated indexes
	detail, _ := h.queries.GetByID(r.Context(), d.ID)
	writeJSON(w, http.StatusOK, detail)
}

// SplitPartsRequest holds input for splitting digest into parts.
type SplitPartsRequest struct {
	Parts int `json:"parts"`
}

// SplitDigest handles POST /v1/cms/digests/{id}/split-parts.
func (h *DigestHandler) SplitDigest(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Digest ID is required", reqID)
		return
	}

	var req SplitPartsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Parts <= 1 {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Number of parts must be 2 or greater", reqID)
		return
	}

	parts, err := h.commands.SplitIntoParts(r.Context(), id, req.Parts)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "SPLIT_FAILED", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"parts": parts,
		"count": len(parts),
	})
}

// PublishDigest handles POST /v1/cms/digests/{id}/publish.
func (h *DigestHandler) PublishDigest(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Digest ID is required", reqID)
		return
	}

	claims := GetAuthClaims(r.Context())
	actorID := "system"
	if claims != nil && claims.UserID != "" {
		actorID = claims.UserID
	}

	d, art, err := h.commands.Publish(r.Context(), id, actorID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "PUBLISH_FAILED", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"digest":  d,
		"article": art,
	})
}

// GenerateMonthlyDrafts handles POST /v1/cms/digests/generate-monthly.
func (h *DigestHandler) GenerateMonthlyDrafts(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	actorID := "system"
	if claims != nil && claims.UserID != "" {
		actorID = claims.UserID
	}

	created, err := h.commands.GenerateMonthlyDrafts(r.Context(), actorID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate monthly digests", reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"created": created,
		"count":   len(created),
	})
}
