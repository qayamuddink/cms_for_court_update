package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/content"
	"github.com/livelaw/backend/internal/domain/identity"
	"github.com/livelaw/backend/internal/domain/judgment"
)

// JudgmentHandler provides public and CMS HTTP handlers for judgments, citations, and PDF downloads.
type JudgmentHandler struct {
	useCases        *commands.JudgmentUseCases
	queries         *queries.JudgmentQueryUseCases
	articleUseCases *commands.ArticleUseCases
	articleQueries  *queries.ArticleQueries
}

// NewJudgmentHandler creates a new JudgmentHandler.
func NewJudgmentHandler(
	useCases *commands.JudgmentUseCases,
	queries *queries.JudgmentQueryUseCases,
	articleUseCases *commands.ArticleUseCases,
	articleQueries *queries.ArticleQueries,
) *JudgmentHandler {
	return &JudgmentHandler{
		useCases:        useCases,
		queries:         queries,
		articleUseCases: articleUseCases,
		articleQueries:  articleQueries,
	}
}

// CreateJudgmentRequest matches the JSON payload for creating or updating a judgment.
type CreateJudgmentRequest struct {
	CaseTitle    string                  `json:"case_title"`
	CaseNumber   string                  `json:"case_number"`
	Court        string                  `json:"court"`
	CourtCode    string                  `json:"court_code"`
	Bench        []string                `json:"bench"`
	JudgmentDate string                  `json:"judgment_date"`
	DecisionDate *string                 `json:"decision_date,omitempty"`
	Appellant    string                  `json:"appellant"`
	Respondent   string                  `json:"respondent"`
	Advocates    []string                `json:"advocates"`
	SubjectTags  []string                `json:"subject_tags"`
	Issues       string                  `json:"issues"`
	Held         string                  `json:"held"`
	Headnote     string                  `json:"headnote"`
	Summary      string                  `json:"summary"`
	KeyTakeaways []string                `json:"key_takeaways"`
	Statutes     []commands.StatuteInput `json:"statutes"`
	CourtCaseID  *string                 `json:"court_case_id,omitempty"`
	ArticleID    *string                 `json:"article_id,omitempty"`
	AccessLevel  string                  `json:"access_level"`

	// Optional fields for legacy article-based judgment summary
	Title         string `json:"title,omitempty"`
	CourtName     string `json:"court_name,omitempty"`
	Citation      string `json:"citation,omitempty"`
	PDFStorageKey string `json:"pdf_storage_key,omitempty"`
	PDFSizeBytes  int64  `json:"pdf_size_bytes,omitempty"`
	PDFChecksum   string `json:"pdf_checksum,omitempty"`
	BodyBlocks    string `json:"body_blocks,omitempty"`
}

// PublishJudgmentRequest holds optional publish options like custom brand prefix.
type PublishJudgmentRequest struct {
	BrandPrefix string `json:"brand_prefix,omitempty"`
}

// ListJudgments handles GET /v1/judgments.
func (h *JudgmentHandler) ListJudgments(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	q := r.URL.Query()

	limit := 20
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := q.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	pubStatus := judgment.StatusPublished
	filter := ports.JudgmentFilter{
		Status: &pubStatus,
		Limit:  limit,
		Offset: offset,
	}

	if cc := q.Get("court_code"); cc != "" {
		filter.CourtCode = &cc
	}
	if yr := q.Get("year"); yr != "" {
		if y, err := strconv.Atoi(yr); err == nil {
			filter.Year = &y
		}
	}
	if sq := q.Get("search"); sq != "" {
		filter.SearchQuery = &sq
	}
	if sn := q.Get("statute"); sn != "" {
		filter.StatuteName = &sn
	}
	if cid := q.Get("case_id"); cid != "" {
		filter.CaseID = &cid
	}

	judgments, total, err := h.queries.List(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve judgments", reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"judgments": judgments,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// GetJudgment handles GET /v1/judgments/{id}.
func (h *JudgmentHandler) GetJudgment(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Judgment ID is required", reqID)
		return
	}

	detail, err := h.queries.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, queries.ErrJudgmentNotFound) && h.articleQueries != nil {
			artDetail, artErr := h.articleQueries.GetJudgment(r.Context(), id)
			if artErr == nil && artDetail != nil {
				writeJSON(w, http.StatusOK, artDetail)
				return
			}
		}
		if errors.Is(err, queries.ErrJudgmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "Judgment not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve judgment", reqID)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

// GetJudgmentByCitation handles GET /v1/judgments/citation/{citation...}.
func (h *JudgmentHandler) GetJudgmentByCitation(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	citation := r.PathValue("citation")
	if unescaped, err := url.PathUnescape(citation); err == nil && unescaped != "" {
		citation = unescaped
	}
	if citation == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Citation is required", reqID)
		return
	}

	detail, err := h.queries.GetByCitation(r.Context(), citation)
	if err != nil {
		if errors.Is(err, queries.ErrJudgmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "Judgment not found for citation: "+citation, reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve judgment", reqID)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

// GetJudgmentPages handles GET /v1/judgments/{id}/pages.
func (h *JudgmentHandler) GetJudgmentPages(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Judgment ID is required", reqID)
		return
	}

	pages, err := h.queries.GetPages(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve judgment pages", reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pages": pages,
	})
}

// GetDownloadURL handles GET /v1/judgments/{id}/download.
func (h *JudgmentHandler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Judgment ID is required", reqID)
		return
	}

	var userID *string
	if claims := GetAuthClaims(r.Context()); claims != nil && claims.UserID != "" {
		userID = &claims.UserID
	}

	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	userAgent := r.UserAgent()

	resp, err := h.queries.GetDownloadURL(r.Context(), queries.DownloadRequest{
		JudgmentID: id,
		UserID:     userID,
		IPAddress:  ip,
		UserAgent:  userAgent,
	})
	if err != nil {
		if errors.Is(err, queries.ErrJudgmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "Judgment not found", reqID)
			return
		}
		if errors.Is(err, queries.ErrPDFNotFound) {
			writeJSONError(w, http.StatusNotFound, "PDF_NOT_FOUND", "No PDF is attached to this judgment", reqID)
			return
		}
		if errors.Is(err, queries.ErrSubscriptionRequired) {
			writeJSONError(w, http.StatusForbidden, "SUBSCRIPTION_REQUIRED", "An active subscription is required to download premium judgments", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate download URL", reqID)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- CMS Endpoints ---

// CreateDraft handles POST /v1/cms/judgments.
func (h *JudgmentHandler) CreateDraft(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req CreateJudgmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", reqID)
		return
	}

	// Support Phase 8 article-based judgment summary payload
	if req.CaseTitle == "" && req.Title != "" && h.articleUseCases != nil {
		accessLvl := content.AccessLevelFree
		if req.AccessLevel == "premium" {
			accessLvl = content.AccessLevelPremium
		}
		caseID := ""
		if req.CourtCaseID != nil {
			caseID = *req.CourtCaseID
		}
		var pCitation, pPDFStorageKey, pPDFChecksum *string
		if req.Citation != "" {
			pCitation = &req.Citation
		}
		if req.PDFStorageKey != "" {
			pPDFStorageKey = &req.PDFStorageKey
		}
		if req.PDFChecksum != "" {
			pPDFChecksum = &req.PDFChecksum
		}
		var pPDFSizeBytes *int64
		if req.PDFSizeBytes > 0 {
			pPDFSizeBytes = &req.PDFSizeBytes
		}

		article, err := h.articleUseCases.CreateJudgmentSummary(r.Context(), commands.CreateJudgmentSummaryCommand{
			ActorID:       claims.UserID,
			ActorRole:     identity.SystemRole(claims.SystemRole),
			CourtCaseID:   caseID,
			Title:         req.Title,
			Summary:       req.Summary,
			BodyBlocks:    []byte(req.BodyBlocks),
			AccessLevel:   accessLvl,
			CourtName:     req.CourtName,
			Citation:      pCitation,
			PDFStorageKey: pPDFStorageKey,
			PDFSizeBytes:  pPDFSizeBytes,
			PDFChecksum:   pPDFChecksum,
		})
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), reqID)
			return
		}
		writeJSON(w, http.StatusCreated, article)
		return
	}

	jDate, err := parseDate(req.JudgmentDate)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid judgment_date format: must be YYYY-MM-DD", reqID)
		return
	}

	var dDate *time.Time
	if req.DecisionDate != nil && *req.DecisionDate != "" {
		if parsed, err := parseDate(*req.DecisionDate); err == nil {
			dDate = &parsed
		}
	}

	cmd := commands.CreateJudgmentCommand{
		ActorID:      claims.UserID,
		ActorRole:    identity.SystemRole(claims.SystemRole),
		CaseTitle:    req.CaseTitle,
		CaseNumber:   req.CaseNumber,
		Court:        req.Court,
		CourtCode:    req.CourtCode,
		Bench:        req.Bench,
		JudgmentDate: jDate,
		DecisionDate: dDate,
		Appellant:    req.Appellant,
		Respondent:   req.Respondent,
		Advocates:    req.Advocates,
		SubjectTags:  req.SubjectTags,
		Issues:       req.Issues,
		Held:         req.Held,
		Headnote:     req.Headnote,
		Summary:      req.Summary,
		KeyTakeaways: req.KeyTakeaways,
		Statutes:     req.Statutes,
		CourtCaseID:  req.CourtCaseID,
		ArticleID:    req.ArticleID,
		AccessLevel:  judgment.AccessLevel(req.AccessLevel),
	}

	j, err := h.useCases.CreateDraft(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, commands.ErrUnauthorizedAction) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error(), reqID)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"judgment": j,
	})
}

// ListCMS handles GET /v1/cms/judgments.
func (h *JudgmentHandler) ListCMS(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	q := r.URL.Query()

	limit := 50
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := q.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	filter := ports.JudgmentFilter{
		Limit:  limit,
		Offset: offset,
	}

	if st := q.Get("status"); st != "" {
		js := judgment.JudgmentStatus(st)
		filter.Status = &js
	}
	if cc := q.Get("court_code"); cc != "" {
		filter.CourtCode = &cc
	}
	if sq := q.Get("search"); sq != "" {
		filter.SearchQuery = &sq
	}

	judgments, total, err := h.queries.List(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve judgments", reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"judgments": judgments,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// GetCMS handles GET /v1/cms/judgments/{id}.
func (h *JudgmentHandler) GetCMS(w http.ResponseWriter, r *http.Request) {
	h.GetJudgment(w, r)
}

// Update handles PUT /v1/cms/judgments/{id}.
func (h *JudgmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Judgment ID is required", reqID)
		return
	}

	var req CreateJudgmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", reqID)
		return
	}

	jDate, err := parseDate(req.JudgmentDate)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid judgment_date format", reqID)
		return
	}

	var dDate *time.Time
	if req.DecisionDate != nil && *req.DecisionDate != "" {
		if parsed, err := parseDate(*req.DecisionDate); err == nil {
			dDate = &parsed
		}
	}

	cmd := commands.UpdateJudgmentCommand{
		ActorID:      claims.UserID,
		ActorRole:    identity.SystemRole(claims.SystemRole),
		JudgmentID:   id,
		CaseTitle:    req.CaseTitle,
		CaseNumber:   req.CaseNumber,
		Court:        req.Court,
		CourtCode:    req.CourtCode,
		Bench:        req.Bench,
		JudgmentDate: jDate,
		DecisionDate: dDate,
		Appellant:    req.Appellant,
		Respondent:   req.Respondent,
		Advocates:    req.Advocates,
		SubjectTags:  req.SubjectTags,
		Issues:       req.Issues,
		Held:         req.Held,
		Headnote:     req.Headnote,
		Summary:      req.Summary,
		KeyTakeaways: req.KeyTakeaways,
		Statutes:     req.Statutes,
		CourtCaseID:  req.CourtCaseID,
		ArticleID:    req.ArticleID,
		AccessLevel:  judgment.AccessLevel(req.AccessLevel),
	}

	j, err := h.useCases.Update(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, commands.ErrUnauthorizedAction) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error(), reqID)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"judgment": j,
	})
}

// AttachPDF handles POST /v1/cms/judgments/{id}/pdf.
func (h *JudgmentHandler) AttachPDF(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Judgment ID is required", reqID)
		return
	}

	var filename string
	var fileData []byte

	// Check if multipart form or raw binary
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// 105 MB max memory for multipart
		if err := r.ParseMultipartForm(105 * 1024 * 1024); err != nil {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Failed to parse multipart form", reqID)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "No file found in form under 'file'", reqID)
			return
		}
		defer file.Close()

		filename = header.Filename
		data, err := io.ReadAll(file)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read uploaded file", reqID)
			return
		}
		fileData = data
	} else {
		// Raw binary upload
		filename = r.URL.Query().Get("filename")
		if filename == "" {
			filename = "judgment.pdf"
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read upload body", reqID)
			return
		}
		fileData = data
	}

	if len(fileData) == 0 {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Uploaded file cannot be empty", reqID)
		return
	}

	cmd := commands.AttachJudgmentPDFCommand{
		ActorID:    claims.UserID,
		ActorRole:  identity.SystemRole(claims.SystemRole),
		JudgmentID: id,
		Filename:   filename,
		Data:       fileData,
		Size:       int64(len(fileData)),
	}

	j, err := h.useCases.AttachPDF(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, commands.ErrUnauthorizedAction) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error(), reqID)
			return
		}
		if errors.Is(err, judgment.ErrInvalidPDFMagicBytes) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_PDF", "File is not a valid PDF document (missing %PDF- magic bytes)", reqID)
			return
		}
		if errors.Is(err, judgment.ErrPDFTooLarge) {
			writeJSONError(w, http.StatusBadRequest, "PAYLOAD_TOO_LARGE", err.Error(), reqID)
			return
		}
		if errors.Is(err, commands.ErrDuplicatePDF) {
			writeJSONError(w, http.StatusConflict, "DUPLICATE_PDF", err.Error(), reqID)
			return
		}
		if errors.Is(err, commands.ErrInfectedFile) {
			writeJSONError(w, http.StatusUnprocessableEntity, "INFECTED_FILE", err.Error(), reqID)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "UPLOAD_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"judgment": j,
	})
}

// Publish handles POST /v1/cms/judgments/{id}/publish.
func (h *JudgmentHandler) Publish(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Judgment ID is required", reqID)
		return
	}

	var req PublishJudgmentRequest
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	cmd := commands.PublishJudgmentCommand{
		ActorID:     claims.UserID,
		ActorRole:   identity.SystemRole(claims.SystemRole),
		JudgmentID:  id,
		BrandPrefix: req.BrandPrefix,
	}

	j, err := h.useCases.Publish(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, commands.ErrUnauthorizedAction) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error(), reqID)
			return
		}
		if errors.Is(err, judgment.ErrNoPDFAttached) {
			writeJSONError(w, http.StatusBadRequest, "MISSING_PDF", "An official PDF must be attached before publishing", reqID)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "PUBLISH_ERROR", err.Error(), reqID)
		return
	}

	citation := ""
	if j.Citation != nil {
		citation = *j.Citation
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"judgment": j,
		"citation": citation,
	})
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid date format: %s", s)
}
