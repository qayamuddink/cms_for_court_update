package content

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Standard domain errors for articles.
var (
	ErrInvalidStatus          = errors.New("invalid article status for this operation")
	ErrUnauthorizedWorkflow   = errors.New("unauthorized action for current role or author")
	ErrValidationFailed       = errors.New("article validation failed")
	ErrOptimisticLockConflict = errors.New("article has been modified by another user (version conflict)")
	ErrCommentRequired        = errors.New("a review comment or reason is required")
	ErrRevisionNotFound       = errors.New("article revision not found")
	ErrSectionNotFound        = errors.New("section not found")
)

// ArticleType represents the journalistic content category.
type ArticleType string

const (
	ArticleTypeNews            ArticleType = "news"
	ArticleTypeCommentary      ArticleType = "commentary"
	ArticleTypeEditorial       ArticleType = "editorial"
	ArticleTypeJudgmentSummary ArticleType = "judgment_summary"
	ArticleTypeTopStory        ArticleType = "top_story"
	ArticleTypeColumn          ArticleType = "column"
	ArticleTypeOpinion         ArticleType = "opinion"
	ArticleTypeInterview       ArticleType = "interview"
	ArticleTypeRoundUp         ArticleType = "round_up"
	ArticleTypeDigest          ArticleType = "digest"
	ArticleTypeKnowTheLaw      ArticleType = "know_the_law"
	ArticleTypeBookReview      ArticleType = "book_review"
	ArticleTypeJobUpdate       ArticleType = "job_update"
	ArticleTypeEvent           ArticleType = "event"
	ArticleTypeVideo           ArticleType = "video"
	ArticleTypeCartoon         ArticleType = "cartoon"
)

// IsValidArticleType validates whether an article type is recognized.
func IsValidArticleType(t ArticleType) bool {
	switch t {
	case ArticleTypeNews, ArticleTypeCommentary, ArticleTypeEditorial, ArticleTypeJudgmentSummary,
		ArticleTypeTopStory, ArticleTypeColumn, ArticleTypeOpinion, ArticleTypeInterview,
		ArticleTypeRoundUp, ArticleTypeDigest, ArticleTypeKnowTheLaw, ArticleTypeBookReview,
		ArticleTypeJobUpdate, ArticleTypeEvent, ArticleTypeVideo, ArticleTypeCartoon:
		return true
	default:
		return false
	}
}

// AccessLevel defines whether the article sits behind the paywall.
type AccessLevel string

const (
	AccessLevelFree    AccessLevel = "free"
	AccessLevelPremium AccessLevel = "premium"
)

// ArticleStatus defines article editorial workflow stages.
type ArticleStatus string

const (
	StatusDraft     ArticleStatus = "draft"
	StatusInReview  ArticleStatus = "in_review"
	StatusScheduled ArticleStatus = "scheduled"
	StatusPublished ArticleStatus = "published"
	StatusArchived  ArticleStatus = "archived"
)

// Section represents a taxonomy category (e.g. Supreme Court, High Courts, Tax, IBC).
type Section struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Description  *string   `json:"description,omitempty"`
	DisplayOrder int       `json:"display_order"`
	ParentID     *string   `json:"parent_id,omitempty"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ArticleRevision stores point-in-time snapshots for audit and side-by-side diff.
type ArticleRevision struct {
	ID         string    `json:"id"`
	ArticleID  string    `json:"article_id"`
	Version    int       `json:"version"`
	Title      string    `json:"title"`
	Subtitle   *string   `json:"subtitle,omitempty"`
	Summary    string    `json:"summary"`
	BodyBlocks []byte    `json:"body_blocks"`
	AuthorID   string    `json:"author_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// SlugRedirect represents an old slug that should 301 redirect to the active slug.
type SlugRedirect struct {
	ID        string    `json:"id"`
	ArticleID string    `json:"article_id"`
	OldSlug   string    `json:"old_slug"`
	CreatedAt time.Time `json:"created_at"`
}

// OutlineItem represents a table-of-contents entry extracted from headings.
type OutlineItem struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Level int    `json:"level"`
}

// TopStoryOrderItem defines reordering payload for pinned top stories.
type TopStoryOrderItem struct {
	ArticleID     string `json:"article_id"`
	TopStoryOrder int    `json:"top_story_order"`
	IsTopStory    bool   `json:"is_top_story"`
}

// Article represents an editorial article or judgment summary.
type Article struct {
	ID                   string        `json:"id"`
	Slug                 string        `json:"slug"`
	Title                string        `json:"title"`
	Subtitle             *string       `json:"subtitle,omitempty"`
	Summary              string        `json:"summary"`
	BodyBlocks           []byte        `json:"body_blocks"` // Raw JSON blocks
	ArticleType          ArticleType   `json:"article_type"`
	AccessLevel          AccessLevel   `json:"access_level"`
	Status               ArticleStatus `json:"status"`
	AuthorID             string        `json:"author_id"`
	EditorID             *string       `json:"editor_id,omitempty"`
	CourtCaseID          *string       `json:"court_case_id,omitempty"`
	Category             string        `json:"category"`
	Tags                 []string      `json:"tags"`
	CourtName            *string       `json:"court_name,omitempty"`
	Citation             *string       `json:"citation,omitempty"`
	PDFStorageKey        *string       `json:"pdf_storage_key,omitempty"`
	PDFSizeBytes         *int64        `json:"pdf_size_bytes,omitempty"`
	PDFChecksum          *string       `json:"pdf_checksum,omitempty"`
	PublishedAt          *time.Time    `json:"published_at,omitempty"`
	Version              int           `json:"version"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`

	// Phase 9 extensions:
	PrimarySectionID     *string       `json:"primary_section_id,omitempty"`
	PrimarySection       *Section      `json:"primary_section,omitempty"`
	WordCount            int           `json:"word_count"`
	ReadingTimeMinutes   int           `json:"reading_time_minutes"`
	FeaturedImageKey     *string       `json:"featured_image_key,omitempty"`
	FeaturedImageCaption *string       `json:"featured_image_caption,omitempty"`
	FeaturedImageAlt     *string       `json:"featured_image_alt,omitempty"`
	MetaTitle            *string       `json:"meta_title,omitempty"`
	MetaDescription      *string       `json:"meta_description,omitempty"`
	CanonicalURL         *string       `json:"canonical_url,omitempty"`
	SocialImageKey       *string       `json:"social_image_key,omitempty"`
	IsTopStory           bool          `json:"is_top_story"`
	TopStoryOrder        int           `json:"top_story_order"`
	ScheduledAt          *time.Time    `json:"scheduled_at,omitempty"`
	DeletedAt            *time.Time    `json:"deleted_at,omitempty"`
	RelatedArticleIDs    []string      `json:"related_article_ids"`
}

// CanSubmit checks if article can transition to in_review.
func (a *Article) CanSubmit() error {
	if a.Status != StatusDraft {
		return ErrInvalidStatus
	}
	if strings.TrimSpace(a.Title) == "" || strings.TrimSpace(a.Summary) == "" || strings.TrimSpace(a.Category) == "" {
		return ErrValidationFailed
	}
	return nil
}

// CanPublish checks if article can be published.
func (a *Article) CanPublish() error {
	if a.Status != StatusInReview && a.Status != StatusDraft && a.Status != StatusScheduled {
		return ErrInvalidStatus
	}
	if strings.TrimSpace(a.Title) == "" || strings.TrimSpace(a.Summary) == "" {
		return ErrValidationFailed
	}
	return nil
}

// SubmitForReview transitions the article from draft to in_review.
func (a *Article) SubmitForReview(actorID string, actorRole string) error {
	if a.Status != StatusDraft {
		return ErrInvalidStatus
	}
	if actorRole != "admin" && actorRole != "editor" && a.AuthorID != actorID {
		return ErrUnauthorizedWorkflow
	}
	if err := a.CanSubmit(); err != nil {
		return err
	}

	a.Status = StatusInReview
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// SendBack transitions the article from in_review back to draft with a required feedback comment.
func (a *Article) SendBack(actorRole string, comment string) error {
	if actorRole != "admin" && actorRole != "editor" {
		return ErrUnauthorizedWorkflow
	}
	if a.Status != StatusInReview && a.Status != StatusScheduled {
		return ErrInvalidStatus
	}
	if strings.TrimSpace(comment) == "" {
		return ErrCommentRequired
	}

	a.Status = StatusDraft
	a.ScheduledAt = nil
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// ApproveAndPublish publishes the article immediately.
func (a *Article) ApproveAndPublish(editorID string, actorRole string, now time.Time) error {
	if actorRole != "admin" && actorRole != "editor" {
		return ErrUnauthorizedWorkflow
	}
	if err := a.CanPublish(); err != nil {
		return err
	}

	a.Status = StatusPublished
	a.EditorID = &editorID
	utcNow := now.UTC()
	a.PublishedAt = &utcNow
	a.UpdatedAt = utcNow
	return nil
}

// Schedule sets the article status to scheduled with a future publish timestamp.
func (a *Article) Schedule(actorRole string, scheduledAt time.Time, now time.Time) error {
	if actorRole != "admin" && actorRole != "editor" {
		return ErrUnauthorizedWorkflow
	}
	if a.Status != StatusDraft && a.Status != StatusInReview && a.Status != StatusScheduled {
		return ErrInvalidStatus
	}
	if strings.TrimSpace(a.Title) == "" || strings.TrimSpace(a.Summary) == "" {
		return ErrValidationFailed
	}
	if scheduledAt.Before(now) {
		return errors.New("scheduled time must be in the future")
	}

	a.Status = StatusScheduled
	utcScheduled := scheduledAt.UTC()
	a.ScheduledAt = &utcScheduled
	a.UpdatedAt = now.UTC()
	return nil
}

// SoftDelete marks the article as deleted.
func (a *Article) SoftDelete(now time.Time) {
	utcNow := now.UTC()
	a.DeletedAt = &utcNow
	a.UpdatedAt = utcNow
}

// Restore restores a soft-deleted article.
func (a *Article) Restore(now time.Time) {
	a.DeletedAt = nil
	a.UpdatedAt = now.UTC()
}

// Archive transitions the article to archived.
func (a *Article) Archive(actorRole string, now time.Time) error {
	if actorRole != "admin" && actorRole != "editor" {
		return ErrUnauthorizedWorkflow
	}
	if a.Status == StatusArchived {
		return errors.New("article is already archived")
	}

	a.Status = StatusArchived
	a.UpdatedAt = now.UTC()
	return nil
}

// AttachPDF sets official judgment or article PDF attachment metadata.
func (a *Article) AttachPDF(storageKey string, sizeBytes int64, checksum string, now time.Time) error {
	if strings.TrimSpace(storageKey) == "" {
		return errors.New("storage key cannot be empty")
	}
	if sizeBytes <= 0 {
		return errors.New("file size must be greater than zero")
	}

	a.PDFStorageKey = &storageKey
	a.PDFSizeBytes = &sizeBytes
	a.PDFChecksum = &checksum
	a.UpdatedAt = now.UTC()
	return nil
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateSlug generates a clean, URL-friendly kebab-case slug from a title.
func GenerateSlug(title string) string {
	cleaned := strings.ToLower(strings.TrimSpace(title))
	if cleaned == "" {
		return "untitled"
	}

	var b strings.Builder
	for _, r := range cleaned {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}

	slug := nonAlphanumericRegex.ReplaceAllString(b.String(), "-")
	slug = strings.Trim(slug, "-")

	if len(slug) > 200 {
		slug = slug[:200]
		slug = strings.TrimRight(slug, "-")
	}

	if slug == "" {
		return "article"
	}
	return slug
}

// EstimateReadingTime calculates estimated reading time in minutes based on words (avg 200 wpm).
func EstimateReadingTime(summary string, bodyBlocks []byte) int {
	_, readingTime := CalculateWordCountAndReadingTime(summary, bodyBlocks)
	return readingTime
}

func extractNodeText(node map[string]any, sb *strings.Builder) {
	if textVal, ok := node["text"].(string); ok {
		sb.WriteString(textVal)
		sb.WriteString(" ")
	}
	if attrs, ok := node["attrs"].(map[string]any); ok {
		if q, ok := attrs["quote"].(string); ok {
			sb.WriteString(q)
			sb.WriteString(" ")
		}
		if caption, ok := attrs["caption"].(string); ok {
			sb.WriteString(caption)
			sb.WriteString(" ")
		}
		if alt, ok := attrs["alt"].(string); ok {
			sb.WriteString(alt)
			sb.WriteString(" ")
		}
	}
	if contentArr, ok := node["content"].([]any); ok {
		for _, child := range contentArr {
			if childMap, ok := child.(map[string]any); ok {
				extractNodeText(childMap, sb)
			}
		}
	}
}

// CalculateWordCountAndReadingTime parses text from summary and body JSON blocks, counting words and minutes.
func CalculateWordCountAndReadingTime(summary string, bodyBlocks []byte) (int, int) {
	var sb strings.Builder
	sb.WriteString(summary)
	sb.WriteString(" ")

	if len(bodyBlocks) > 0 {
		var raw any
		if err := json.Unmarshal(bodyBlocks, &raw); err == nil {
			switch val := raw.(type) {
			case []any:
				for _, item := range val {
					if itemMap, ok := item.(map[string]any); ok {
						extractNodeText(itemMap, &sb)
					}
				}
			case map[string]any:
				extractNodeText(val, &sb)
			}
		} else {
			sb.WriteString(string(bodyBlocks))
		}
	}

	words := strings.Fields(sb.String())
	wordCount := len(words)
	readingTime := wordCount / 200
	if readingTime < 1 {
		readingTime = 1
	}
	return wordCount, readingTime
}

// ExtractOutline extracts a structured table of contents from heading blocks in bodyBlocks.
func ExtractOutline(bodyBlocks []byte) []OutlineItem {
	var outline []OutlineItem
	if len(bodyBlocks) == 0 {
		return outline
	}

	var raw any
	if err := json.Unmarshal(bodyBlocks, &raw); err != nil {
		return outline
	}

	findHeadings(raw, &outline)
	return outline
}

func findHeadings(node any, outline *[]OutlineItem) {
	switch v := node.(type) {
	case []any:
		for _, item := range v {
			findHeadings(item, outline)
		}
	case map[string]any:
		if nodeType, ok := v["type"].(string); ok && nodeType == "heading" {
			level := 2
			if attrs, ok := v["attrs"].(map[string]any); ok {
				if lvlFloat, ok := attrs["level"].(float64); ok {
					level = int(lvlFloat)
				}
			}
			var sb strings.Builder
			extractNodeText(v, &sb)
			headingText := strings.TrimSpace(sb.String())
			if headingText != "" {
				anchorID := GenerateSlug(headingText)
				*outline = append(*outline, OutlineItem{
					ID:    anchorID,
					Text:  headingText,
					Level: level,
				})
			}
		}
		if contentArr, ok := v["content"].([]any); ok {
			for _, child := range contentArr {
				findHeadings(child, outline)
			}
		}
	}
}
