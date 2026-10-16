package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/identity"
	"github.com/livelaw/backend/internal/domain/livecase"
)

// CMSCaseHandler handles case management and courtroom reporting operations for staff.
type CMSCaseHandler struct {
	useCases *commands.LiveCaseUseCases
	queries  *queries.LiveCaseQueries
}

// NewCMSCaseHandler creates a new CMSCaseHandler.
func NewCMSCaseHandler(useCases *commands.LiveCaseUseCases, queries *queries.LiveCaseQueries) *CMSCaseHandler {
	return &CMSCaseHandler{
		useCases: useCases,
		queries:  queries,
	}
}

// CreateCaseRequest holds parameters for creating a court case.
type CreateCaseRequest struct {
	CourtName         string     `json:"court_name"`
	CaseNumber        string     `json:"case_number"`
	CaseType          string     `json:"case_type"`
	Title             string     `json:"title"`
	BenchJudges       []string   `json:"bench_judges"`
	PresidingJudge    string     `json:"presiding_judge"`
	PetitionerParties []string   `json:"petitioner_parties"`
	RespondentParties []string   `json:"respondent_parties"`
	NextHearingAt     *time.Time `json:"next_hearing_at,omitempty"`
}

// UpdateCaseRequest holds parameters for updating a court case.
type UpdateCaseRequest struct {
	CourtName         string              `json:"court_name"`
	CaseNumber        string              `json:"case_number"`
	CaseType          string              `json:"case_type"`
	Title             string              `json:"title"`
	BenchJudges       []string            `json:"bench_judges"`
	PresidingJudge    string              `json:"presiding_judge"`
	PetitionerParties []string            `json:"petitioner_parties"`
	RespondentParties []string            `json:"respondent_parties"`
	Status            livecase.CaseStatus `json:"status"`
	NextHearingAt     *time.Time          `json:"next_hearing_at,omitempty"`
}

// SetLiveRequest holds parameters for toggling case live state.
type SetLiveRequest struct {
	IsLive bool `json:"is_live"`
}

// AssignReporterRequest holds parameters for assigning a reporter to a case.
type AssignReporterRequest struct {
	UserID string `json:"user_id"`
}

// ReportersResponse wraps list of reporter user IDs.
type ReportersResponse struct {
	ReporterIDs []string `json:"reporter_ids"`
}

// PublishUpdatePayload carries parameters for appending a courtroom update.
type PublishUpdatePayload struct {
	UpdateType     livecase.UpdateType `json:"update_type"`
	SpeakerLabel   *string             `json:"speaker_label,omitempty"`
	Content        string              `json:"content"`
	EventTime      time.Time           `json:"event_time,omitempty"`
	IdempotencyKey *string             `json:"idempotency_key,omitempty"`
}

// CorrectUpdatePayload carries parameters for correcting an update.
type CorrectUpdatePayload struct {
	UpdateType     livecase.UpdateType `json:"update_type"`
	SpeakerLabel   *string             `json:"speaker_label,omitempty"`
	Content        string              `json:"content"`
	IdempotencyKey *string             `json:"idempotency_key,omitempty"`
}

// RetractUpdatePayload carries parameters for retracting an update.
type RetractUpdatePayload struct {
	Reason         string  `json:"reason"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

// CreateCase handles POST /v1/cms/cases (Editor, Admin)
func (h *CMSCaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req CreateCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	c, err := h.useCases.CreateCase(r.Context(), commands.CreateCaseCommand{
		ActorRole:         identity.SystemRole(claims.SystemRole),
		CourtName:         req.CourtName,
		CaseNumber:        req.CaseNumber,
		CaseType:          req.CaseType,
		Title:             req.Title,
		BenchJudges:       req.BenchJudges,
		PresidingJudge:    req.PresidingJudge,
		PetitionerParties: req.PetitionerParties,
		RespondentParties: req.RespondentParties,
		NextHearingAt:     req.NextHearingAt,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusCreated, c)
}

// UpdateCase handles PATCH /v1/cms/cases/{id} (Editor, Admin)
func (h *CMSCaseHandler) UpdateCase(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID is required", reqID)
		return
	}

	var req UpdateCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	c, err := h.useCases.UpdateCase(r.Context(), commands.UpdateCaseCommand{
		ActorRole:         identity.SystemRole(claims.SystemRole),
		CaseID:            caseID,
		CourtName:         req.CourtName,
		CaseNumber:        req.CaseNumber,
		CaseType:          req.CaseType,
		Title:             req.Title,
		BenchJudges:       req.BenchJudges,
		PresidingJudge:    req.PresidingJudge,
		PetitionerParties: req.PetitionerParties,
		RespondentParties: req.RespondentParties,
		Status:            req.Status,
		NextHearingAt:     req.NextHearingAt,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// SetCaseLive handles POST /v1/cms/cases/{id}/live (Editor, Admin)
func (h *CMSCaseHandler) SetCaseLive(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID is required", reqID)
		return
	}

	var req SetLiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	c, err := h.useCases.SetCaseLive(r.Context(), identity.SystemRole(claims.SystemRole), caseID, req.IsLive)
	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// AssignReporter handles POST /v1/cms/cases/{id}/assign (Editor, Admin)
func (h *CMSCaseHandler) AssignReporter(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID is required", reqID)
		return
	}

	var req AssignReporterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	if err := h.useCases.AssignReporter(r.Context(), identity.SystemRole(claims.SystemRole), caseID, req.UserID); err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

// UnassignReporter handles DELETE /v1/cms/cases/{id}/assign/{userId} (Editor, Admin)
func (h *CMSCaseHandler) UnassignReporter(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	caseID := r.PathValue("id")
	targetUserID := r.PathValue("userId")
	if caseID == "" || targetUserID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID and user ID are required", reqID)
		return
	}

	if err := h.useCases.UnassignReporter(r.Context(), identity.SystemRole(claims.SystemRole), caseID, targetUserID); err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unassigned"})
}

// ListReporters handles GET /v1/cms/cases/{id}/reporters (Editor, Admin)
func (h *CMSCaseHandler) ListReporters(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID is required", reqID)
		return
	}

	reporters, err := h.queries.ListAssignedReporters(r.Context(), caseID)
	if err != nil {
		if errors.Is(err, queries.ErrCaseNotFound) {
			writeJSONError(w, http.StatusNotFound, "CASE_NOT_FOUND", "Court case not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve assigned reporters", reqID)
		return
	}

	if reporters == nil {
		reporters = []string{}
	}

	writeJSON(w, http.StatusOK, ReportersResponse{ReporterIDs: reporters})
}

// PublishUpdate handles POST /v1/cms/cases/{id}/updates (Reporter, Editor, Admin)
func (h *CMSCaseHandler) PublishUpdate(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID is required", reqID)
		return
	}

	var req PublishUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	idempotencyKey := h.extractIdempotencyKey(r, req.IdempotencyKey)

	update, err := h.useCases.PublishLiveUpdate(r.Context(), commands.PublishLiveUpdateCommand{
		ActorRole:      identity.SystemRole(claims.SystemRole),
		ReporterUserID: claims.UserID,
		CaseID:         caseID,
		UpdateType:     req.UpdateType,
		SpeakerLabel:   req.SpeakerLabel,
		Content:        req.Content,
		EventTime:      req.EventTime,
		IdempotencyKey: idempotencyKey,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusCreated, update)
}

// CorrectUpdate handles POST /v1/cms/cases/{id}/updates/{updateId}/correct (Reporter, Editor, Admin)
func (h *CMSCaseHandler) CorrectUpdate(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	caseID := r.PathValue("id")
	updateID := r.PathValue("updateId")
	if caseID == "" || updateID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Case ID and update ID are required", reqID)
		return
	}

	var req CorrectUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	idempotencyKey := h.extractIdempotencyKey(r, req.IdempotencyKey)

	update, err := h.useCases.CorrectLiveUpdate(r.Context(), commands.CorrectLiveUpdateCommand{
		ActorRole:      identity.SystemRole(claims.SystemRole),
		ReporterUserID: claims.UserID,
		CaseID:         caseID,
		TargetUpdateID: updateID,
		UpdateType:     req.UpdateType,
		SpeakerLabel:   req.SpeakerLabel,
		Content:        req.Content,
		IdempotencyKey: idempotencyKey,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusCreated, update)
}

// RetractUpdate handles POST /v1/cms/cases/{id}/updates/{updateId}/retract (Reporter, Editor, Admin)
func (h *CMSCaseHandler) RetractUpdate(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	caseID := r.PathValue("id")
	updateID := r.PathValue("updateId")
	if caseID == "" || updateID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Case ID and update ID are required", reqID)
		return
	}

	var req RetractUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	idempotencyKey := h.extractIdempotencyKey(r, req.IdempotencyKey)

	update, err := h.useCases.RetractLiveUpdate(r.Context(), commands.RetractLiveUpdateCommand{
		ActorRole:      identity.SystemRole(claims.SystemRole),
		ReporterUserID: claims.UserID,
		CaseID:         caseID,
		TargetUpdateID: updateID,
		Reason:         req.Reason,
		IdempotencyKey: idempotencyKey,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusCreated, update)
}

func (h *CMSCaseHandler) extractIdempotencyKey(r *http.Request, bodyKey *string) *string {
	if headerKey := strings.TrimSpace(r.Header.Get("Idempotency-Key")); headerKey != "" {
		return &headerKey
	}
	if bodyKey != nil && strings.TrimSpace(*bodyKey) != "" {
		trimmed := strings.TrimSpace(*bodyKey)
		return &trimmed
	}
	return nil
}

func (h *CMSCaseHandler) handleError(w http.ResponseWriter, err error, reqID string) {
	switch {
	case errors.Is(err, commands.ErrForbidden):
		writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error(), reqID)
	case errors.Is(err, commands.ErrReporterNotAssigned):
		writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error(), reqID)
	case errors.Is(err, commands.ErrCaseNotFound):
		writeJSONError(w, http.StatusNotFound, "CASE_NOT_FOUND", err.Error(), reqID)
	case errors.Is(err, commands.ErrUpdateNotFound):
		writeJSONError(w, http.StatusNotFound, "UPDATE_NOT_FOUND", err.Error(), reqID)
	case errors.Is(err, commands.ErrUserNotFound):
		writeJSONError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error(), reqID)
	case errors.Is(err, commands.ErrCaseClosed):
		writeJSONError(w, http.StatusBadRequest, "CASE_CLOSED", err.Error(), reqID)
	case errors.Is(err, commands.ErrVerdictAlreadyPosted):
		writeJSONError(w, http.StatusConflict, "VERDICT_ALREADY_EXISTS", err.Error(), reqID)
	case errors.Is(err, commands.ErrCannotModifyRetracted):
		writeJSONError(w, http.StatusBadRequest, "CANNOT_MODIFY_RETRACTED", err.Error(), reqID)
	case errors.Is(err, commands.ErrUpdateAlreadyRetracted):
		writeJSONError(w, http.StatusBadRequest, "ALREADY_RETRACTED", err.Error(), reqID)
	case errors.Is(err, commands.ErrInvalidReporterAccount):
		writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ROLE", err.Error(), reqID)
	default:
		// Check for input validation errors
		msg := err.Error()
		if strings.Contains(msg, "required") || strings.Contains(msg, "invalid") || strings.Contains(msg, "characters") {
			writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", msg, reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", reqID)
	}
}
