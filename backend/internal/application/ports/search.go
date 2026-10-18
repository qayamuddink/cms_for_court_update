package ports

import (
	"context"
	"time"

	"github.com/livelaw/backend/internal/domain/search"
)

// ArticleSearchDoc represents an article for full-text search indexing.
type ArticleSearchDoc struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Subtitle    string    `json:"subtitle"`
	Summary     string    `json:"summary"`
	BodyText    string    `json:"body_text"`
	Slug        string    `json:"slug"`
	ArticleType string    `json:"article_type"`
	SectionSlug string    `json:"section_slug"`
	SectionName string    `json:"section_name"`
	IsPremium   bool      `json:"is_premium"`
	PublishedAt time.Time `json:"published_at"`
}

// JudgmentStatuteSearchDoc represents structured statute citations in judgments.
type JudgmentStatuteSearchDoc struct {
	StatuteName   string   `json:"statute_name"`
	SectionsCited []string `json:"sections_cited"`
}

// JudgmentPageSearchDoc represents extracted text per PDF page.
type JudgmentPageSearchDoc struct {
	PageNumber int    `json:"page_number"`
	Text       string `json:"text"`
}

// JudgmentSearchDoc represents a court judgment for search indexing.
type JudgmentSearchDoc struct {
	ID                 string                     `json:"id"`
	Citation           string                     `json:"citation"`
	CitationYear       int                        `json:"citation_year"`
	CitationCourtCode  string                     `json:"citation_court_code"`
	CitationNumber     int                        `json:"citation_number"`
	CaseTitle          string                     `json:"case_title"`
	CaseNumber         string                     `json:"case_number"`
	Court              string                     `json:"court"`
	CourtCode          string                     `json:"court_code"`
	Bench              []string                   `json:"bench"`
	JudgmentDate       time.Time                  `json:"judgment_date"`
	Appellant          string                     `json:"appellant"`
	Respondent         string                     `json:"respondent"`
	Headnote           string                     `json:"headnote"`
	Summary            string                     `json:"summary"`
	Held               string                     `json:"held"`
	Statutes           []JudgmentStatuteSearchDoc `json:"statutes"`
	Pages              []JudgmentPageSearchDoc    `json:"pages"`
	AccessLevel        string                     `json:"access_level"`
	PublishedAt        time.Time                  `json:"published_at"`
}

// CaseSearchDoc represents a court hearing for search indexing.
type CaseSearchDoc struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	CourtName          string    `json:"court_name"`
	CaseNumber         string    `json:"case_number"`
	CaseType           string    `json:"case_type"`
	PresidingJudge     string    `json:"presiding_judge"`
	BenchJudges        []string  `json:"bench_judges"`
	PetitionerParties  []string  `json:"petitioner_parties"`
	RespondentParties  []string  `json:"respondent_parties"`
	IsLive             bool      `json:"is_live"`
	Status             string    `json:"status"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// SearchIndexStats contains index sizing and health metrics for the CMS admin dashboard.
type SearchIndexStats struct {
	TotalDocuments int64      `json:"total_documents"`
	ArticleCount   int64      `json:"article_count"`
	JudgmentCount  int64      `json:"judgment_count"`
	CaseCount      int64      `json:"case_count"`
	Engine         string     `json:"engine"`
	Status         string     `json:"status"`
	LastSyncAt     *time.Time `json:"last_sync_at,omitempty"`
}

// SearchIndex defines the port for search engine operations.
type SearchIndex interface {
	IndexArticle(ctx context.Context, doc ArticleSearchDoc) error
	DeleteArticle(ctx context.Context, id string) error
	IndexJudgment(ctx context.Context, doc JudgmentSearchDoc) error
	DeleteJudgment(ctx context.Context, id string) error
	IndexCourtCase(ctx context.Context, doc CaseSearchDoc) error
	DeleteCourtCase(ctx context.Context, id string) error
	Search(ctx context.Context, query search.SearchQuery, isEntitled bool) (*search.SearchResult, error)
	Autosuggest(ctx context.Context, prefix string, limit int) (*search.AutosuggestResult, error)
	GetStats(ctx context.Context) (*SearchIndexStats, error)
	Reset(ctx context.Context) error
}

// QueryCount models query aggregation.
type QueryCount struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}

// ZeroResultQuery models queries that returned zero hits.
type ZeroResultQuery struct {
	Query      string    `json:"query"`
	SearchedAt time.Time `json:"searched_at"`
	HasFilters bool      `json:"has_filters"`
}

// SearchAnalyticsRepository defines the port for query logging and metrics.
type SearchAnalyticsRepository interface {
	RecordQuery(ctx context.Context, query string, hitCount int, ipHash string, hasFilters bool) error
	GetTopQueries(ctx context.Context, limit int) ([]QueryCount, error)
	GetZeroResultQueries(ctx context.Context, limit int) ([]ZeroResultQuery, error)
	GetTotalSearches(ctx context.Context) (int64, error)
}
