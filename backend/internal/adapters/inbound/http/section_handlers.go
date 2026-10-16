package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
	"github.com/livelaw/backend/internal/domain/identity"
)

// SectionHandler handles public and CMS HTTP requests for taxonomy sections.
type SectionHandler struct {
	useCases *commands.SectionUseCases
}

// NewSectionHandler creates a new SectionHandler.
func NewSectionHandler(useCases *commands.SectionUseCases) *SectionHandler {
	return &SectionHandler{useCases: useCases}
}

// ListPublic handles GET /v1/sections (active sections for navigation)
func (h *SectionHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	sections, err := h.useCases.ListSections(r.Context(), true)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve sections", reqID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sections": sections})
}

// GetBySlug handles GET /v1/sections/{slug}
func (h *SectionHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	slug := r.PathValue("slug")
	if slug == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_SLUG", "Section slug is required", reqID)
		return
	}

	sec, err := h.useCases.GetSectionBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, ports.ErrSectionNotFound) {
			writeJSONError(w, http.StatusNotFound, "SECTION_NOT_FOUND", "Section not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve section", reqID)
		return
	}

	writeJSON(w, http.StatusOK, sec)
}

// ListCMS handles GET /v1/cms/sections (all sections, active and inactive)
func (h *SectionHandler) ListCMS(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	sections, err := h.useCases.ListSections(r.Context(), false)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve sections", reqID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sections": sections})
}

// CreateSectionPayload represents JSON body for creating a section.
type CreateSectionPayload struct {
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  *string `json:"description,omitempty"`
	DisplayOrder int     `json:"display_order"`
	ParentID     *string `json:"parent_id,omitempty"`
	IsActive     *bool   `json:"is_active,omitempty"`
}

// Create handles POST /v1/cms/sections
func (h *SectionHandler) Create(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req CreateSectionPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	sec, err := h.useCases.CreateSection(r.Context(), commands.CreateSectionCommand{
		ActorRole:    identity.SystemRole(claims.SystemRole),
		Name:         req.Name,
		Slug:         req.Slug,
		Description:  req.Description,
		DisplayOrder: req.DisplayOrder,
		ParentID:     req.ParentID,
		IsActive:     req.IsActive,
	})

	if err != nil {
		if errors.Is(err, ports.ErrSectionSlugExists) {
			writeJSONError(w, http.StatusConflict, "SLUG_ALREADY_EXISTS", "A section with this slug already exists", reqID)
			return
		}
		if errors.Is(err, content.ErrUnauthorizedWorkflow) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to manage sections", reqID)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusCreated, sec)
}

// UpdateSectionPayload represents JSON body for updating a section.
type UpdateSectionPayload struct {
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  *string `json:"description,omitempty"`
	DisplayOrder int     `json:"display_order"`
	ParentID     *string `json:"parent_id,omitempty"`
	IsActive     bool    `json:"is_active"`
}

// Update handles PUT /v1/cms/sections/{id}
func (h *SectionHandler) Update(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Section ID is required", reqID)
		return
	}

	var req UpdateSectionPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	sec, err := h.useCases.UpdateSection(r.Context(), commands.UpdateSectionCommand{
		ActorRole:    identity.SystemRole(claims.SystemRole),
		ID:           id,
		Name:         req.Name,
		Slug:         req.Slug,
		Description:  req.Description,
		DisplayOrder: req.DisplayOrder,
		ParentID:     req.ParentID,
		IsActive:     req.IsActive,
	})

	if err != nil {
		if errors.Is(err, ports.ErrSectionNotFound) {
			writeJSONError(w, http.StatusNotFound, "SECTION_NOT_FOUND", "Section not found", reqID)
			return
		}
		if errors.Is(err, ports.ErrSectionSlugExists) {
			writeJSONError(w, http.StatusConflict, "SLUG_ALREADY_EXISTS", "A section with this slug already exists", reqID)
			return
		}
		if errors.Is(err, content.ErrUnauthorizedWorkflow) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to manage sections", reqID)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, sec)
}

// Delete handles DELETE /v1/cms/sections/{id}
func (h *SectionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Section ID is required", reqID)
		return
	}

	if err := h.useCases.DeleteSection(r.Context(), id, identity.SystemRole(claims.SystemRole)); err != nil {
		if errors.Is(err, ports.ErrSectionNotFound) {
			writeJSONError(w, http.StatusNotFound, "SECTION_NOT_FOUND", "Section not found", reqID)
			return
		}
		if errors.Is(err, content.ErrUnauthorizedWorkflow) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Only administrators can delete sections", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete section", reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
