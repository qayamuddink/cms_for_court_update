package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/identity"
)

// CMSUserHandler handles admin staff management endpoints.
type CMSUserHandler struct {
	authUseCases *commands.AuthUseCases
	userQueries  *queries.UserQueries
}

// NewCMSUserHandler creates a new CMSUserHandler.
func NewCMSUserHandler(authUseCases *commands.AuthUseCases, userQueries *queries.UserQueries) *CMSUserHandler {
	return &CMSUserHandler{
		authUseCases: authUseCases,
		userQueries:  userQueries,
	}
}

type CreateStaffRequest struct {
	Email            string  `json:"email"`
	Password         string  `json:"password"`
	FullName         string  `json:"full_name"`
	SystemRole       string  `json:"system_role"`
	DisplayRole      string  `json:"display_role,omitempty"`
	BarCouncilNumber *string `json:"bar_council_number,omitempty"`
	Phone            *string `json:"phone,omitempty"`
}

type UpdateUserRequest struct {
	SystemRole *string `json:"system_role,omitempty"`
	Status     *string `json:"status,omitempty"`
}

type UsersListResponse struct {
	Users      []UserProfileResponse `json:"users"`
	NextCursor *string               `json:"next_cursor,omitempty"`
}

// CreateStaff handles POST /v1/cms/users
func (h *CMSUserHandler) CreateStaff(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req CreateStaffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	systemRole := identity.SystemRole(req.SystemRole)
	if !identity.IsStaffRole(systemRole) {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ROLE", "Only staff roles (reporter, editor, admin) can be created", reqID)
		return
	}

	displayRole := identity.DisplayRole(req.DisplayRole)
	if displayRole == "" {
		displayRole = identity.DisplayLawyer
	}

	staff, err := h.authUseCases.AdminCreateStaff(r.Context(), commands.AdminCreateStaffCommand{
		AdminUserID:      claims.UserID,
		Email:            req.Email,
		Password:         req.Password,
		FullName:         req.FullName,
		SystemRole:       systemRole,
		DisplayRole:      displayRole,
		BarCouncilNumber: req.BarCouncilNumber,
		Phone:            req.Phone,
	})

	if err != nil {
		switch {
		case errors.Is(err, commands.ErrForbidden):
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Only admins can create staff accounts", reqID)
		case errors.Is(err, commands.ErrEmailAlreadyExists):
			writeJSONError(w, http.StatusConflict, "EMAIL_EXISTS", "Email is already registered", reqID)
		default:
			writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), reqID)
		}
		return
	}

	writeJSON(w, http.StatusCreated, toUserProfileResponse(staff))
}

// ListUsers handles GET /v1/cms/users
func (h *CMSUserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	var beforeCreatedAt *time.Time
	if bTime := r.URL.Query().Get("before_created_at"); bTime != "" {
		if t, err := time.Parse(time.RFC3339, bTime); err == nil {
			beforeCreatedAt = &t
		}
	}

	var beforeID *string
	if bID := r.URL.Query().Get("before_id"); bID != "" {
		beforeID = &bID
	}

	users, err := h.userQueries.AdminListUsers(r.Context(), claims.UserID, limit, beforeCreatedAt, beforeID)
	if err != nil {
		if errors.Is(err, queries.ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Only admins can list users", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list users", reqID)
		return
	}

	var userResponses []UserProfileResponse
	for _, u := range users {
		userResponses = append(userResponses, toUserProfileResponse(&u))
	}

	var nextCursor *string
	if len(users) == limit {
		lastUser := users[len(users)-1]
		cursor := lastUser.CreatedAt.Format(time.RFC3339) + "_" + lastUser.ID
		nextCursor = &cursor
	}

	writeJSON(w, http.StatusOK, UsersListResponse{
		Users:      userResponses,
		NextCursor: nextCursor,
	})
}

// UpdateUser handles PATCH /v1/cms/users/{id}
func (h *CMSUserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	targetID := r.PathValue("id")
	if targetID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Missing user ID in path", reqID)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	var sysRole *identity.SystemRole
	if req.SystemRole != nil {
		role := identity.SystemRole(*req.SystemRole)
		if !identity.IsValidSystemRole(role) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ROLE", "Invalid system role", reqID)
			return
		}
		sysRole = &role
	}

	var status *identity.UserStatus
	if req.Status != nil {
		s := identity.UserStatus(*req.Status)
		switch s {
		case identity.StatusActive, identity.StatusSuspended, identity.StatusDeleted:
			status = &s
		default:
			writeJSONError(w, http.StatusBadRequest, "INVALID_STATUS", "Invalid status (active, suspended, deleted)", reqID)
			return
		}
	}

	updated, err := h.authUseCases.AdminUpdateUser(r.Context(), commands.AdminUpdateUserCommand{
		AdminUserID:  claims.UserID,
		TargetUserID: targetID,
		SystemRole:   sysRole,
		Status:       status,
	})

	if err != nil {
		switch {
		case errors.Is(err, commands.ErrForbidden):
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Only admins can modify users", reqID)
		case errors.Is(err, commands.ErrUserNotFound):
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "User not found", reqID)
		default:
			writeJSONError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error(), reqID)
		}
		return
	}

	writeJSON(w, http.StatusOK, toUserProfileResponse(updated))
}
