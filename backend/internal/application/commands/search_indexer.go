package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
	"github.com/livelaw/backend/internal/domain/judgment"
)

// SearchIndexer synchronizes domain events from the transactional outbox into the SearchIndex.
type SearchIndexer struct {
	searchIndex  ports.SearchIndex
	articleRepo  ports.ArticleRepository
	judgmentRepo ports.JudgmentRepository
	caseRepo     ports.CourtCaseRepository
	sectionRepo  ports.SectionRepository
	logger       *slog.Logger
}

// NewSearchIndexer creates a new SearchIndexer instance.
func NewSearchIndexer(
	searchIndex ports.SearchIndex,
	articleRepo ports.ArticleRepository,
	judgmentRepo ports.JudgmentRepository,
	caseRepo ports.CourtCaseRepository,
	sectionRepo ports.SectionRepository,
	logger *slog.Logger,
) *SearchIndexer {
	return &SearchIndexer{
		searchIndex:  searchIndex,
		articleRepo:  articleRepo,
		judgmentRepo: judgmentRepo,
		caseRepo:     caseRepo,
		sectionRepo:  sectionRepo,
		logger:       logger,
	}
}

// Publish implements ports.EventPublisher to handle outbox events for search indexing.
func (s *SearchIndexer) Publish(ctx context.Context, event ports.OutboxEvent) error {
	switch event.EventType {
	case "article.published", "article.updated":
		return s.indexArticleByID(ctx, event.AggregateID)
	case "article.archived", "article.deleted":
		return s.searchIndex.DeleteArticle(ctx, event.AggregateID)
	case "judgment.published", "judgment.updated", "judgment.pages_extracted":
		return s.indexJudgmentByID(ctx, event.AggregateID)
	case "case.created", "case.updated", "case.live_status_changed":
		return s.indexCaseByID(ctx, event.AggregateID)
	default:
		// Not a search indexable event; ignore cleanly
		return nil
	}
}

func (s *SearchIndexer) indexArticleByID(ctx context.Context, articleID string) error {
	art, err := s.articleRepo.GetByID(ctx, articleID)
	if err != nil {
		s.logger.Warn("failed to fetch article for indexing", slog.String("id", articleID), slog.Any("error", err))
		return nil
	}

	if art.Status != content.StatusPublished || art.DeletedAt != nil {
		return s.searchIndex.DeleteArticle(ctx, articleID)
	}

	var sectionSlug, sectionName string
	if art.PrimarySectionID != nil && s.sectionRepo != nil {
		sec, err := s.sectionRepo.GetByID(ctx, *art.PrimarySectionID)
		if err == nil && sec != nil {
			sectionSlug = sec.Slug
			sectionName = sec.Name
		}
	}

	var subtitle string
	if art.Subtitle != nil {
		subtitle = *art.Subtitle
	}

	pubAt := art.CreatedAt
	if art.PublishedAt != nil {
		pubAt = *art.PublishedAt
	}

	doc := ports.ArticleSearchDoc{
		ID:          art.ID,
		Title:       art.Title,
		Subtitle:    subtitle,
		Summary:     art.Summary,
		BodyText:    string(art.BodyBlocks),
		Slug:        art.Slug,
		ArticleType: string(art.ArticleType),
		SectionSlug: sectionSlug,
		SectionName: sectionName,
		IsPremium:   art.AccessLevel == content.AccessLevelPremium,
		PublishedAt: pubAt,
	}

	return s.searchIndex.IndexArticle(ctx, doc)
}

func (s *SearchIndexer) indexJudgmentByID(ctx context.Context, judgmentID string) error {
	j, err := s.judgmentRepo.GetByID(ctx, judgmentID)
	if err != nil {
		s.logger.Warn("failed to fetch judgment for indexing", slog.String("id", judgmentID), slog.Any("error", err))
		return nil
	}

	if j.Status != judgment.StatusPublished {
		return s.searchIndex.DeleteJudgment(ctx, judgmentID)
	}

	statutes, _ := s.judgmentRepo.GetStatutes(ctx, judgmentID)
	pages, _ := s.judgmentRepo.GetPages(ctx, judgmentID)

	statuteDocs := make([]ports.JudgmentStatuteSearchDoc, 0, len(statutes))
	for _, st := range statutes {
		statuteDocs = append(statuteDocs, ports.JudgmentStatuteSearchDoc{
			StatuteName:   st.StatuteName,
			SectionsCited: st.SectionsCited,
		})
	}

	pageDocs := make([]ports.JudgmentPageSearchDoc, 0, len(pages))
	for _, p := range pages {
		pageDocs = append(pageDocs, ports.JudgmentPageSearchDoc{
			PageNumber: p.PageNumber,
			Text:       p.TextContent,
		})
	}

	pubAt := j.CreatedAt
	if j.PublishedAt != nil {
		pubAt = *j.PublishedAt
	}

	citation := ""
	if j.Citation != nil {
		citation = *j.Citation
	}
	citationYear := 0
	if j.CitationYear != nil {
		citationYear = *j.CitationYear
	}
	citationCourtCode := ""
	if j.CitationCourtCode != nil {
		citationCourtCode = *j.CitationCourtCode
	}
	citationNumber := 0
	if j.CitationNumber != nil {
		citationNumber = *j.CitationNumber
	}

	doc := ports.JudgmentSearchDoc{
		ID:                j.ID,
		Citation:          citation,
		CitationYear:      citationYear,
		CitationCourtCode: citationCourtCode,
		CitationNumber:    citationNumber,
		CaseTitle:         j.CaseTitle,
		CaseNumber:        j.CaseNumber,
		Court:             j.Court,
		CourtCode:         j.CourtCode,
		Bench:             j.Bench,
		JudgmentDate:      j.JudgmentDate,
		Appellant:         j.Appellant,
		Respondent:        j.Respondent,
		Headnote:          j.Headnote,
		Summary:           j.Summary,
		Held:              j.Held,
		Statutes:          statuteDocs,
		Pages:             pageDocs,
		AccessLevel:       string(j.AccessLevel),
		PublishedAt:       pubAt,
	}

	return s.searchIndex.IndexJudgment(ctx, doc)
}

func (s *SearchIndexer) indexCaseByID(ctx context.Context, caseID string) error {
	c, err := s.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		s.logger.Warn("failed to fetch court case for indexing", slog.String("id", caseID), slog.Any("error", err))
		return nil
	}

	doc := ports.CaseSearchDoc{
		ID:                c.ID,
		Title:             c.Title,
		CourtName:         c.CourtName,
		CaseNumber:        c.CaseNumber,
		CaseType:          c.CaseType,
		PresidingJudge:    c.PresidingJudge,
		BenchJudges:       c.BenchJudges,
		PetitionerParties: c.PetitionerParties,
		RespondentParties: c.RespondentParties,
		IsLive:            c.IsLive,
		Status:            string(c.Status),
		UpdatedAt:         c.UpdatedAt,
	}

	return s.searchIndex.IndexCourtCase(ctx, doc)
}

// ReindexAll flushes and completely rebuilds the search index from the database.
func (s *SearchIndexer) ReindexAll(ctx context.Context) (*ports.SearchIndexStats, error) {
	s.logger.Info("initiating full search re-index")

	if err := s.searchIndex.Reset(ctx); err != nil {
		return nil, fmt.Errorf("failed to reset search index: %w", err)
	}

	// 1. Re-index published articles
	pubStatus := content.StatusPublished
	articles, _, err := s.articleRepo.ListCMS(ctx, ports.ArticleFilter{
		Status: &pubStatus,
		Limit:  1000,
	})
	if err != nil {
		s.logger.Warn("reindex: error querying articles", slog.Any("error", err))
	} else {
		for _, art := range articles {
			_ = s.indexArticleByID(ctx, art.ID)
		}
	}

	// 2. Re-index published judgments
	jPubStatus := judgment.StatusPublished
	judgments, _, err := s.judgmentRepo.List(ctx, ports.JudgmentFilter{
		Status: &jPubStatus,
		Limit:  1000,
	})
	if err != nil {
		s.logger.Warn("reindex: error querying judgments", slog.Any("error", err))
	} else {
		for _, j := range judgments {
			_ = s.indexJudgmentByID(ctx, j.ID)
		}
	}

	// 3. Re-index court cases
	cases, err := s.caseRepo.ListAll(ctx, 1000)
	if err != nil {
		s.logger.Warn("reindex: error querying court cases", slog.Any("error", err))
	} else {
		for _, c := range cases {
			_ = s.indexCaseByID(ctx, c.ID)
		}
	}

	stats, err := s.searchIndex.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	s.logger.Info("full search re-index completed successfully",
		slog.Int64("total_docs", stats.TotalDocuments),
		slog.Int64("articles", stats.ArticleCount),
		slog.Int64("judgments", stats.JudgmentCount),
		slog.Int64("cases", stats.CaseCount),
	)

	return stats, nil
}
