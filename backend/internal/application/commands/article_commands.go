package commands

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
	"github.com/livelaw/backend/internal/domain/identity"
)

var (
	ErrInvalidFileType = errors.New("unsupported file type: must be application/pdf or image (jpeg, png, webp)")
	ErrFileTooLarge    = errors.New("file exceeds maximum allowed size")
)

const (
	MaxPDFSizeBytes   int64 = 50 * 1024 * 1024 // 50 MB
	MaxImageSizeBytes int64 = 10 * 1024 * 1024 // 10 MB
)

// ArticleUseCases orchestrates editorial workflow, articles, and file upload business logic.
type ArticleUseCases struct {
	articleRepo ports.ArticleRepository
	idGen       ports.IDGenerator
	clock       ports.Clock
	fileStore   ports.FileStore
}

// NewArticleUseCases creates a new ArticleUseCases instance.
func NewArticleUseCases(
	articleRepo ports.ArticleRepository,
	idGen ports.IDGenerator,
	clock ports.Clock,
	fileStore ports.FileStore,
) *ArticleUseCases {
	return &ArticleUseCases{
		articleRepo: articleRepo,
		idGen:       idGen,
		clock:       clock,
		fileStore:   fileStore,
	}
}

// CreateDraftCommand holds parameters for creating a new article draft.
type CreateDraftCommand struct {
	ActorID              string
	ActorRole            identity.SystemRole
	Title                string
	Subtitle             *string
	Summary              string
	BodyBlocks           []byte
	ArticleType          content.ArticleType
	AccessLevel          content.AccessLevel
	Category             string
	Tags                 []string
	CourtCaseID          *string
	CourtName            *string
	Citation             *string
	PrimarySectionID     *string
	FeaturedImageKey     *string
	FeaturedImageCaption *string
	FeaturedImageAlt     *string
	MetaTitle            *string
	MetaDescription      *string
	CanonicalURL         *string
	SocialImageKey       *string
	IsTopStory           bool
	TopStoryOrder        int
	RelatedArticleIDs    []string
}

// CreateDraft creates a new draft article with a guaranteed unique slug.
func (uc *ArticleUseCases) CreateDraft(ctx context.Context, cmd CreateDraftCommand) (*content.Article, error) {
	if cmd.ActorRole != identity.RoleReporter && cmd.ActorRole != identity.RoleEditor && cmd.ActorRole != identity.RoleAdmin {
		return nil, content.ErrUnauthorizedWorkflow
	}

	if strings.TrimSpace(cmd.Title) == "" || strings.TrimSpace(cmd.Summary) == "" || strings.TrimSpace(cmd.Category) == "" {
		return nil, content.ErrValidationFailed
	}

	articleType := cmd.ArticleType
	if articleType == "" {
		articleType = content.ArticleTypeNews
	}
	if !content.IsValidArticleType(articleType) {
		return nil, fmt.Errorf("%w: invalid article type %s", content.ErrValidationFailed, articleType)
	}

	accessLevel := cmd.AccessLevel
	if accessLevel == "" {
		accessLevel = content.AccessLevelFree
	}

	articleID := uc.idGen.NewID()
	now := uc.clock.Now().UTC()

	baseSlug := content.GenerateSlug(cmd.Title)
	slug, err := uc.ensureUniqueSlug(ctx, baseSlug, articleID)
	if err != nil {
		return nil, err
	}

	bodyBlocks := cmd.BodyBlocks
	if len(bodyBlocks) == 0 {
		bodyBlocks = []byte("[]")
	}

	tags := cmd.Tags
	if tags == nil {
		tags = []string{}
	}

	wordCount, readingTime := content.CalculateWordCountAndReadingTime(cmd.Summary, bodyBlocks)

	a := &content.Article{
		ID:                   articleID,
		Slug:                 slug,
		Title:                strings.TrimSpace(cmd.Title),
		Subtitle:             cmd.Subtitle,
		Summary:              strings.TrimSpace(cmd.Summary),
		BodyBlocks:           bodyBlocks,
		ArticleType:          articleType,
		AccessLevel:          accessLevel,
		Status:               content.StatusDraft,
		AuthorID:             cmd.ActorID,
		Category:             strings.TrimSpace(cmd.Category),
		Tags:                 tags,
		CourtCaseID:          cmd.CourtCaseID,
		CourtName:            cmd.CourtName,
		Citation:             cmd.Citation,
		PrimarySectionID:     cmd.PrimarySectionID,
		WordCount:            wordCount,
		ReadingTimeMinutes:   readingTime,
		FeaturedImageKey:     cmd.FeaturedImageKey,
		FeaturedImageCaption: cmd.FeaturedImageCaption,
		FeaturedImageAlt:     cmd.FeaturedImageAlt,
		MetaTitle:            cmd.MetaTitle,
		MetaDescription:      cmd.MetaDescription,
		CanonicalURL:         cmd.CanonicalURL,
		SocialImageKey:       cmd.SocialImageKey,
		IsTopStory:           cmd.IsTopStory,
		TopStoryOrder:        cmd.TopStoryOrder,
		RelatedArticleIDs:    cmd.RelatedArticleIDs,
		Version:              1,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := uc.articleRepo.Create(ctx, a); err != nil {
		return nil, err
	}

	return a, nil
}

// UpdateDraftCommand holds parameters for modifying an existing article draft.
type UpdateDraftCommand struct {
	ID                   string
	ActorID              string
	ActorRole            identity.SystemRole
	ExpectedVersion      int
	Title                string
	Subtitle             *string
	Summary              string
	BodyBlocks           []byte
	ArticleType          content.ArticleType
	AccessLevel          content.AccessLevel
	Category             string
	Tags                 []string
	CourtCaseID          *string
	CourtName            *string
	Citation             *string
	PrimarySectionID     *string
	FeaturedImageKey     *string
	FeaturedImageCaption *string
	FeaturedImageAlt     *string
	MetaTitle            *string
	MetaDescription      *string
	CanonicalURL         *string
	SocialImageKey       *string
	IsTopStory           *bool
	TopStoryOrder        *int
	RelatedArticleIDs    []string
}

// UpdateDraft modifies an article draft with optimistic concurrency locking.
func (uc *ArticleUseCases) UpdateDraft(ctx context.Context, cmd UpdateDraftCommand) (*content.Article, error) {
	existing, err := uc.articleRepo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	// Permission check: authors can only edit their own drafts; editors and admins can edit any
	if cmd.ActorRole != identity.RoleAdmin && cmd.ActorRole != identity.RoleEditor {
		if existing.AuthorID != cmd.ActorID {
			return nil, content.ErrUnauthorizedWorkflow
		}
		if existing.Status != content.StatusDraft {
			return nil, content.ErrInvalidStatus
		}
	} else {
		if existing.Status == content.StatusArchived {
			return nil, errors.New("cannot edit an archived article")
		}
	}

	if strings.TrimSpace(cmd.Title) == "" || strings.TrimSpace(cmd.Summary) == "" || strings.TrimSpace(cmd.Category) == "" {
		return nil, content.ErrValidationFailed
	}

	// If title changed, update slug
	if existing.Title != cmd.Title {
		baseSlug := content.GenerateSlug(cmd.Title)
		newSlug, err := uc.ensureUniqueSlug(ctx, baseSlug, existing.ID)
		if err != nil {
			return nil, err
		}
		existing.Slug = newSlug
	}

	existing.Title = strings.TrimSpace(cmd.Title)
	existing.Subtitle = cmd.Subtitle
	existing.Summary = strings.TrimSpace(cmd.Summary)
	if len(cmd.BodyBlocks) > 0 {
		existing.BodyBlocks = cmd.BodyBlocks
	}
	if cmd.ArticleType != "" {
		if !content.IsValidArticleType(cmd.ArticleType) {
			return nil, fmt.Errorf("%w: invalid article type %s", content.ErrValidationFailed, cmd.ArticleType)
		}
		existing.ArticleType = cmd.ArticleType
	}
	if cmd.AccessLevel != "" {
		existing.AccessLevel = cmd.AccessLevel
	}
	existing.Category = strings.TrimSpace(cmd.Category)
	if cmd.Tags != nil {
		existing.Tags = cmd.Tags
	}
	existing.CourtCaseID = cmd.CourtCaseID
	existing.CourtName = cmd.CourtName
	existing.Citation = cmd.Citation
	existing.PrimarySectionID = cmd.PrimarySectionID
	if cmd.FeaturedImageKey != nil {
		existing.FeaturedImageKey = cmd.FeaturedImageKey
	}
	if cmd.FeaturedImageCaption != nil {
		existing.FeaturedImageCaption = cmd.FeaturedImageCaption
	}
	if cmd.FeaturedImageAlt != nil {
		existing.FeaturedImageAlt = cmd.FeaturedImageAlt
	}
	if cmd.MetaTitle != nil {
		existing.MetaTitle = cmd.MetaTitle
	}
	if cmd.MetaDescription != nil {
		existing.MetaDescription = cmd.MetaDescription
	}
	if cmd.CanonicalURL != nil {
		existing.CanonicalURL = cmd.CanonicalURL
	}
	if cmd.SocialImageKey != nil {
		existing.SocialImageKey = cmd.SocialImageKey
	}
	if cmd.IsTopStory != nil {
		existing.IsTopStory = *cmd.IsTopStory
	}
	if cmd.TopStoryOrder != nil {
		existing.TopStoryOrder = *cmd.TopStoryOrder
	}
	if cmd.RelatedArticleIDs != nil {
		existing.RelatedArticleIDs = cmd.RelatedArticleIDs
	}

	existing.UpdatedAt = uc.clock.Now().UTC()

	if err := uc.articleRepo.Update(ctx, existing, cmd.ExpectedVersion); err != nil {
		return nil, err
	}

	return existing, nil
}

// SubmitForReviewCommand moves a draft into in_review.
type SubmitForReviewCommand struct {
	ID        string
	ActorID   string
	ActorRole identity.SystemRole
}

// SubmitForReview transitions an article from draft to in_review.
func (uc *ArticleUseCases) SubmitForReview(ctx context.Context, cmd SubmitForReviewCommand) (*content.Article, error) {
	a, err := uc.articleRepo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	if err := a.SubmitForReview(cmd.ActorID, string(cmd.ActorRole)); err != nil {
		return nil, err
	}

	if err := uc.articleRepo.Update(ctx, a, a.Version); err != nil {
		return nil, err
	}

	return a, nil
}

// SendBackCommand moves an in_review article back to draft with review comments.
type SendBackCommand struct {
	ID        string
	ActorRole identity.SystemRole
	Comment   string
}

// SendBack returns an article in review to draft status for revisions.
func (uc *ArticleUseCases) SendBack(ctx context.Context, cmd SendBackCommand) (*content.Article, error) {
	a, err := uc.articleRepo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	if err := a.SendBack(string(cmd.ActorRole), cmd.Comment); err != nil {
		return nil, err
	}

	if err := uc.articleRepo.Update(ctx, a, a.Version); err != nil {
		return nil, err
	}

	return a, nil
}

// PublishArticleCommand approves and publishes an article.
type PublishArticleCommand struct {
	ID        string
	ActorID   string
	ActorRole identity.SystemRole
}

// PublishArticle publishes an article, making it available on public feeds.
func (uc *ArticleUseCases) PublishArticle(ctx context.Context, cmd PublishArticleCommand) (*content.Article, error) {
	a, err := uc.articleRepo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	now := uc.clock.Now().UTC()
	if err := a.ApproveAndPublish(cmd.ActorID, string(cmd.ActorRole), now); err != nil {
		return nil, err
	}

	if err := uc.articleRepo.Update(ctx, a, a.Version); err != nil {
		return nil, err
	}

	return a, nil
}

// ScheduleArticleCommand schedules an article for future publication.
type ScheduleArticleCommand struct {
	ID          string
	ActorRole   identity.SystemRole
	ScheduledAt time.Time
}

// ScheduleArticle schedules publication for an article.
func (uc *ArticleUseCases) ScheduleArticle(ctx context.Context, cmd ScheduleArticleCommand) (*content.Article, error) {
	a, err := uc.articleRepo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	now := uc.clock.Now().UTC()
	if err := a.Schedule(string(cmd.ActorRole), cmd.ScheduledAt, now); err != nil {
		return nil, err
	}

	if err := uc.articleRepo.Update(ctx, a, a.Version); err != nil {
		return nil, err
	}

	return a, nil
}

// SoftDeleteArticle soft-deletes an article.
func (uc *ArticleUseCases) SoftDeleteArticle(ctx context.Context, id string, actorRole identity.SystemRole) error {
	if actorRole != identity.RoleAdmin && actorRole != identity.RoleEditor {
		return content.ErrUnauthorizedWorkflow
	}
	now := uc.clock.Now().UTC()
	return uc.articleRepo.SoftDelete(ctx, id, now)
}

// RestoreArticle restores a soft-deleted article.
func (uc *ArticleUseCases) RestoreArticle(ctx context.Context, id string, actorRole identity.SystemRole) error {
	if actorRole != identity.RoleAdmin && actorRole != identity.RoleEditor {
		return content.ErrUnauthorizedWorkflow
	}
	now := uc.clock.Now().UTC()
	return uc.articleRepo.Restore(ctx, id, now)
}

// RestoreRevision restores an article's draft content to a historical revision version.
func (uc *ArticleUseCases) RestoreRevision(ctx context.Context, articleID string, version int, actorID string, actorRole identity.SystemRole) (*content.Article, error) {
	if actorRole != identity.RoleAdmin && actorRole != identity.RoleEditor {
		return nil, content.ErrUnauthorizedWorkflow
	}

	existing, err := uc.articleRepo.GetByID(ctx, articleID)
	if err != nil {
		return nil, err
	}

	rev, err := uc.articleRepo.GetRevision(ctx, articleID, version)
	if err != nil {
		return nil, err
	}

	// Apply revision content
	existing.Title = rev.Title
	existing.Subtitle = rev.Subtitle
	existing.Summary = rev.Summary
	existing.BodyBlocks = rev.BodyBlocks
	existing.UpdatedAt = uc.clock.Now().UTC()

	if err := uc.articleRepo.Update(ctx, existing, existing.Version); err != nil {
		return nil, err
	}

	return existing, nil
}

// UpdateTopStories updates pinned top stories orders.
func (uc *ArticleUseCases) UpdateTopStories(ctx context.Context, items []content.TopStoryOrderItem, actorRole identity.SystemRole) error {
	if actorRole != identity.RoleAdmin && actorRole != identity.RoleEditor {
		return content.ErrUnauthorizedWorkflow
	}
	return uc.articleRepo.UpdateTopStoriesOrder(ctx, items)
}

// ArchiveArticleCommand archives an article.
type ArchiveArticleCommand struct {
	ID        string
	ActorRole identity.SystemRole
}

// ArchiveArticle moves an article into archived status.
func (uc *ArticleUseCases) ArchiveArticle(ctx context.Context, cmd ArchiveArticleCommand) (*content.Article, error) {
	a, err := uc.articleRepo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	now := uc.clock.Now().UTC()
	if err := a.Archive(string(cmd.ActorRole), now); err != nil {
		return nil, err
	}

	if err := uc.articleRepo.Update(ctx, a, a.Version); err != nil {
		return nil, err
	}

	return a, nil
}

// CreateJudgmentSummaryCommand creates an article dedicated to summarizing a judgment and attaches to court case.
type CreateJudgmentSummaryCommand struct {
	ActorID       string
	ActorRole     identity.SystemRole
	CourtCaseID   string
	Title         string
	Summary       string
	BodyBlocks    []byte
	AccessLevel   content.AccessLevel
	CourtName     string
	Citation      *string
	PDFStorageKey *string
	PDFSizeBytes  *int64
	PDFChecksum   *string
}

// CreateJudgmentSummary creates a judgment summary article and transactionally links it to the court case.
func (uc *ArticleUseCases) CreateJudgmentSummary(ctx context.Context, cmd CreateJudgmentSummaryCommand) (*content.Article, error) {
	if cmd.ActorRole != identity.RoleEditor && cmd.ActorRole != identity.RoleAdmin {
		return nil, content.ErrUnauthorizedWorkflow
	}

	if strings.TrimSpace(cmd.CourtCaseID) == "" || strings.TrimSpace(cmd.Title) == "" || strings.TrimSpace(cmd.Summary) == "" {
		return nil, content.ErrValidationFailed
	}

	articleID := uc.idGen.NewID()
	now := uc.clock.Now().UTC()

	baseSlug := content.GenerateSlug(cmd.Title)
	slug, err := uc.ensureUniqueSlug(ctx, baseSlug, articleID)
	if err != nil {
		return nil, err
	}

	bodyBlocks := cmd.BodyBlocks
	if len(bodyBlocks) == 0 {
		bodyBlocks = []byte("[]")
	}

	accessLevel := cmd.AccessLevel
	if accessLevel == "" {
		accessLevel = content.AccessLevelFree
	}

	wordCount, readingTime := content.CalculateWordCountAndReadingTime(cmd.Summary, bodyBlocks)

	a := &content.Article{
		ID:                 articleID,
		Slug:               slug,
		Title:              strings.TrimSpace(cmd.Title),
		Summary:            strings.TrimSpace(cmd.Summary),
		BodyBlocks:         bodyBlocks,
		ArticleType:        content.ArticleTypeJudgmentSummary,
		AccessLevel:        accessLevel,
		Status:             content.StatusDraft,
		AuthorID:           cmd.ActorID,
		CourtCaseID:        &cmd.CourtCaseID,
		Category:           "Judgments",
		Tags:               []string{"judgment", "summary"},
		CourtName:          &cmd.CourtName,
		Citation:           cmd.Citation,
		PDFStorageKey:      cmd.PDFStorageKey,
		PDFSizeBytes:       cmd.PDFSizeBytes,
		PDFChecksum:        cmd.PDFChecksum,
		WordCount:          wordCount,
		ReadingTimeMinutes: readingTime,
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := uc.articleRepo.Create(ctx, a); err != nil {
		return nil, err
	}

	if err := uc.articleRepo.LinkJudgmentToCase(ctx, cmd.CourtCaseID, articleID); err != nil {
		return nil, fmt.Errorf("failed linking judgment to court case: %w", err)
	}

	return a, nil
}

// AttachPDFCommand attaches official judgment PDF metadata to an article.
type AttachPDFCommand struct {
	ArticleID  string
	ActorID    string
	ActorRole  identity.SystemRole
	StorageKey string
	SizeBytes  int64
	Checksum   string
}

// AttachPDF updates the article's PDF attachment metadata.
func (uc *ArticleUseCases) AttachPDF(ctx context.Context, cmd AttachPDFCommand) (*content.Article, error) {
	if cmd.ActorRole != identity.RoleEditor && cmd.ActorRole != identity.RoleAdmin && cmd.ActorRole != identity.RoleReporter {
		return nil, content.ErrUnauthorizedWorkflow
	}

	a, err := uc.articleRepo.GetByID(ctx, cmd.ArticleID)
	if err != nil {
		return nil, err
	}

	if cmd.ActorRole == identity.RoleReporter && a.AuthorID != cmd.ActorID {
		return nil, content.ErrUnauthorizedWorkflow
	}

	now := uc.clock.Now().UTC()
	if err := a.AttachPDF(cmd.StorageKey, cmd.SizeBytes, cmd.Checksum, now); err != nil {
		return nil, err
	}

	if err := uc.articleRepo.Update(ctx, a, a.Version); err != nil {
		return nil, err
	}

	return a, nil
}

// RequestUploadURLCommand validates upload intent and requests a signed S3 upload URL.
type RequestUploadURLCommand struct {
	ActorRole   identity.SystemRole
	Filename    string
	ContentType string
	SizeBytes   int64
	Purpose     string // "judgment_pdf" or "article_image"
}

// UploadURLResult contains the generated presigned upload URL and the permanent storage key.
type UploadURLResult struct {
	UploadURL  string `json:"upload_url"`
	StorageKey string `json:"storage_key"`
	ExpiresIn  int    `json:"expires_in_seconds"`
}

// RequestUploadURL validates file type/size limits and returns a signed direct-to-S3 upload URL.
func (uc *ArticleUseCases) RequestUploadURL(ctx context.Context, cmd RequestUploadURLCommand) (*UploadURLResult, error) {
	if cmd.ActorRole != identity.RoleReporter && cmd.ActorRole != identity.RoleEditor && cmd.ActorRole != identity.RoleAdmin {
		return nil, content.ErrUnauthorizedWorkflow
	}

	if uc.fileStore == nil {
		return nil, errors.New("file storage service unavailable")
	}

	ext := strings.ToLower(filepath.Ext(cmd.Filename))
	contentType := strings.ToLower(strings.TrimSpace(cmd.ContentType))

	var subfolder string
	switch {
	case contentType == "application/pdf" || ext == ".pdf":
		if cmd.SizeBytes > MaxPDFSizeBytes {
			return nil, fmt.Errorf("%w: PDF size %d bytes exceeds %d bytes limit", ErrFileTooLarge, cmd.SizeBytes, MaxPDFSizeBytes)
		}
		subfolder = "judgments"
		contentType = "application/pdf"
		if ext == "" {
			ext = ".pdf"
		}
	case contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/webp" ||
		ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp":
		if cmd.SizeBytes > MaxImageSizeBytes {
			return nil, fmt.Errorf("%w: Image size %d bytes exceeds %d bytes limit", ErrFileTooLarge, cmd.SizeBytes, MaxImageSizeBytes)
		}
		subfolder = "images"
		if ext == "" {
			switch contentType {
			case "image/png":
				ext = ".png"
			case "image/webp":
				ext = ".webp"
			default:
				ext = ".jpg"
			}
		}
	default:
		return nil, ErrInvalidFileType
	}

	objectID := uc.idGen.NewID()
	yearMonth := uc.clock.Now().UTC().Format("2006/01")
	storageKey := fmt.Sprintf("%s/%s/%s%s", subfolder, yearMonth, objectID, ext)

	expiry := 15 * time.Minute
	uploadURL, err := uc.fileStore.PresignedPutURL(ctx, storageKey, contentType, expiry)
	if err != nil {
		return nil, fmt.Errorf("failed generating upload URL: %w", err)
	}

	return &UploadURLResult{
		UploadURL:  uploadURL,
		StorageKey: storageKey,
		ExpiresIn:  int(expiry.Seconds()),
	}, nil
}

func (uc *ArticleUseCases) ensureUniqueSlug(ctx context.Context, baseSlug string, articleID string) (string, error) {
	slug := baseSlug
	suffix := 1

	for {
		exists, err := uc.articleRepo.SlugExists(ctx, slug, articleID)
		if err != nil {
			return "", fmt.Errorf("error checking slug uniqueness: %w", err)
		}
		if !exists {
			return slug, nil
		}

		suffix++
		slug = fmt.Sprintf("%s-%d", baseSlug, suffix)
		if suffix > 100 {
			return fmt.Sprintf("%s-%s", baseSlug, articleID[:8]), nil
		}
	}
}
