package queries

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
)

// ErrSlugRedirected is returned when an old slug has been permanently moved to a new slug.
type ErrSlugRedirected struct {
	OldSlug string
	NewSlug string
}

func (e ErrSlugRedirected) Error() string {
	return fmt.Sprintf("slug %s moved permanently to %s", e.OldSlug, e.NewSlug)
}

// ArticleQueries handles read queries for articles, feed, revisions, and judgments.
type ArticleQueries struct {
	articleRepo ports.ArticleRepository
	sectionRepo ports.SectionRepository
	entitlement ports.EntitlementChecker
	fileStore   ports.FileStore
}

// NewArticleQueries creates a new ArticleQueries instance.
func NewArticleQueries(
	articleRepo ports.ArticleRepository,
	sectionRepo ports.SectionRepository,
	entitlement ports.EntitlementChecker,
	fileStore ports.FileStore,
) *ArticleQueries {
	return &ArticleQueries{
		articleRepo: articleRepo,
		sectionRepo: sectionRepo,
		entitlement: entitlement,
		fileStore:   fileStore,
	}
}

// ArticleFeedItem represents a summary card in the public news feed or section feed.
type ArticleFeedItem struct {
	ID                   string              `json:"id"`
	Slug                 string              `json:"slug"`
	Title                string              `json:"title"`
	Subtitle             *string             `json:"subtitle,omitempty"`
	Summary              string              `json:"summary"`
	ArticleType          content.ArticleType `json:"article_type"`
	AccessLevel          content.AccessLevel `json:"access_level"`
	Category             string              `json:"category"`
	Tags                 []string            `json:"tags"`
	CourtName            *string             `json:"court_name,omitempty"`
	Citation             *string             `json:"citation,omitempty"`
	HasAttachment        bool                `json:"has_attachment"`
	ReadingTimeMinutes   int                 `json:"reading_time_minutes"`
	WordCount            int                 `json:"word_count"`
	PrimarySectionID     *string             `json:"primary_section_id,omitempty"`
	FeaturedImageKey     *string             `json:"featured_image_key,omitempty"`
	FeaturedImageCaption *string             `json:"featured_image_caption,omitempty"`
	IsTopStory           bool                `json:"is_top_story"`
	TopStoryOrder        int                 `json:"top_story_order"`
	PublishedAt          *time.Time          `json:"published_at,omitempty"`
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
			ID:                   a.ID,
			Slug:                 a.Slug,
			Title:                a.Title,
			Subtitle:             a.Subtitle,
			Summary:              a.Summary,
			ArticleType:          a.ArticleType,
			AccessLevel:          a.AccessLevel,
			Category:             a.Category,
			Tags:                 a.Tags,
			CourtName:            a.CourtName,
			Citation:             a.Citation,
			HasAttachment:        hasPDF,
			ReadingTimeMinutes:   a.ReadingTimeMinutes,
			WordCount:            a.WordCount,
			PrimarySectionID:     a.PrimarySectionID,
			FeaturedImageKey:     a.FeaturedImageKey,
			FeaturedImageCaption: a.FeaturedImageCaption,
			IsTopStory:           a.IsTopStory,
			TopStoryOrder:        a.TopStoryOrder,
			PublishedAt:          a.PublishedAt,
		}
	}

	return &FeedResponse{
		Articles:   items,
		NextCursor: nextCursor,
	}, nil
}

// ArticleDetailResponse represents a full article view with outline, SEO fields, and related items.
type ArticleDetailResponse struct {
	ID                   string                `json:"id"`
	Slug                 string                `json:"slug"`
	Title                string                `json:"title"`
	Subtitle             *string               `json:"subtitle,omitempty"`
	Summary              string                `json:"summary"`
	BodyBlocks           any                   `json:"body_blocks,omitempty"`
	ArticleType          content.ArticleType   `json:"article_type"`
	AccessLevel          content.AccessLevel   `json:"access_level"`
	Status               content.ArticleStatus `json:"status"`
	AuthorID             string                `json:"author_id"`
	Category             string                `json:"category"`
	Tags                 []string              `json:"tags"`
	CourtName            *string               `json:"court_name,omitempty"`
	Citation             *string               `json:"citation,omitempty"`
	CourtCaseID          *string               `json:"court_case_id,omitempty"`
	PDFStorageKey        *string               `json:"pdf_storage_key,omitempty"`
	PDFDownloadURL       *string               `json:"pdf_download_url,omitempty"`
	PDFSizeBytes         *int64                `json:"pdf_size_bytes,omitempty"`
	IsLocked             bool                  `json:"is_locked"`
	ReadingTimeMinutes   int                   `json:"reading_time_minutes"`
	WordCount            int                   `json:"word_count"`
	PrimarySectionID     *string               `json:"primary_section_id,omitempty"`
	PrimarySection       *content.Section      `json:"primary_section,omitempty"`
	FeaturedImageKey     *string               `json:"featured_image_key,omitempty"`
	FeaturedImageCaption *string               `json:"featured_image_caption,omitempty"`
	FeaturedImageAlt     *string               `json:"featured_image_alt,omitempty"`
	MetaTitle            *string               `json:"meta_title,omitempty"`
	MetaDescription      *string               `json:"meta_description,omitempty"`
	CanonicalURL         *string               `json:"canonical_url,omitempty"`
	SocialImageKey       *string               `json:"social_image_key,omitempty"`
	IsTopStory           bool                  `json:"is_top_story"`
	TopStoryOrder        int                   `json:"top_story_order"`
	ScheduledAt          *time.Time            `json:"scheduled_at,omitempty"`
	Outline              []content.OutlineItem `json:"outline"`
	RelatedArticles      []ArticleFeedItem     `json:"related_articles"`
	PublishedAt          *time.Time            `json:"published_at,omitempty"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
}

// GetArticleBySlug retrieves an article by its slug, handles 301 redirects, and enforces paywall entitlement logic.
func (q *ArticleQueries) GetArticleBySlug(ctx context.Context, slug string, userID *string) (*ArticleDetailResponse, error) {
	a, err := q.articleRepo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, ports.ErrArticleNotFound) {
			// Check if slug redirect exists
			redir, redirErr := q.articleRepo.GetSlugRedirect(ctx, slug)
			if redirErr == nil && redir != nil {
				targetArt, targetErr := q.articleRepo.GetByID(ctx, redir.ArticleID)
				if targetErr == nil && targetArt != nil && targetArt.Slug != "" {
					return nil, ErrSlugRedirected{
						OldSlug: slug,
						NewSlug: targetArt.Slug,
					}
				}
			}
		}
		return nil, err
	}

	// Only published non-deleted articles are visible through public slug query
	if a.Status != content.StatusPublished || a.DeletedAt != nil {
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

	// Extract outline from headings in body
	outline := content.ExtractOutline(a.BodyBlocks)
	if outline == nil {
		outline = []content.OutlineItem{}
	}

	// Enrich section if available
	var primarySection *content.Section
	if a.PrimarySectionID != nil && *a.PrimarySectionID != "" && q.sectionRepo != nil {
		sec, secErr := q.sectionRepo.GetByID(ctx, *a.PrimarySectionID)
		if secErr == nil {
			primarySection = sec
		}
	}

	// Fetch related articles
	var relatedItems []ArticleFeedItem
	relList, relErr := q.articleRepo.GetRelatedArticles(ctx, a.ID, 4)
	if relErr == nil && len(relList) > 0 {
		relatedItems = make([]ArticleFeedItem, len(relList))
		for i, item := range relList {
			hasPDF := item.PDFStorageKey != nil && *item.PDFStorageKey != ""
			relatedItems[i] = ArticleFeedItem{
				ID:                   item.ID,
				Slug:                 item.Slug,
				Title:                item.Title,
				Subtitle:             item.Subtitle,
				Summary:              item.Summary,
				ArticleType:          item.ArticleType,
				AccessLevel:          item.AccessLevel,
				Category:             item.Category,
				Tags:                 item.Tags,
				CourtName:            item.CourtName,
				Citation:             item.Citation,
				HasAttachment:        hasPDF,
				ReadingTimeMinutes:   item.ReadingTimeMinutes,
				WordCount:            item.WordCount,
				PrimarySectionID:     item.PrimarySectionID,
				FeaturedImageKey:     item.FeaturedImageKey,
				FeaturedImageCaption: item.FeaturedImageCaption,
				IsTopStory:           item.IsTopStory,
				TopStoryOrder:        item.TopStoryOrder,
				PublishedAt:          item.PublishedAt,
			}
		}
	} else {
		relatedItems = []ArticleFeedItem{}
	}

	readingTime := a.ReadingTimeMinutes
	if readingTime <= 0 {
		readingTime = content.EstimateReadingTime(a.Summary, a.BodyBlocks)
	}

	resp := &ArticleDetailResponse{
		ID:                   a.ID,
		Slug:                 a.Slug,
		Title:                a.Title,
		Subtitle:             a.Subtitle,
		Summary:              a.Summary,
		ArticleType:          a.ArticleType,
		AccessLevel:          a.AccessLevel,
		Status:               a.Status,
		AuthorID:             a.AuthorID,
		Category:             a.Category,
		Tags:                 a.Tags,
		CourtName:            a.CourtName,
		Citation:             a.Citation,
		CourtCaseID:          a.CourtCaseID,
		PDFSizeBytes:         a.PDFSizeBytes,
		ReadingTimeMinutes:   readingTime,
		WordCount:            a.WordCount,
		PrimarySectionID:     a.PrimarySectionID,
		PrimarySection:       primarySection,
		FeaturedImageKey:     a.FeaturedImageKey,
		FeaturedImageCaption: a.FeaturedImageCaption,
		FeaturedImageAlt:     a.FeaturedImageAlt,
		MetaTitle:            a.MetaTitle,
		MetaDescription:      a.MetaDescription,
		CanonicalURL:         a.CanonicalURL,
		SocialImageKey:       a.SocialImageKey,
		IsTopStory:           a.IsTopStory,
		TopStoryOrder:        a.TopStoryOrder,
		ScheduledAt:          a.ScheduledAt,
		Outline:              outline,
		RelatedArticles:      relatedItems,
		PublishedAt:          a.PublishedAt,
		CreatedAt:            a.CreatedAt,
		UpdatedAt:            a.UpdatedAt,
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
		resp.IsLocked = true
		resp.BodyBlocks = nil
	}

	return resp, nil
}

// GetTopStories retrieves ordered pinned top stories.
func (q *ArticleQueries) GetTopStories(ctx context.Context, limit int) ([]ArticleFeedItem, error) {
	stories, err := q.articleRepo.ListTopStories(ctx, limit)
	if err != nil {
		return nil, err
	}

	items := make([]ArticleFeedItem, len(stories))
	for i, a := range stories {
		hasPDF := a.PDFStorageKey != nil && *a.PDFStorageKey != ""
		items[i] = ArticleFeedItem{
			ID:                   a.ID,
			Slug:                 a.Slug,
			Title:                a.Title,
			Subtitle:             a.Subtitle,
			Summary:              a.Summary,
			ArticleType:          a.ArticleType,
			AccessLevel:          a.AccessLevel,
			Category:             a.Category,
			Tags:                 a.Tags,
			CourtName:            a.CourtName,
			Citation:             a.Citation,
			HasAttachment:        hasPDF,
			ReadingTimeMinutes:   a.ReadingTimeMinutes,
			WordCount:            a.WordCount,
			PrimarySectionID:     a.PrimarySectionID,
			FeaturedImageKey:     a.FeaturedImageKey,
			FeaturedImageCaption: a.FeaturedImageCaption,
			IsTopStory:           a.IsTopStory,
			TopStoryOrder:        a.TopStoryOrder,
			PublishedAt:          a.PublishedAt,
		}
	}
	return items, nil
}

// GetRelatedArticles retrieves related articles for a given article ID.
func (q *ArticleQueries) GetRelatedArticles(ctx context.Context, articleID string, limit int) ([]ArticleFeedItem, error) {
	articles, err := q.articleRepo.GetRelatedArticles(ctx, articleID, limit)
	if err != nil {
		return nil, err
	}

	items := make([]ArticleFeedItem, len(articles))
	for i, a := range articles {
		hasPDF := a.PDFStorageKey != nil && *a.PDFStorageKey != ""
		items[i] = ArticleFeedItem{
			ID:                   a.ID,
			Slug:                 a.Slug,
			Title:                a.Title,
			Subtitle:             a.Subtitle,
			Summary:              a.Summary,
			ArticleType:          a.ArticleType,
			AccessLevel:          a.AccessLevel,
			Category:             a.Category,
			Tags:                 a.Tags,
			CourtName:            a.CourtName,
			Citation:             a.Citation,
			HasAttachment:        hasPDF,
			ReadingTimeMinutes:   a.ReadingTimeMinutes,
			WordCount:            a.WordCount,
			PrimarySectionID:     a.PrimarySectionID,
			FeaturedImageKey:     a.FeaturedImageKey,
			FeaturedImageCaption: a.FeaturedImageCaption,
			IsTopStory:           a.IsTopStory,
			TopStoryOrder:        a.TopStoryOrder,
			PublishedAt:          a.PublishedAt,
		}
	}
	return items, nil
}

// ListRevisions returns all revision snapshots for an article.
func (q *ArticleQueries) ListRevisions(ctx context.Context, articleID string) ([]content.ArticleRevision, error) {
	return q.articleRepo.ListRevisions(ctx, articleID)
}

// GetRevision returns a specific revision snapshot.
func (q *ArticleQueries) GetRevision(ctx context.Context, articleID string, version int) (*content.ArticleRevision, error) {
	return q.articleRepo.GetRevision(ctx, articleID, version)
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
	IsLocked       bool                `json:"is_locked"`
	PublishedAt    *time.Time          `json:"published_at,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
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
		CreatedAt:     a.CreatedAt,
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
func (q *ArticleQueries) GetJudgmentByCaseID(ctx context.Context, caseID string, userID *string) (*JudgmentDetailResponse, error) {
	a, err := q.articleRepo.GetJudgmentByCaseID(ctx, caseID)
	if err != nil {
		return nil, err
	}

	if a.Status != content.StatusPublished || a.DeletedAt != nil {
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
		PDFSizeBytes:  a.PDFSizeBytes,
		PublishedAt:   a.PublishedAt,
		CreatedAt:     a.CreatedAt,
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
		resp.IsLocked = true
		resp.BodyBlocks = nil
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
