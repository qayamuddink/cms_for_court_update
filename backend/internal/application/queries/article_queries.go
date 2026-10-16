package queries

import (
	"context"
	"errors"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
)

// ArticleQueries handles read queries for articles, feed, and judgments.
type ArticleQueries struct {
	articleRepo ports.ArticleRepository
	entitlement ports.EntitlementChecker
	fileStore   ports.FileStore
}

// NewArticleQueries creates a new ArticleQueries instance.
func NewArticleQueries(
	articleRepo ports.ArticleRepository,
	entitlement ports.EntitlementChecker,
	fileStore ports.FileStore,
) *ArticleQueries {
	return &ArticleQueries{
		articleRepo: articleRepo,
		entitlement: entitlement,
		fileStore:   fileStore,
	}
}

// ArticleFeedItem represents a summary card in the public news feed.
type ArticleFeedItem struct {
	ID                 string              `json:"id"`
	Slug               string              `json:"slug"`
	Title              string              `json:"title"`
	Subtitle           *string             `json:"subtitle,omitempty"`
	Summary            string              `json:"summary"`
	ArticleType        content.ArticleType `json:"article_type"`
	AccessLevel        content.AccessLevel `json:"access_level"`
	Category           string              `json:"category"`
	Tags               []string            `json:"tags"`
	CourtName          *string             `json:"court_name,omitempty"`
	Citation           *string             `json:"citation,omitempty"`
	HasAttachment      bool                `json:"has_attachment"`
	ReadingTimeMinutes int                 `json:"reading_time_minutes"`
	PublishedAt        *time.Time          `json:"published_at,omitempty"`
}

// FeedResponse represents keyset-paginated articles feed.
type FeedResponse struct {
	Articles   []ArticleFeedItem `json:"articles"`
	NextCursor *string           `json:"next_cursor,omitempty"`
}

// GetFeed retrieves published articles for the reader feed using keyset pagination.
func (q *ArticleQueries) GetFeed(ctx context.Context, filter ports.ArticleFilter) (*FeedResponse, error) {
	articles, nextCursor, err := q.articleRepo.ListFeed(ctx, filter)
	if err != nil {
		return nil, err
	}

	items := make([]ArticleFeedItem, len(articles))
	for i, a := range articles {
		hasPDF := a.PDFStorageKey != nil && *a.PDFStorageKey != ""
		items[i] = ArticleFeedItem{
			ID:                 a.ID,
			Slug:               a.Slug,
			Title:              a.Title,
			Subtitle:           a.Subtitle,
			Summary:            a.Summary,
			ArticleType:        a.ArticleType,
			AccessLevel:        a.AccessLevel,
			Category:           a.Category,
			Tags:               a.Tags,
			CourtName:          a.CourtName,
			Citation:           a.Citation,
			HasAttachment:      hasPDF,
			ReadingTimeMinutes: content.EstimateReadingTime(a.Summary, a.BodyBlocks),
			PublishedAt:        a.PublishedAt,
		}
	}

	return &FeedResponse{
		Articles:   items,
		NextCursor: nextCursor,
	}, nil
}

// ArticleDetailResponse represents a full article view, applying paywall locking when appropriate.
type ArticleDetailResponse struct {
	ID                 string              `json:"id"`
	Slug               string              `json:"slug"`
	Title              string              `json:"title"`
	Subtitle           *string             `json:"subtitle,omitempty"`
	Summary            string              `json:"summary"`
	BodyBlocks         any                 `json:"body_blocks,omitempty"`
	ArticleType        content.ArticleType `json:"article_type"`
	AccessLevel        content.AccessLevel `json:"access_level"`
	Status             content.ArticleStatus `json:"status"`
	AuthorID           string              `json:"author_id"`
	Category           string              `json:"category"`
	Tags               []string            `json:"tags"`
	CourtName          *string             `json:"court_name,omitempty"`
	Citation           *string             `json:"citation,omitempty"`
	CourtCaseID        *string             `json:"court_case_id,omitempty"`
	PDFStorageKey      *string             `json:"pdf_storage_key,omitempty"`
	PDFDownloadURL     *string             `json:"pdf_download_url,omitempty"`
	PDFSizeBytes       *int64              `json:"pdf_size_bytes,omitempty"`
	IsLocked           bool                `json:"is_locked"`
	ReadingTimeMinutes int                 `json:"reading_time_minutes"`
	PublishedAt        *time.Time          `json:"published_at,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

// GetArticleBySlug retrieves an article by its slug and enforces paywall entitlement logic.
func (q *ArticleQueries) GetArticleBySlug(ctx context.Context, slug string, userID *string) (*ArticleDetailResponse, error) {
	a, err := q.articleRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Only published articles are visible through the public slug query
	if a.Status != content.StatusPublished {
		return nil, ports.ErrArticleNotFound
	}

	hasAccess := true
	if q.entitlement != nil {
		allowed, err := q.entitlement.HasArticleAccess(ctx, userID, a)
		if err != nil {
			return nil, err
		}
		hasAccess = allowed
	}

	resp := &ArticleDetailResponse{
		ID:                 a.ID,
		Slug:               a.Slug,
		Title:              a.Title,
		Subtitle:           a.Subtitle,
		Summary:            a.Summary,
		ArticleType:        a.ArticleType,
		AccessLevel:        a.AccessLevel,
		Status:             a.Status,
		AuthorID:           a.AuthorID,
		Category:           a.Category,
		Tags:               a.Tags,
		CourtName:          a.CourtName,
		Citation:           a.Citation,
		CourtCaseID:        a.CourtCaseID,
		PDFSizeBytes:       a.PDFSizeBytes,
		ReadingTimeMinutes: content.EstimateReadingTime(a.Summary, a.BodyBlocks),
		PublishedAt:        a.PublishedAt,
		CreatedAt:          a.CreatedAt,
		UpdatedAt:          a.UpdatedAt,
	}

	if hasAccess {
		resp.IsLocked = false
		resp.BodyBlocks = a.BodyBlocks
		if a.PDFStorageKey != nil && *a.PDFStorageKey != "" && q.fileStore != nil {
			downloadURL, err := q.fileStore.PresignedGetURL(ctx, *a.PDFStorageKey, 15*time.Minute)
			if err == nil {
				resp.PDFDownloadURL = &downloadURL
			}
		}
	} else {
		// Premium locked: body is omitted
		resp.IsLocked = true
		resp.BodyBlocks = nil
	}

	return resp, nil
}

// JudgmentDetailResponse represents a judgment summary with official PDF download link.
type JudgmentDetailResponse struct {
	ID             string              `json:"id"`
	Slug           string              `json:"slug"`
	Title          string              `json:"title"`
	Summary        string              `json:"summary"`
	BodyBlocks     any                 `json:"body_blocks,omitempty"`
	AccessLevel    content.AccessLevel `json:"access_level"`
	CourtCaseID    *string             `json:"court_case_id,omitempty"`
	CourtName      *string             `json:"court_name,omitempty"`
	Citation       *string             `json:"citation,omitempty"`
	PDFStorageKey  *string             `json:"pdf_storage_key,omitempty"`
	PDFDownloadURL *string             `json:"pdf_download_url,omitempty"`
	PDFSizeBytes   *int64              `json:"pdf_size_bytes,omitempty"`
	PublishedAt    *time.Time          `json:"published_at,omitempty"`
}

// GetJudgment retrieves a judgment summary by article ID and provides a signed PDF download link.
func (q *ArticleQueries) GetJudgment(ctx context.Context, id string) (*JudgmentDetailResponse, error) {
	a, err := q.articleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if a.ArticleType != content.ArticleTypeJudgmentSummary {
		return nil, errors.New("article is not a judgment summary")
	}

	resp := &JudgmentDetailResponse{
		ID:            a.ID,
		Slug:          a.Slug,
		Title:         a.Title,
		Summary:       a.Summary,
		BodyBlocks:    a.BodyBlocks,
		AccessLevel:   a.AccessLevel,
		CourtCaseID:   a.CourtCaseID,
		CourtName:     a.CourtName,
		Citation:      a.Citation,
		PDFStorageKey: a.PDFStorageKey,
		PDFSizeBytes:  a.PDFSizeBytes,
		PublishedAt:   a.PublishedAt,
	}

	if a.PDFStorageKey != nil && *a.PDFStorageKey != "" && q.fileStore != nil {
		downloadURL, err := q.fileStore.PresignedGetURL(ctx, *a.PDFStorageKey, 15*time.Minute)
		if err == nil {
			resp.PDFDownloadURL = &downloadURL
		}
	}

	return resp, nil
}

// GetJudgmentByCaseID retrieves the judgment summary linked to a court case.
func (q *ArticleQueries) GetJudgmentByCaseID(ctx context.Context, caseID string) (*JudgmentDetailResponse, error) {
	a, err := q.articleRepo.GetJudgmentByCaseID(ctx, caseID)
	if err != nil {
		return nil, err
	}

	resp := &JudgmentDetailResponse{
		ID:            a.ID,
		Slug:          a.Slug,
		Title:         a.Title,
		Summary:       a.Summary,
		BodyBlocks:    a.BodyBlocks,
		AccessLevel:   a.AccessLevel,
		CourtCaseID:   a.CourtCaseID,
		CourtName:     a.CourtName,
		Citation:      a.Citation,
		PDFStorageKey: a.PDFStorageKey,
		PDFSizeBytes:  a.PDFSizeBytes,
		PublishedAt:   a.PublishedAt,
	}

	if a.PDFStorageKey != nil && *a.PDFStorageKey != "" && q.fileStore != nil {
		downloadURL, err := q.fileStore.PresignedGetURL(ctx, *a.PDFStorageKey, 15*time.Minute)
		if err == nil {
			resp.PDFDownloadURL = &downloadURL
		}
	}

	return resp, nil
}

// CMSArticleListResponse represents CMS articles listing with total count.
type CMSArticleListResponse struct {
	Articles []content.Article `json:"articles"`
	Total    int               `json:"total"`
}

// ListCMSArticles queries articles for the CMS editorial interface.
func (q *ArticleQueries) ListCMSArticles(ctx context.Context, filter ports.ArticleFilter) (*CMSArticleListResponse, error) {
	articles, total, err := q.articleRepo.ListCMS(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &CMSArticleListResponse{
		Articles: articles,
		Total:    total,
	}, nil
}

// GetCMSArticleByID retrieves a single article with full editorial fields for the CMS editor.
func (q *ArticleQueries) GetCMSArticleByID(ctx context.Context, id string) (*content.Article, error) {
	return q.articleRepo.GetByID(ctx, id)
}
