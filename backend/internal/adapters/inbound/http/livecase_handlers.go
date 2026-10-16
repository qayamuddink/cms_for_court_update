package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/livecase"
)

// LiveCasePublicHandler handles public reader queries for court cases and live updates.
type LiveCasePublicHandler struct {
	queries    *queries.LiveCaseQueries
	liveEngine ports.LiveEnginePort
}

// NewLiveCasePublicHandler creates a new LiveCasePublicHandler.
func NewLiveCasePublicHandler(q *queries.LiveCaseQueries, liveEngine ports.LiveEnginePort) *LiveCasePublicHandler {
	return &LiveCasePublicHandler{
		queries:    q,
		liveEngine: liveEngine,
	}
}

// LiveCasesResponse wraps a slice of active court cases.
type LiveCasesResponse struct {
	Cases []livecase.CourtCase `json:"cases"`
}

// LiveUpdatesResponse wraps a slice of live courtroom updates.
type LiveUpdatesResponse struct {
	Updates []livecase.LiveUpdate `json:"updates"`
}

// ListLiveCases handles GET /v1/cases/live
func (h *LiveCasePublicHandler) ListLiveCases(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	cases, err := h.queries.ListLiveCases(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve live cases", reqID)
		return
	}

	if cases == nil {
		cases = []livecase.CourtCase{}
	}

	writeJSON(w, http.StatusOK, LiveCasesResponse{Cases: cases})
}

// GetCase handles GET /v1/cases/{id}
func (h *LiveCasePublicHandler) GetCase(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID is required", reqID)
		return
	}

	c, err := h.queries.GetCase(r.Context(), caseID)
	if err != nil {
		if errors.Is(err, queries.ErrCaseNotFound) {
			writeJSONError(w, http.StatusNotFound, "CASE_NOT_FOUND", "Court case not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve court case", reqID)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// GetUpdates handles GET /v1/cases/{id}/updates?after_sequence=N&limit=M
func (h *LiveCasePublicHandler) GetUpdates(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID is required", reqID)
		return
	}

	var afterSeq int64 = 0
	if afterParam := r.URL.Query().Get("after_sequence"); afterParam != "" {
		parsed, err := strconv.ParseInt(afterParam, 10, 64)
		if err != nil || parsed < 0 {
			writeJSONError(w, http.StatusBadRequest, "INVALID_PARAMETER", "after_sequence must be a non-negative integer", reqID)
			return
		}
		afterSeq = parsed
	}

	limit := 50
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		parsed, err := strconv.Atoi(limitParam)
		if err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	updates, err := h.queries.GetUpdatesAfter(r.Context(), caseID, afterSeq, limit)
	if err != nil {
		if errors.Is(err, queries.ErrCaseNotFound) {
			writeJSONError(w, http.StatusNotFound, "CASE_NOT_FOUND", "Court case not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve updates", reqID)
		return
	}

	if updates == nil {
		updates = []livecase.LiveUpdate{}
	}

	writeJSON(w, http.StatusOK, LiveUpdatesResponse{Updates: updates})
}

// GetLatestUpdates handles GET /v1/cases/{id}/updates/latest?limit=M
func (h *LiveCasePublicHandler) GetLatestUpdates(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID is required", reqID)
		return
	}

	limit := 50
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		parsed, err := strconv.Atoi(limitParam)
		if err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	updates, err := h.queries.GetLatestUpdates(r.Context(), caseID, limit)
	if err != nil {
		if errors.Is(err, queries.ErrCaseNotFound) {
			writeJSONError(w, http.StatusNotFound, "CASE_NOT_FOUND", "Court case not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve latest updates", reqID)
		return
	}

	if updates == nil {
		updates = []livecase.LiveUpdate{}
	}

	writeJSON(w, http.StatusOK, LiveUpdatesResponse{Updates: updates})
}

// StreamUpdates handles GET /v1/cases/{id}/stream using Server-Sent Events (SSE).
// Sends the sequence number as the SSE event ID so clients can detect any gaps.
func (h *LiveCasePublicHandler) StreamUpdates(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID is required", reqID)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "STREAMING_UNSUPPORTED", "Streaming not supported by server", reqID)
		return
	}

	// Verify case exists
	_, err := h.queries.GetCase(r.Context(), caseID)
	if err != nil {
		if errors.Is(err, queries.ErrCaseNotFound) {
			writeJSONError(w, http.StatusNotFound, "CASE_NOT_FOUND", "Court case not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed retrieving court case", reqID)
		return
	}

	var afterSeq int64 = 0
	if lastEventID := r.Header.Get("Last-Event-ID"); lastEventID != "" {
		if parsed, err := strconv.ParseInt(lastEventID, 10, 64); err == nil && parsed >= 0 {
			afterSeq = parsed
		}
	} else if queryAfter := r.URL.Query().Get("after_sequence"); queryAfter != "" {
		if parsed, err := strconv.ParseInt(queryAfter, 10, 64); err == nil && parsed >= 0 {
			afterSeq = parsed
		}
	}

	if h.liveEngine == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "LIVE_ENGINE_UNAVAILABLE", "Live streaming engine unavailable", reqID)
		return
	}

	ch, err := h.liveEngine.Subscribe(r.Context(), caseID, afterSeq)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "SUBSCRIBE_FAILED", "Failed subscribing to live stream", reqID)
		return
	}

	// Set SSE response headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeatTicker.C:
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case update, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(update)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "id: %d\nevent: live_update\ndata: %s\n\n", update.SequenceNo, data)
			flusher.Flush()
		}
	}
}
