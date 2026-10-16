package content

import (
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
)

// ArticleType represents the journalistic content category.
type ArticleType string

const (
	ArticleTypeNews            ArticleType = "news"
	ArticleTypeCommentary      ArticleType = "commentary"
	ArticleTypeEditorial       ArticleType = "editorial"
	ArticleTypeJudgmentSummary ArticleType = "judgment_summary"
)

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
	StatusPublished ArticleStatus = "published"
	StatusArchived  ArticleStatus = "archived"
)

// Article represents an editorial article or judgment summary.
type Article struct {
	ID             string        `json:"id"`
	Slug           string        `json:"slug"`
	Title          string        `json:"title"`
	Subtitle       *string       `json:"subtitle,omitempty"`
	Summary        string        `json:"summary"`
	BodyBlocks     []byte        `json:"body_blocks"` // Raw JSON blocks
	ArticleType    ArticleType   `json:"article_type"`
	AccessLevel    AccessLevel   `json:"access_level"`
	Status         ArticleStatus `json:"status"`
	AuthorID       string        `json:"author_id"`
	EditorID       *string       `json:"editor_id,omitempty"`
	CourtCaseID    *string       `json:"court_case_id,omitempty"`
	Category       string        `json:"category"`
	Tags           []string      `json:"tags"`
	CourtName      *string       `json:"court_name,omitempty"`
	Citation       *string       `json:"citation,omitempty"`
	PDFStorageKey  *string       `json:"pdf_storage_key,omitempty"`
	PDFSizeBytes   *int64        `json:"pdf_size_bytes,omitempty"`
	PDFChecksum    *string       `json:"pdf_checksum,omitempty"`
	PublishedAt    *time.Time    `json:"published_at,omitempty"`
	Version        int           `json:"version"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
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
	if a.Status != StatusInReview && a.Status != StatusDraft {
		return ErrInvalidStatus
	}
	if strings.TrimSpace(a.Title) == "" || strings.TrimSpace(a.Summary) == "" {
		return ErrValidationFailed
	}
	return nil
}

// SubmitForReview transitions the article from draft to in_review.
// Only the author or staff (editor/admin) can submit a draft.
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
// Only an editor or admin can send back an article.
func (a *Article) SendBack(actorRole string, comment string) error {
	if actorRole != "admin" && actorRole != "editor" {
		return ErrUnauthorizedWorkflow
	}
	if a.Status != StatusInReview {
		return ErrInvalidStatus
	}
	if strings.TrimSpace(comment) == "" {
		return ErrCommentRequired
	}

	a.Status = StatusDraft
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// ApproveAndPublish publishes the article.
// Only an editor or admin can approve and publish an article.
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

// Archive transitions the article to archived.
// Only an editor or admin can archive an article.
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

	// Map unicode non-alphanumeric runes to ASCII or hyphens
	var b strings.Builder
	for _, r := range cleaned {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}

	// Replace consecutive hyphens with a single hyphen
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
	totalWords := len(strings.Fields(summary))
	if len(bodyBlocks) > 0 {
		// Approximate word count from body text blocks
		bodyStr := string(bodyBlocks)
		totalWords += len(strings.Fields(bodyStr)) / 2 // Accounts for JSON structure overhead
	}
	mins := totalWords / 200
	if mins < 1 {
		return 1
	}
	return mins
}
