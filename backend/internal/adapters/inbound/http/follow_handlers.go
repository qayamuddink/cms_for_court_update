package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
)

// FollowHandler handles court case follows and user push device token registration.
type FollowHandler struct {
	useCases *commands.FollowUseCases
	queries  *queries.FollowQueries
}

// NewFollowHandler creates a new FollowHandler instance.
func NewFollowHandler(useCases *commands.FollowUseCases, queries *queries.FollowQueries) *FollowHandler {
	return &FollowHandler{
		useCases: useCases,
		queries:  queries,
	}
}

type followCaseReq struct {
	NotifyOn []string `json:"notify_on"`
}

// FollowCase handles POST /v1/cases/{id}/follow
func (h *FollowHandler) FollowCase(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Case ID is required", reqID)
		return
	}

	var req followCaseReq
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.useCases.FollowCase(r.Context(), claims.UserID, caseID, req.NotifyOn); err != nil {
		if errors.Is(err, ports.ErrCaseNotFound) {
			writeJSONError(w, http.StatusNotFound, "CASE_NOT_FOUND", "Court case not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "following",
		"case_id":   caseID,
		"notify_on": req.NotifyOn,
	})
}

// UnfollowCase handles DELETE /v1/cases/{id}/follow
func (h *FollowHandler) UnfollowCase(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Case ID is required", reqID)
		return
	}

	if err := h.useCases.UnfollowCase(r.Context(), claims.UserID, caseID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "unfollowed",
		"case_id": caseID,
	})
}

// GetCaseFollowStatus handles GET /v1/cases/{id}/follow
func (h *FollowHandler) GetCaseFollowStatus(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Case ID is required", reqID)
		return
	}

	var userID string
	if claims := GetAuthClaims(r.Context()); claims != nil {
		userID = claims.UserID
	}

	status, err := h.queries.GetCaseFollowStatus(r.Context(), userID, caseID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

type registerDeviceReq struct {
	Platform  string `json:"platform"`
	PushToken string `json:"push_token"`
}

// RegisterDevice handles POST /v1/devices
func (h *FollowHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req registerDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	if err := h.useCases.RegisterDevice(r.Context(), claims.UserID, req.Platform, req.PushToken); err != nil {
		if errors.Is(err, commands.ErrInvalidPlatform) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_PLATFORM", err.Error(), reqID)
			return
		}
		if errors.Is(err, commands.ErrEmptyPushToken) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_TOKEN", err.Error(), reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"status":   "registered",
		"platform": req.Platform,
	})
}

type unregisterDeviceReq struct {
	PushToken string `json:"push_token"`
}

// UnregisterDevice handles DELETE /v1/devices
func (h *FollowHandler) UnregisterDevice(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	pushToken := r.URL.Query().Get("push_token")
	if pushToken == "" {
		var req unregisterDeviceReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		pushToken = req.PushToken
	}

	if pushToken == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_TOKEN", "push_token is required", reqID)
		return
	}

	if err := h.useCases.UnregisterDevice(r.Context(), claims.UserID, pushToken); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "unregistered",
	})
}
