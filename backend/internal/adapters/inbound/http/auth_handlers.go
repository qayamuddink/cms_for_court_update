package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/identity"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authUseCases *commands.AuthUseCases
	userQueries  *queries.UserQueries
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authUseCases *commands.AuthUseCases, userQueries *queries.UserQueries) *AuthHandler {
	return &AuthHandler{
		authUseCases: authUseCases,
		userQueries:  userQueries,
	}
}

// DTOs for requests and responses

type RegisterRequest struct {
	Email            string  `json:"email"`
	Password         string  `json:"password"`
	FullName         string  `json:"full_name"`
	DisplayRole      string  `json:"display_role,omitempty"`
	BarCouncilNumber *string `json:"bar_council_number,omitempty"`
	Phone            *string `json:"phone,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type UserProfileResponse struct {
	ID               string               `json:"id"`
	Email            string               `json:"email"`
	FullName         string               `json:"full_name"`
	DisplayRole      identity.DisplayRole `json:"display_role"`
	SystemRole       identity.SystemRole  `json:"system_role"`
	BarCouncilNumber *string              `json:"bar_council_number,omitempty"`
	Phone            *string              `json:"phone,omitempty"`
	Status           identity.UserStatus  `json:"status"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type AuthResponse struct {
	User         UserProfileResponse `json:"user"`
	AccessToken  string              `json:"access_token"`
	RefreshToken string              `json:"refresh_token"`
	ExpiresAt    time.Time           `json:"expires_at"`
}

func toUserProfileResponse(u *identity.User) UserProfileResponse {
	return UserProfileResponse{
		ID:               u.ID,
		Email:            u.Email,
		FullName:         u.FullName,
		DisplayRole:      u.DisplayRole,
		SystemRole:       u.SystemRole,
		BarCouncilNumber: u.BarCouncilNumber,
		Phone:            u.Phone,
		Status:           u.Status,
		CreatedAt:        u.CreatedAt,
		UpdatedAt:        u.UpdatedAt,
	}
}

// Register handles POST /v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	result, err := h.authUseCases.Register(r.Context(), commands.RegisterCommand{
		Email:            req.Email,
		Password:         req.Password,
		FullName:         req.FullName,
		DisplayRole:      identity.DisplayRole(req.DisplayRole),
		BarCouncilNumber: req.BarCouncilNumber,
		Phone:            req.Phone,
	})

	if err != nil {
		if errors.Is(err, commands.ErrEmailAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "EMAIL_EXISTS", "Email is already registered", reqID)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), reqID)
		return
	}

	resp := AuthResponse{
		User:         toUserProfileResponse(result.User),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
	}

	writeJSON(w, http.StatusCreated, resp)
}

// Login handles POST /v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	result, err := h.authUseCases.Login(r.Context(), commands.LoginCommand{
		Email:    req.Email,
		Password: req.Password,
	})

	if err != nil {
		switch {
		case errors.Is(err, commands.ErrInvalidCredentials):
			writeJSONError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password", reqID)
		case errors.Is(err, commands.ErrUserSuspended):
			writeJSONError(w, http.StatusForbidden, "USER_SUSPENDED", "User account is suspended", reqID)
		case errors.Is(err, commands.ErrUserDeleted):
			writeJSONError(w, http.StatusForbidden, "USER_DELETED", "User account has been deleted", reqID)
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		}
		return
	}

	resp := AuthResponse{
		User:         toUserProfileResponse(result.User),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
	}

	writeJSON(w, http.StatusOK, resp)
}

// Refresh handles POST /v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())

	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	result, err := h.authUseCases.RefreshToken(r.Context(), commands.RefreshCommand{
		RefreshToken: req.RefreshToken,
	})

	if err != nil {
		switch {
		case errors.Is(err, commands.ErrInvalidToken):
			writeJSONError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid or expired refresh token", reqID)
		case errors.Is(err, commands.ErrUserSuspended):
			writeJSONError(w, http.StatusForbidden, "USER_SUSPENDED", "User account is suspended", reqID)
		case errors.Is(err, commands.ErrUserDeleted):
			writeJSONError(w, http.StatusForbidden, "USER_DELETED", "User account has been deleted", reqID)
		default:
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Refresh failed", reqID)
		}
		return
	}

	resp := AuthResponse{
		User:         toUserProfileResponse(result.User),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
	}

	writeJSON(w, http.StatusOK, resp)
}

// Logout handles POST /v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	_ = h.authUseCases.Logout(r.Context(), commands.LogoutCommand{
		RefreshToken: req.RefreshToken,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Me handles GET /v1/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	user, err := h.userQueries.GetMyProfile(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, queries.ErrUserNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "User profile not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve profile", reqID)
		return
	}

	writeJSON(w, http.StatusOK, toUserProfileResponse(user))
}
