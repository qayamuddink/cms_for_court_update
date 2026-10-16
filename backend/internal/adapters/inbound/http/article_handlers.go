package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

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
	if secSlug := r.URL.Query().Get("section_slug"); secSlug != "" {
		filter.SectionSlug = &secSlug
	}
	if secID := r.URL.Query().Get("section_id"); secID != "" {
		filter.SectionID = &secID
	}
	if topStory := r.URL.Query().Get("is_top_story"); topStory != "" {
		isTop := topStory == "true" || topStory == "1"
		filter.IsTopStory = &isTop
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

// GetBySlug handles GET /v1/articles/{slug} with 301 Permanent Redirect on historical slugs.
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
		var redir queries.ErrSlugRedirected
		if errors.As(err, &redir) {
			w.Header().Set("Location", "/v1/articles/"+redir.NewSlug)
			w.WriteHeader(http.StatusMovedPermanently)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"redirect_url": "/v1/articles/" + redir.NewSlug,
				"old_slug":     redir.OldSlug,
				"new_slug":     redir.NewSlug,
			})
			return
		}

		if errors.Is(err, ports.ErrArticleNotFound) {
			writeJSONError(w, http.StatusNotFound, "ARTICLE_NOT_FOUND", "Article not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve article", reqID)
		return
	}

	if article.AccessLevel == content.AccessLevelPremium {
		w.Header().Set("Cache-Control", "private, no-store")
	}

	writeJSON(w, http.StatusOK, article)
}

// ListTopStories handles GET /v1/top-stories
func (h *ArticlePublicHandler) ListTopStories(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	stories, err := h.queries.GetTopStories(r.Context(), limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve top stories", reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"stories": stories})
}

// GetRelated handles GET /v1/articles/{id}/related
func (h *ArticlePublicHandler) GetRelated(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Article ID is required", reqID)
		return
	}

	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	items, err := h.queries.GetRelatedArticles(r.Context(), id, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve related articles", reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"articles": items})
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
			writeJSONError(w, http.StatusNotFound, "JUDGMENT_NOT_FOUND", "Judgment not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve judgment", reqID)
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

	var userID *string
	if claims := GetAuthClaims(r.Context()); claims != nil {
		userID = &claims.UserID
	}

	judgment, err := h.queries.GetJudgmentByCaseID(r.Context(), caseID, userID)
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
	Title                string              `json:"title"`
	Subtitle             *string             `json:"subtitle,omitempty"`
	Summary              string              `json:"summary"`
	BodyBlocks           json.RawMessage     `json:"body_blocks"`
	ArticleType          content.ArticleType `json:"article_type"`
	AccessLevel          content.AccessLevel `json:"access_level"`
	Category             string              `json:"category"`
	Tags                 []string            `json:"tags"`
	CourtCaseID          *string             `json:"court_case_id,omitempty"`
	CourtName            *string             `json:"court_name,omitempty"`
	Citation             *string             `json:"citation,omitempty"`
	PrimarySectionID     *string             `json:"primary_section_id,omitempty"`
	FeaturedImageKey     *string             `json:"featured_image_key,omitempty"`
	FeaturedImageCaption *string             `json:"featured_image_caption,omitempty"`
	FeaturedImageAlt     *string             `json:"featured_image_alt,omitempty"`
	MetaTitle            *string             `json:"meta_title,omitempty"`
	MetaDescription      *string             `json:"meta_description,omitempty"`
	CanonicalURL         *string             `json:"canonical_url,omitempty"`
	SocialImageKey       *string             `json:"social_image_key,omitempty"`
	IsTopStory           bool                `json:"is_top_story"`
	TopStoryOrder        int                 `json:"top_story_order"`
	RelatedArticleIDs    []string            `json:"related_article_ids,omitempty"`
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
		ActorID:              claims.UserID,
		ActorRole:            identity.SystemRole(claims.SystemRole),
		Title:                req.Title,
		Subtitle:             req.Subtitle,
		Summary:              req.Summary,
		BodyBlocks:           []byte(req.BodyBlocks),
		ArticleType:          req.ArticleType,
		AccessLevel:          req.AccessLevel,
		Category:             req.Category,
		Tags:                 req.Tags,
		CourtCaseID:          req.CourtCaseID,
		CourtName:            req.CourtName,
		Citation:             req.Citation,
		PrimarySectionID:     req.PrimarySectionID,
		FeaturedImageKey:     req.FeaturedImageKey,
		FeaturedImageCaption: req.FeaturedImageCaption,
		FeaturedImageAlt:     req.FeaturedImageAlt,
		MetaTitle:            req.MetaTitle,
		MetaDescription:      req.MetaDescription,
		CanonicalURL:         req.CanonicalURL,
		SocialImageKey:       req.SocialImageKey,
		IsTopStory:           req.IsTopStory,
		TopStoryOrder:        req.TopStoryOrder,
		RelatedArticleIDs:    req.RelatedArticleIDs,
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
	if secID := r.URL.Query().Get("section_id"); secID != "" {
		filter.SectionID = &secID
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
	ExpectedVersion      int                 `json:"expected_version"`
	Title                string              `json:"title"`
	Subtitle             *string             `json:"subtitle,omitempty"`
	Summary              string              `json:"summary"`
	BodyBlocks           json.RawMessage     `json:"body_blocks"`
	ArticleType          content.ArticleType `json:"article_type"`
	AccessLevel          content.AccessLevel `json:"access_level"`
	Category             string              `json:"category"`
	Tags                 []string            `json:"tags"`
	CourtCaseID          *string             `json:"court_case_id,omitempty"`
	CourtName            *string             `json:"court_name,omitempty"`
	Citation             *string             `json:"citation,omitempty"`
	PrimarySectionID     *string             `json:"primary_section_id,omitempty"`
	FeaturedImageKey     *string             `json:"featured_image_key,omitempty"`
	FeaturedImageCaption *string             `json:"featured_image_caption,omitempty"`
	FeaturedImageAlt     *string             `json:"featured_image_alt,omitempty"`
	MetaTitle            *string             `json:"meta_title,omitempty"`
	MetaDescription      *string             `json:"meta_description,omitempty"`
	CanonicalURL         *string             `json:"canonical_url,omitempty"`
	SocialImageKey       *string             `json:"social_image_key,omitempty"`
	IsTopStory           *bool               `json:"is_top_story,omitempty"`
	TopStoryOrder        *int                `json:"top_story_order,omitempty"`
	RelatedArticleIDs    []string            `json:"related_article_ids,omitempty"`
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
		ID:                   id,
		ActorID:              claims.UserID,
		ActorRole:            identity.SystemRole(claims.SystemRole),
		ExpectedVersion:      req.ExpectedVersion,
		Title:                req.Title,
		Subtitle:             req.Subtitle,
		Summary:              req.Summary,
		BodyBlocks:           []byte(req.BodyBlocks),
		ArticleType:          req.ArticleType,
		AccessLevel:          req.AccessLevel,
		Category:             req.Category,
		Tags:                 req.Tags,
		CourtCaseID:          req.CourtCaseID,
		CourtName:            req.CourtName,
		Citation:             req.Citation,
		PrimarySectionID:     req.PrimarySectionID,
		FeaturedImageKey:     req.FeaturedImageKey,
		FeaturedImageCaption: req.FeaturedImageCaption,
		FeaturedImageAlt:     req.FeaturedImageAlt,
		MetaTitle:            req.MetaTitle,
		MetaDescription:      req.MetaDescription,
		CanonicalURL:         req.CanonicalURL,
		SocialImageKey:       req.SocialImageKey,
		IsTopStory:           req.IsTopStory,
		TopStoryOrder:        req.TopStoryOrder,
		RelatedArticleIDs:    req.RelatedArticleIDs,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, article)
}

// SchedulePayload represents JSON body for scheduling an article.
type SchedulePayload struct {
	ScheduledAt string `json:"scheduled_at"`
}

// Schedule handles POST /v1/cms/articles/{id}/schedule
func (h *CMSArticleHandler) Schedule(w http.ResponseWriter, r *http.Request) {
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

	var req SchedulePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	schedTime, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_DATETIME", "scheduled_at must be RFC3339 format", reqID)
		return
	}

	article, err := h.useCases.ScheduleArticle(r.Context(), commands.ScheduleArticleCommand{
		ID:          id,
		ActorRole:   identity.SystemRole(claims.SystemRole),
		ScheduledAt: schedTime,
	})

	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, article)
}

// SoftDelete handles DELETE /v1/cms/articles/{id}
func (h *CMSArticleHandler) SoftDelete(w http.ResponseWriter, r *http.Request) {
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

	if err := h.useCases.SoftDeleteArticle(r.Context(), id, identity.SystemRole(claims.SystemRole)); err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Restore handles POST /v1/cms/articles/{id}/restore
func (h *CMSArticleHandler) Restore(w http.ResponseWriter, r *http.Request) {
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

	if err := h.useCases.RestoreArticle(r.Context(), id, identity.SystemRole(claims.SystemRole)); err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

// ListRevisions handles GET /v1/cms/articles/{id}/revisions
func (h *CMSArticleHandler) ListRevisions(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Article ID is required", reqID)
		return
	}

	revs, err := h.queries.ListRevisions(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve revisions", reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"revisions": revs})
}

// GetRevision handles GET /v1/cms/articles/{id}/revisions/{version}
func (h *CMSArticleHandler) GetRevision(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	verStr := r.PathValue("version")

	ver, err := strconv.Atoi(verStr)
	if err != nil || ver <= 0 {
		writeJSONError(w, http.StatusBadRequest, "INVALID_VERSION", "Valid revision version number is required", reqID)
		return
	}

	rev, err := h.queries.GetRevision(r.Context(), id, ver)
	if err != nil {
		if errors.Is(err, content.ErrRevisionNotFound) {
			writeJSONError(w, http.StatusNotFound, "REVISION_NOT_FOUND", "Revision not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve revision", reqID)
		return
	}

	writeJSON(w, http.StatusOK, rev)
}

// RestoreRevision handles POST /v1/cms/articles/{id}/revisions/{version}/restore
func (h *CMSArticleHandler) RestoreRevision(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	verStr := r.PathValue("version")
	ver, err := strconv.Atoi(verStr)
	if err != nil || ver <= 0 {
		writeJSONError(w, http.StatusBadRequest, "INVALID_VERSION", "Valid revision version number is required", reqID)
		return
	}

	art, err := h.useCases.RestoreRevision(r.Context(), id, ver, claims.UserID, identity.SystemRole(claims.SystemRole))
	if err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, art)
}

// TopStoriesPayload represents JSON body for reordering pinned top stories.
type TopStoriesPayload struct {
	Items []content.TopStoryOrderItem `json:"items"`
}

// UpdateTopStories handles PUT /v1/cms/top-stories
func (h *CMSArticleHandler) UpdateTopStories(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req TopStoriesPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	if err := h.useCases.UpdateTopStories(r.Context(), req.Items, identity.SystemRole(claims.SystemRole)); err != nil {
		h.handleError(w, err, reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
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
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Article ID is required", reqID)
		return
	}

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

// SendBackPayload represents JSON body for editor sending back with comments.
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
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Article ID is required", reqID)
		return
	}

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
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Article ID is required", reqID)
		return
	}

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
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Article ID is required", reqID)
		return
	}

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

// AttachPDFPayload represents the JSON payload to attach official judgment PDF metadata.
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
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Article ID is required", reqID)
		return
	}

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

// CreateJudgmentSummaryPayload represents JSON body for creating a judgment summary linked to a court case.
type CreateJudgmentSummaryPayload struct {
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

// CreateJudgmentSummary handles POST /v1/cms/judgments
func (h *CMSArticleHandler) CreateJudgmentSummary(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req CreateJudgmentSummaryPayload
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

// PresignUploadPayload represents upload request parameters.
type PresignUploadPayload struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Purpose     string `json:"purpose"`
}

// PresignUpload handles POST /v1/cms/uploads/presign
func (h *CMSArticleHandler) PresignUpload(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req PresignUploadPayload
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
		writeJSONError(w, http.StatusNotFound, "ARTICLE_NOT_FOUND", "Article not found", reqID)
	case errors.Is(err, content.ErrInvalidStatus):
		writeJSONError(w, http.StatusBadRequest, "INVALID_STATUS", "Invalid article status for this operation", reqID)
	case errors.Is(err, content.ErrUnauthorizedWorkflow):
		writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Unauthorized action for current role or author", reqID)
	case errors.Is(err, content.ErrValidationFailed):
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), reqID)
	case errors.Is(err, content.ErrOptimisticLockConflict):
		writeJSONError(w, http.StatusConflict, "VERSION_CONFLICT", "Article has been modified by another editor. Please reload and review diff.", reqID)
	case errors.Is(err, content.ErrCommentRequired):
		writeJSONError(w, http.StatusBadRequest, "COMMENT_REQUIRED", "A review comment is required when sending back", reqID)
	case errors.Is(err, ports.ErrSlugAlreadyExists):
		writeJSONError(w, http.StatusConflict, "SLUG_ALREADY_EXISTS", "An article with this title slug already exists", reqID)
	case errors.Is(err, commands.ErrInvalidFileType):
		writeJSONError(w, http.StatusBadRequest, "INVALID_FILE_TYPE", err.Error(), reqID)
	case errors.Is(err, commands.ErrFileTooLarge):
		writeJSONError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", err.Error(), reqID)
	default:
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
	}
}
