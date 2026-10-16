package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/content"
	"github.com/livelaw/backend/internal/domain/identity"
)

// ArticlePublicHandler handles public reader endpoints for news feed, articles, and judgments.
type ArticlePublicHandler struct {
	queries *queries.ArticleQueries
}

// NewArticlePublicHandler creates a new ArticlePublicHandler.
func NewArticlePublicHandler(q *queries.ArticleQueries) *ArticlePublicHandler {
	return &ArticlePublicHandler{queries: q}
}

// ListFeed handles GET /v1/articles
func (h *ArticlePublicHandler) ListFeed(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	filter := ports.ArticleFilter{
		Limit: limit,
	}

	if t := r.URL.Query().Get("type"); t != "" {
		artType := content.ArticleType(t)
		filter.ArticleType = &artType
	}
	if cat := r.URL.Query().Get("category"); cat != "" {
		filter.Category = &cat
	}
	if tag := r.URL.Query().Get("tag"); tag != "" {
		filter.Tag = &tag
	}
	if caseID := r.URL.Query().Get("court_case_id"); caseID != "" {
		filter.CourtCaseID = &caseID
	}
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		filter.Cursor = &cursor
	}

	feed, err := h.queries.GetFeed(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve feed", reqID)
		return
	}

	writeJSON(w, http.StatusOK, feed)
}

// GetBySlug handles GET /v1/articles/{slug}
func (h *ArticlePublicHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	slug := r.PathValue("slug")
	if slug == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_SLUG", "Article slug is required", reqID)
		return
	}

	var userID *string
	if claims := GetAuthClaims(r.Context()); claims != nil {
		userID = &claims.UserID
	}

	article, err := h.queries.GetArticleBySlug(r.Context(), slug, userID)
	if err != nil {
		if errors.Is(err, ports.ErrArticleNotFound) {
			writeJSONError(w, http.StatusNotFound, "ARTICLE_NOT_FOUND", "Article not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve article", reqID)
		return
	}

	writeJSON(w, http.StatusOK, article)
}

// GetJudgment handles GET /v1/judgments/{id}
func (h *ArticlePublicHandler) GetJudgment(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Judgment ID is required", reqID)
		return
	}

	judgment, err := h.queries.GetJudgment(r.Context(), id)
	if err != nil {
		if errors.Is(err, ports.ErrArticleNotFound) {
			writeJSONError(w, http.StatusNotFound, "JUDGMENT_NOT_FOUND", "Judgment summary not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, judgment)
}

// GetCaseJudgment handles GET /v1/cases/{id}/judgment
func (h *ArticlePublicHandler) GetCaseJudgment(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	caseID := r.PathValue("id")
	if caseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Court case ID is required", reqID)
		return
	}

	judgment, err := h.queries.GetJudgmentByCaseID(r.Context(), caseID)
	if err != nil {
		if errors.Is(err, ports.ErrArticleNotFound) {
			writeJSONError(w, http.StatusNotFound, "JUDGMENT_NOT_FOUND", "No judgment summary linked to this court case", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve judgment summary", reqID)
		return
	}

	writeJSON(w, http.StatusOK, judgment)
}

// CMSArticleHandler handles editorial back-office operations for articles and judgments.
type CMSArticleHandler struct {
	useCases *commands.ArticleUseCases
	queries  *queries.ArticleQueries
}

// NewCMSArticleHandler creates a new CMSArticleHandler.
func NewCMSArticleHandler(useCases *commands.ArticleUseCases, q *queries.ArticleQueries) *CMSArticleHandler {
	return &CMSArticleHandler{
		useCases: useCases,
		queries:  q,
	}
}

// CreateArticleDraftPayload represents the JSON payload to create a draft article.
type CreateArticleDraftPayload struct {
	Title       string              `json:"title"`
	Subtitle    *string             `json:"subtitle,omitempty"`
	Summary     string              `json:"summary"`
	BodyBlocks  json.RawMessage     `json:"body_blocks"`
	ArticleType content.ArticleType `json:"article_type"`
	AccessLevel content.AccessLevel `json:"access_level"`
	Category    string              `json:"category"`
	Tags        []string            `json:"tags"`
	CourtCaseID *string             `json:"court_case_id,omitempty"`
	CourtName   *string             `json:"court_name,omitempty"`
	Citation    *string             `json:"citation,omitempty"`
}

// CreateDraft handles POST /v1/cms/articles
func (h *CMSArticleHandler) CreateDraft(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req CreateArticleDraftPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	article, err := h.useCases.CreateDraft(r.Context(), commands.CreateDraftCommand{
		ActorID:     claims.UserID,
		ActorRole:   identity.SystemRole(claims.SystemRole),
		Title:       req.Title,
		Subtitle:    req.Subtitle,
		Summary:     req.Summary,
		BodyBlocks:  []byte(req.BodyBlocks),
		ArticleType: req.ArticleType,
		AccessLevel: req.AccessLevel,
		Category:    req.Category,
		Tags:        req.Tags,
		CourtCaseID: req.CourtCaseID,
		CourtName:   req.CourtName,
		Citation:    req.Citation,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusCreated, article)
}

// List handles GET /v1/cms/articles
func (h *CMSArticleHandler) List(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	filter := ports.ArticleFilter{
		Limit: limit,
	}

	if s := r.URL.Query().Get("status"); s != "" {
		status := content.ArticleStatus(s)
		filter.Status = &status
	}
	if t := r.URL.Query().Get("type"); t != "" {
		artType := content.ArticleType(t)
		filter.ArticleType = &artType
	}
	if cat := r.URL.Query().Get("category"); cat != "" {
		filter.Category = &cat
	}
	if authorID := r.URL.Query().Get("author_id"); authorID != "" {
		filter.AuthorID = &authorID
	}
	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = &search
	}

	resp, err := h.queries.ListCMSArticles(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve CMS articles", reqID)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/cms/articles/{id}
func (h *CMSArticleHandler) Get(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Article ID is required", reqID)
		return
	}

	article, err := h.queries.GetCMSArticleByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ports.ErrArticleNotFound) {
			writeJSONError(w, http.StatusNotFound, "ARTICLE_NOT_FOUND", "Article not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve article", reqID)
		return
	}

	writeJSON(w, http.StatusOK, article)
}

// UpdateArticleDraftPayload represents the JSON payload to update an article.
type UpdateArticleDraftPayload struct {
	ExpectedVersion int                 `json:"expected_version"`
	Title           string              `json:"title"`
	Subtitle        *string             `json:"subtitle,omitempty"`
	Summary         string              `json:"summary"`
	BodyBlocks      json.RawMessage     `json:"body_blocks"`
	ArticleType     content.ArticleType `json:"article_type"`
	AccessLevel     content.AccessLevel `json:"access_level"`
	Category        string              `json:"category"`
	Tags            []string            `json:"tags"`
	CourtCaseID     *string             `json:"court_case_id,omitempty"`
	CourtName       *string             `json:"court_name,omitempty"`
	Citation        *string             `json:"citation,omitempty"`
}

// UpdateDraft handles PATCH /v1/cms/articles/{id}
func (h *CMSArticleHandler) UpdateDraft(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Article ID is required", reqID)
		return
	}

	var req UpdateArticleDraftPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	if req.ExpectedVersion <= 0 {
		writeJSONError(w, http.StatusBadRequest, "MISSING_VERSION", "expected_version is required for optimistic concurrency", reqID)
		return
	}

	article, err := h.useCases.UpdateDraft(r.Context(), commands.UpdateDraftCommand{
		ID:              id,
		ActorID:         claims.UserID,
		ActorRole:       identity.SystemRole(claims.SystemRole),
		ExpectedVersion: req.ExpectedVersion,
		Title:           req.Title,
		Subtitle:        req.Subtitle,
		Summary:         req.Summary,
		BodyBlocks:      []byte(req.BodyBlocks),
		ArticleType:     req.ArticleType,
		AccessLevel:     req.AccessLevel,
		Category:        req.Category,
		Tags:            req.Tags,
		CourtCaseID:     req.CourtCaseID,
		CourtName:       req.CourtName,
		Citation:        req.Citation,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, article)
}

// SubmitForReview handles POST /v1/cms/articles/{id}/submit
func (h *CMSArticleHandler) SubmitForReview(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	article, err := h.useCases.SubmitForReview(r.Context(), commands.SubmitForReviewCommand{
		ID:        id,
		ActorID:   claims.UserID,
		ActorRole: identity.SystemRole(claims.SystemRole),
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, article)
}

// SendBackPayload carries review feedback.
type SendBackPayload struct {
	Comment string `json:"comment"`
}

// SendBack handles POST /v1/cms/articles/{id}/send-back
func (h *CMSArticleHandler) SendBack(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	var req SendBackPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	article, err := h.useCases.SendBack(r.Context(), commands.SendBackCommand{
		ID:        id,
		ActorRole: identity.SystemRole(claims.SystemRole),
		Comment:   req.Comment,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, article)
}

// Publish handles POST /v1/cms/articles/{id}/publish
func (h *CMSArticleHandler) Publish(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	article, err := h.useCases.PublishArticle(r.Context(), commands.PublishArticleCommand{
		ID:        id,
		ActorID:   claims.UserID,
		ActorRole: identity.SystemRole(claims.SystemRole),
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, article)
}

// Archive handles POST /v1/cms/articles/{id}/archive
func (h *CMSArticleHandler) Archive(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	article, err := h.useCases.ArchiveArticle(r.Context(), commands.ArchiveArticleCommand{
		ID:        id,
		ActorRole: identity.SystemRole(claims.SystemRole),
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, article)
}

// AttachPDFPayload carries attachment confirmation details.
type AttachPDFPayload struct {
	StorageKey string `json:"storage_key"`
	SizeBytes  int64  `json:"size_bytes"`
	Checksum   string `json:"checksum"`
}

// AttachPDF handles POST /v1/cms/articles/{id}/attachment
func (h *CMSArticleHandler) AttachPDF(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	var req AttachPDFPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	article, err := h.useCases.AttachPDF(r.Context(), commands.AttachPDFCommand{
		ArticleID:  id,
		ActorID:    claims.UserID,
		ActorRole:  identity.SystemRole(claims.SystemRole),
		StorageKey: req.StorageKey,
		SizeBytes:  req.SizeBytes,
		Checksum:   req.Checksum,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, article)
}

// CreateJudgmentPayload represents creation of a judgment summary.
type CreateJudgmentPayload struct {
	CourtCaseID   string              `json:"court_case_id"`
	Title         string              `json:"title"`
	Summary       string              `json:"summary"`
	BodyBlocks    json.RawMessage     `json:"body_blocks"`
	AccessLevel   content.AccessLevel `json:"access_level"`
	CourtName     string              `json:"court_name"`
	Citation      *string             `json:"citation,omitempty"`
	PDFStorageKey *string             `json:"pdf_storage_key,omitempty"`
	PDFSizeBytes  *int64              `json:"pdf_size_bytes,omitempty"`
	PDFChecksum   *string             `json:"pdf_checksum,omitempty"`
}

// CreateJudgment handles POST /v1/cms/judgments
func (h *CMSArticleHandler) CreateJudgment(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req CreateJudgmentPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	article, err := h.useCases.CreateJudgmentSummary(r.Context(), commands.CreateJudgmentSummaryCommand{
		ActorID:       claims.UserID,
		ActorRole:     identity.SystemRole(claims.SystemRole),
		CourtCaseID:   req.CourtCaseID,
		Title:         req.Title,
		Summary:       req.Summary,
		BodyBlocks:    []byte(req.BodyBlocks),
		AccessLevel:   req.AccessLevel,
		CourtName:     req.CourtName,
		Citation:      req.Citation,
		PDFStorageKey: req.PDFStorageKey,
		PDFSizeBytes:  req.PDFSizeBytes,
		PDFChecksum:   req.PDFChecksum,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusCreated, article)
}

// RequestUploadURLPayload carries upload intent specifications.
type RequestUploadURLPayload struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Purpose     string `json:"purpose"`
}

// RequestUploadURL handles POST /v1/cms/uploads/presigned-url
func (h *CMSArticleHandler) RequestUploadURL(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req RequestUploadURLPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	res, err := h.useCases.RequestUploadURL(r.Context(), commands.RequestUploadURLCommand{
		ActorRole:   identity.SystemRole(claims.SystemRole),
		Filename:    req.Filename,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
		Purpose:     req.Purpose,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (h *CMSArticleHandler) handleError(w http.ResponseWriter, err error, reqID string) {
	switch {
	case errors.Is(err, ports.ErrArticleNotFound):
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "Article not found", reqID)
	case errors.Is(err, content.ErrOptimisticLockConflict):
		writeJSONError(w, http.StatusConflict, "OPTIMISTIC_LOCK_CONFLICT", "Article was modified by another editor. Please refresh and try again.", reqID)
	case errors.Is(err, ports.ErrSlugAlreadyExists):
		writeJSONError(w, http.StatusConflict, "SLUG_EXISTS", "Article slug already exists", reqID)
	case errors.Is(err, content.ErrUnauthorizedWorkflow):
		writeJSONError(w, http.StatusForbidden, "FORBIDDEN_WORKFLOW", "You are not authorized to perform this workflow transition", reqID)
	case errors.Is(err, content.ErrInvalidStatus):
		writeJSONError(w, http.StatusBadRequest, "INVALID_STATUS", err.Error(), reqID)
	case errors.Is(err, content.ErrValidationFailed), errors.Is(err, content.ErrCommentRequired):
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), reqID)
	case errors.Is(err, commands.ErrInvalidFileType):
		writeJSONError(w, http.StatusBadRequest, "INVALID_FILE_TYPE", err.Error(), reqID)
	case errors.Is(err, commands.ErrFileTooLarge):
		writeJSONError(w, http.StatusBadRequest, "FILE_TOO_LARGE", err.Error(), reqID)
	default:
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
	}
}
