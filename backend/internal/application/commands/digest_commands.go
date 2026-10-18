package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
	"github.com/livelaw/backend/internal/domain/digest"
	"github.com/livelaw/backend/internal/domain/judgment"
)

var (
	ErrDigestNotFound = errors.New("digest not found")
)

// DigestCommands coordinates digest lifecycle, auto-assembly, and publishing.
type DigestCommands struct {
	digestRepo   ports.DigestRepository
	judgmentRepo ports.JudgmentRepository
	articleRepo  ports.ArticleRepository
	outboxRepo   ports.OutboxRepository
	idGen        ports.IDGenerator
	clock        ports.Clock
}

// NewDigestCommands creates a new DigestCommands handler.
func NewDigestCommands(
	digestRepo ports.DigestRepository,
	judgmentRepo ports.JudgmentRepository,
	articleRepo ports.ArticleRepository,
	outboxRepo ports.OutboxRepository,
	idGen ports.IDGenerator,
	clock ports.Clock,
) *DigestCommands {
	return &DigestCommands{
		digestRepo:   digestRepo,
		judgmentRepo: judgmentRepo,
		articleRepo:  articleRepo,
		outboxRepo:   outboxRepo,
		idGen:        idGen,
		clock:        clock,
	}
}

// CreateDigestInput holds arguments for creating a new digest.
type CreateDigestInput struct {
	Title              string              `json:"title"`
	Court              string              `json:"court"`
	CourtCode          string              `json:"court_code"`
	DigestType         digest.DigestType   `json:"digest_type"`
	Period             digest.DigestPeriod `json:"period"`
	PeriodStart        time.Time           `json:"period_start"`
	PeriodEnd          time.Time           `json:"period_end"`
	CitationRangeStart *int                `json:"citation_range_start,omitempty"`
	CitationRangeEnd   *int                `json:"citation_range_end,omitempty"`
	CitationPrefix     string              `json:"citation_prefix,omitempty"`
	Intro              string              `json:"intro,omitempty"`
	Conclusion         string              `json:"conclusion,omitempty"`
	AccessLevel        digest.AccessLevel  `json:"access_level"`
	CreatedBy          string              `json:"created_by"`
}

// Create creates a new digest draft.
func (c *DigestCommands) Create(ctx context.Context, input CreateDigestInput) (*digest.Digest, error) {
	now := c.clock.Now().UTC()
	slug := digest.GenerateSlug(input.Title)

	d := &digest.Digest{
		ID:                 c.idGen.NewID(),
		Title:              strings.TrimSpace(input.Title),
		Slug:               slug,
		Court:              strings.TrimSpace(input.Court),
		CourtCode:          strings.ToUpper(strings.TrimSpace(input.CourtCode)),
		DigestType:         input.DigestType,
		Period:             input.Period,
		PeriodStart:        input.PeriodStart,
		PeriodEnd:          input.PeriodEnd,
		CitationRangeStart: input.CitationRangeStart,
		CitationRangeEnd:   input.CitationRangeEnd,
		CitationPrefix:     strings.TrimSpace(input.CitationPrefix),
		IsPart:             false,
		PartNumber:         1,
		TotalParts:         1,
		Intro:              input.Intro,
		Conclusion:         input.Conclusion,
		Status:             digest.StatusDraft,
		AccessLevel:        input.AccessLevel,
		CreatedBy:          &input.CreatedBy,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := d.Validate(); err != nil {
		return nil, err
	}

	if err := c.digestRepo.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("failed to create digest: %w", err)
	}

	return d, nil
}

// AutoAssemble populates digest items by querying judgments matching the court and citation/period range.
func (c *DigestCommands) AutoAssemble(ctx context.Context, digestID string) (*digest.Digest, error) {
	d, err := c.digestRepo.GetByID(ctx, digestID)
	if err != nil {
		return nil, err
	}

	// Fetch published judgments for court
	filterCourt := d.CourtCode
	publishedStatus := judgment.StatusPublished
	jFilter := ports.JudgmentFilter{
		CourtCode: &filterCourt,
		Status:    &publishedStatus,
		Limit:     500,
	}

	judgments, _, err := c.judgmentRepo.List(ctx, jFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to list judgments for auto-assembly: %w", err)
	}

	// Filter judgments by date range and citation range
	var matchedJudgments []judgment.Judgment
	for _, j := range judgments {
		// Date filtering
		jDate := j.JudgmentDate
		if jDate.Before(d.PeriodStart) || jDate.After(d.PeriodEnd) {
			continue
		}

		// Citation number filtering if range is specified
		if j.CitationNumber != nil {
			num := *j.CitationNumber
			if d.CitationRangeStart != nil && num < *d.CitationRangeStart {
				continue
			}
			if d.CitationRangeEnd != nil && num > *d.CitationRangeEnd {
				continue
			}
		}

		matchedJudgments = append(matchedJudgments, j)
	}

	now := c.clock.Now().UTC()
	items := make([]digest.DigestItem, 0, len(matchedJudgments))
	var minCit, maxCit *int

	for i, j := range matchedJudgments {
		subject := "General"
		if len(j.SubjectTags) > 0 {
			subject = j.SubjectTags[0]
		}
		blurb := j.Summary
		if blurb == "" {
			blurb = j.Held
		}

		if j.CitationNumber != nil {
			num := *j.CitationNumber
			if minCit == nil || num < *minCit {
				s := num
				minCit = &s
			}
			if maxCit == nil || num > *maxCit {
				e := num
				maxCit = &e
			}
		}

		citStr := ""
		if j.Citation != nil {
			citStr = *j.Citation
		}

		item := digest.DigestItem{
			ID:             c.idGen.NewID(),
			DigestID:       d.ID,
			JudgmentID:     j.ID,
			DisplayOrder:   i + 1,
			CustomTitle:    j.CaseTitle,
			Blurb:          blurb,
			Commentary:     j.Headnote,
			SubjectTag:     subject,
			CreatedAt:      now,
			CaseTitle:      j.CaseTitle,
			Citation:       citStr,
			CitationNumber: j.CitationNumber,
			JudgmentDate:   j.JudgmentDate.Format("2006-01-02"),
			Bench:          j.Bench,
			Headnote:       j.Headnote,
			Held:           j.Held,
		}
		items = append(items, item)
	}

	if d.CitationRangeStart == nil && minCit != nil {
		d.CitationRangeStart = minCit
	}
	if d.CitationRangeEnd == nil && maxCit != nil {
		d.CitationRangeEnd = maxCit
	}
	d.UpdatedAt = now

	if err := c.digestRepo.Update(ctx, d); err != nil {
		return nil, fmt.Errorf("failed to update digest metadata: %w", err)
	}
	if err := c.digestRepo.SaveItems(ctx, d.ID, items); err != nil {
		return nil, fmt.Errorf("failed to save auto-assembled items: %w", err)
	}

	d.Items = items
	return d, nil
}

// SplitIntoParts splits a digest into N distinct sequential parts with contiguous citation ranges.
func (c *DigestCommands) SplitIntoParts(ctx context.Context, digestID string, numParts int) ([]*digest.Digest, error) {
	d, err := c.digestRepo.GetByID(ctx, digestID)
	if err != nil {
		return nil, err
	}

	parts, err := digest.SplitIntoParts(d, numParts, c.idGen.NewID)
	if err != nil {
		return nil, err
	}

	createdParts := make([]*digest.Digest, 0, len(parts))
	for _, p := range parts {
		if err := c.digestRepo.Create(ctx, p); err != nil {
			return nil, fmt.Errorf("failed to persist digest part %d: %w", p.PartNumber, err)
		}
		createdParts = append(createdParts, p)
	}

	// Archive or mark original as split
	d.Status = digest.StatusArchived
	d.UpdatedAt = c.clock.Now().UTC()
	_ = c.digestRepo.Update(ctx, d)

	return createdParts, nil
}

// Publish creates an Article of type 'digest' with full index and judgment commentary, and marks digest as published.
func (c *DigestCommands) Publish(ctx context.Context, digestID string, actorID string) (*digest.Digest, *content.Article, error) {
	d, err := c.digestRepo.GetByID(ctx, digestID)
	if err != nil {
		return nil, nil, err
	}

	if len(d.Items) == 0 {
		return nil, nil, digest.ErrDigestEmpty
	}

	now := c.clock.Now().UTC()

	// 1. Build Assembled Indexes
	nominal := digest.BuildNominalIndex(d.Items)
	subjects := digest.BuildSubjectIndex(d.Items)
	statutes := digest.BuildStatuteIndex(d.Items)

	// 2. Format Rich Text TipTap Body Blocks
	blocks := buildDigestBodyBlocks(d, nominal, subjects, statutes)
	bodyJSON, err := json.Marshal(blocks)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal body blocks: %w", err)
	}

	summary := fmt.Sprintf("%s comprehensive digest covering %d landmark rulings from %s to %s.",
		d.Court, len(d.Items), d.PeriodStart.Format("02 Jan 2006"), d.PeriodEnd.Format("02 Jan 2006"))
	if d.Intro != "" {
		summary = d.Intro
	}

	accessLevel := content.AccessLevelPremium
	if d.AccessLevel == digest.AccessLevelFree {
		accessLevel = content.AccessLevelFree
	}

	artType := content.ArticleTypeDigest
	if d.DigestType == digest.DigestTypeRoundUp {
		artType = content.ArticleTypeRoundUp
	}

	artID := c.idGen.NewID()
	if d.ArticleID != nil && *d.ArticleID != "" {
		artID = *d.ArticleID
	}

	art := &content.Article{
		ID:                 artID,
		Slug:               d.Slug,
		Title:              d.Title,
		Summary:            summary,
		BodyBlocks:         bodyJSON,
		ArticleType:        artType,
		AccessLevel:        accessLevel,
		Status:             content.StatusPublished,
		AuthorID:           actorID,
		Category:           d.Court,
		Tags:               []string{"digest", strings.ToLower(d.CourtCode), string(d.Period)},
		WordCount:          len(d.Items) * 120,
		ReadingTimeMinutes: (len(d.Items) * 120 / 200) + 1,
		PublishedAt:        &now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Check if article already exists
	existingArt, _ := c.articleRepo.GetByID(ctx, artID)
	if existingArt == nil {
		if err := c.articleRepo.Create(ctx, art); err != nil {
			return nil, nil, fmt.Errorf("failed to create digest article: %w", err)
		}
	} else {
		art.CreatedAt = existingArt.CreatedAt
		art.Version = existingArt.Version
		if err := c.articleRepo.Update(ctx, art, existingArt.Version); err != nil {
			return nil, nil, fmt.Errorf("failed to update digest article: %w", err)
		}
	}

	// 3. Update Digest state
	d.ArticleID = &art.ID
	d.Status = digest.StatusPublished
	d.PublishedAt = &now
	d.UpdatedAt = now

	if err := c.digestRepo.Update(ctx, d); err != nil {
		return nil, nil, fmt.Errorf("failed to update digest to published: %w", err)
	}

	// 4. Emit outbox event for search indexing
	if c.outboxRepo != nil {
		payloadMap := map[string]interface{}{
			"article_id":   art.ID,
			"title":        art.Title,
			"slug":         art.Slug,
			"article_type": string(art.ArticleType),
			"published_at": now.Format(time.RFC3339),
		}
		payloadBytes, _ := json.Marshal(payloadMap)
		event := ports.OutboxEvent{
			ID:            c.idGen.NewID(),
			AggregateType: "article",
			AggregateID:   art.ID,
			EventType:     "article.published",
			Payload:       payloadBytes,
			SchemaVersion: 1,
			CreatedAt:     now.Format(time.RFC3339),
		}
		_ = c.outboxRepo.Save(ctx, &event)
	}

	return d, art, nil
}

// GenerateMonthlyDrafts generates monthly digest drafts for the prior month for all configured key courts.
func (c *DigestCommands) GenerateMonthlyDrafts(ctx context.Context, actorID string) ([]*digest.Digest, error) {
	now := c.clock.Now().UTC()
	// Previous month calculation
	firstOfCurrentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastOfPrevMonth := firstOfCurrentMonth.Add(-time.Second)
	firstOfPrevMonth := time.Date(lastOfPrevMonth.Year(), lastOfPrevMonth.Month(), 1, 0, 0, 0, 0, time.UTC)

	monthName := firstOfPrevMonth.Format("January 2006")

	courts := []struct {
		name string
		code string
	}{
		{"Supreme Court of India", "SC"},
		{"Delhi High Court", "DEL"},
		{"Bombay High Court", "BOM"},
	}

	var created []*digest.Digest
	for _, crt := range courts {
		title := fmt.Sprintf("%s Monthly Digest: %s", crt.name, monthName)
		input := CreateDigestInput{
			Title:          title,
			Court:          crt.name,
			CourtCode:      crt.code,
			DigestType:     digest.DigestTypeComprehensive,
			Period:         digest.PeriodMonthly,
			PeriodStart:    firstOfPrevMonth,
			PeriodEnd:      lastOfPrevMonth,
			CitationPrefix: fmt.Sprintf("%d LiveLaw (%s)", firstOfPrevMonth.Year(), crt.code),
			AccessLevel:    digest.AccessLevelPremium,
			CreatedBy:      actorID,
		}

		d, err := c.Create(ctx, input)
		if err != nil {
			continue // skip if already exists or invalid
		}

		// Run auto assembly
		assembled, err := c.AutoAssemble(ctx, d.ID)
		if err == nil {
			created = append(created, assembled)
		} else {
			created = append(created, d)
		}
	}

	return created, nil
}

type tipTapBlock struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content,omitempty"`
	Level   int         `json:"level,omitempty"`
}

func buildDigestBodyBlocks(
	d *digest.Digest,
	nominal []digest.NominalIndexEntry,
	subjects []digest.SubjectIndexGroup,
	statutes []digest.StatuteIndexGroup,
) []tipTapBlock {
	var blocks []tipTapBlock

	// 1. Title & Intro
	blocks = append(blocks, tipTapBlock{
		Type:  "heading",
		Level: 1,
		Content: fmt.Sprintf("%s (%s to %s)", d.Title,
			d.PeriodStart.Format("02 Jan 2006"), d.PeriodEnd.Format("02 Jan 2006")),
	})

	if d.Intro != "" {
		blocks = append(blocks, tipTapBlock{
			Type:    "paragraph",
			Content: d.Intro,
		})
	}

	// 2. Nominal Index
	blocks = append(blocks, tipTapBlock{
		Type:    "heading",
		Level:   2,
		Content: "I. Nominal Index (Alphabetical by Case Title)",
	})
	for _, n := range nominal {
		cit := n.Citation
		if cit == "" {
			cit = "Unreported"
		}
		blocks = append(blocks, tipTapBlock{
			Type:    "paragraph",
			Content: fmt.Sprintf("• %s — [%s] (Decided on %s)", n.CaseTitle, cit, n.Date),
		})
	}

	// 3. Subject Index
	blocks = append(blocks, tipTapBlock{
		Type:    "heading",
		Level:   2,
		Content: "II. Subject Index (Topic Breakdown)",
	})
	for _, s := range subjects {
		blocks = append(blocks, tipTapBlock{
			Type:    "heading",
			Level:   3,
			Content: fmt.Sprintf("Topic: %s (%d rulings)", s.Subject, len(s.Judgments)),
		})
		for _, j := range s.Judgments {
			blocks = append(blocks, tipTapBlock{
				Type:    "paragraph",
				Content: fmt.Sprintf("  - %s [%s]", j.CaseTitle, j.Citation),
			})
		}
	}

	// 4. Statute Index
	blocks = append(blocks, tipTapBlock{
		Type:    "heading",
		Level:   2,
		Content: "III. Statute Index (Acts & Sections Cited)",
	})
	for _, st := range statutes {
		blocks = append(blocks, tipTapBlock{
			Type:    "heading",
			Level:   3,
			Content: fmt.Sprintf("%s — %s", st.StatuteName, st.SectionCited),
		})
		for _, j := range st.Judgments {
			blocks = append(blocks, tipTapBlock{
				Type:    "paragraph",
				Content: fmt.Sprintf("  - %s [%s]", j.CaseTitle, j.Citation),
			})
		}
	}

	// 5. Detailed Case Summaries
	blocks = append(blocks, tipTapBlock{
		Type:    "heading",
		Level:   2,
		Content: "IV. Comprehensive Case Summaries & Headnotes",
	})
	for i, it := range d.Items {
		blocks = append(blocks, tipTapBlock{
			Type:    "heading",
			Level:   3,
			Content: fmt.Sprintf("%d. %s [%s]", i+1, it.CaseTitle, it.Citation),
		})
		if len(it.Bench) > 0 {
			blocks = append(blocks, tipTapBlock{
				Type:    "paragraph",
				Content: fmt.Sprintf("Coram: %s | Date: %s", strings.Join(it.Bench, ", "), it.JudgmentDate),
			})
		}
		if it.Headnote != "" {
			blocks = append(blocks, tipTapBlock{
				Type:    "paragraph",
				Content: fmt.Sprintf("Headnote: %s", it.Headnote),
			})
		}
		if it.Blurb != "" {
			blocks = append(blocks, tipTapBlock{
				Type:    "paragraph",
				Content: fmt.Sprintf("Held: %s", it.Blurb),
			})
		}
		if it.Commentary != "" && it.Commentary != it.Headnote {
			blocks = append(blocks, tipTapBlock{
				Type:    "paragraph",
				Content: fmt.Sprintf("Editorial Note: %s", it.Commentary),
			})
		}
	}

	if d.Conclusion != "" {
		blocks = append(blocks, tipTapBlock{
			Type:    "paragraph",
			Content: d.Conclusion,
		})
	}

	return blocks
}
