package content

import (
	"errors"
	"time"
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
	ID             string
	Slug           string
	Title          string
	Subtitle       *string
	Summary        string
	BodyBlocks     []byte // Raw JSON blocks
	ArticleType    ArticleType
	AccessLevel    AccessLevel
	Status         ArticleStatus
	AuthorID       string
	EditorID       *string
	CourtCaseID    *string
	Category       string
	Tags           []string
	CourtName      *string
	Citation       *string
	PDFStorageKey  *string
	PDFSizeBytes   *int64
	PDFChecksum    *string
	PublishedAt    *time.Time
	Version        int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CanSubmit checks if article can transition to in_review.
func (a *Article) CanSubmit() error {
	if a.Status != StatusDraft {
		return errors.New("only draft articles can be submitted for review")
	}
	return nil
}

// CanPublish checks if article can be published.
func (a *Article) CanPublish() error {
	if a.Status != StatusInReview && a.Status != StatusDraft {
		return errors.New("article must be in review or draft to be published")
	}
	return nil
}
