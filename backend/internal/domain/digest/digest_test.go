package digest_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/livelaw/backend/internal/domain/digest"
)

func TestBuildNominalIndex_Alphabetical(t *testing.T) {
	cit1 := 10
	cit2 := 5
	cit3 := 20
	items := []digest.DigestItem{
		{
			JudgmentID:     "j-1",
			CaseTitle:      "Zeta Industries v. Union of India",
			Citation:       "2026 LiveLaw (SC) 10",
			CitationNumber: &cit1,
			JudgmentDate:   "2026-01-15",
		},
		{
			JudgmentID:     "j-2",
			CaseTitle:      "Alpha Logistics v. State of Delhi",
			Citation:       "2026 LiveLaw (SC) 5",
			CitationNumber: &cit2,
			JudgmentDate:   "2026-01-10",
		},
		{
			JudgmentID:     "j-3",
			CaseTitle:      "Beta Traders v. Commissioner",
			Citation:       "2026 LiveLaw (SC) 20",
			CitationNumber: &cit3,
			JudgmentDate:   "2026-01-20",
		},
	}

	index := digest.BuildNominalIndex(items)
	if len(index) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(index))
	}

	// Should be Alpha, Beta, Zeta
	if index[0].CaseTitle != "Alpha Logistics v. State of Delhi" {
		t.Errorf("expected Alpha first, got %s", index[0].CaseTitle)
	}
	if index[1].CaseTitle != "Beta Traders v. Commissioner" {
		t.Errorf("expected Beta second, got %s", index[1].CaseTitle)
	}
	if index[2].CaseTitle != "Zeta Industries v. Union of India" {
		t.Errorf("expected Zeta third, got %s", index[2].CaseTitle)
	}
}

func TestBuildSubjectIndex_Grouping(t *testing.T) {
	items := []digest.DigestItem{
		{
			JudgmentID: "j-1",
			CaseTitle:  "Alpha v. State",
			SubjectTag: "Arbitration",
		},
		{
			JudgmentID: "j-2",
			CaseTitle:  "Beta v. State",
			SubjectTag: "Criminal Law - Bail",
		},
		{
			JudgmentID: "j-3",
			CaseTitle:  "Gamma v. State",
			SubjectTag: "Arbitration",
		},
	}

	groups := digest.BuildSubjectIndex(items)
	if len(groups) != 2 {
		t.Fatalf("expected 2 subject groups, got %d", len(groups))
	}

	if groups[0].Subject != "Arbitration" {
		t.Errorf("expected Arbitration first alphabetically, got %s", groups[0].Subject)
	}
	if len(groups[0].Judgments) != 2 {
		t.Errorf("expected 2 judgments in Arbitration, got %d", len(groups[0].Judgments))
	}

	if groups[1].Subject != "Criminal Law - Bail" {
		t.Errorf("expected Criminal Law second, got %s", groups[1].Subject)
	}
	if len(groups[1].Judgments) != 1 {
		t.Errorf("expected 1 judgment in Criminal Law, got %d", len(groups[1].Judgments))
	}
}

func TestBuildStatuteIndex_SectionBreakdown(t *testing.T) {
	items := []digest.DigestItem{
		{
			JudgmentID:    "j-1",
			CaseTitle:     "Cheque Bounce Case A",
			StatutesCited: []string{"Negotiable Instruments Act 1881 - Section 138"},
		},
		{
			JudgmentID:    "j-2",
			CaseTitle:     "Cheque Bounce Case B",
			StatutesCited: []string{"Negotiable Instruments Act 1881 - Section 138", "Negotiable Instruments Act 1881 - Section 141"},
		},
		{
			JudgmentID:    "j-3",
			CaseTitle:     "Arbitration Appointment",
			StatutesCited: []string{"Arbitration and Conciliation Act 1996 - Section 11"},
		},
	}

	statuteGroups := digest.BuildStatuteIndex(items)
	if len(statuteGroups) != 3 {
		t.Fatalf("expected 3 statute groups, got %d", len(statuteGroups))
	}

	// 1. Arbitration Section 11
	if statuteGroups[0].StatuteName != "Arbitration and Conciliation Act 1996" || statuteGroups[0].SectionCited != "Section 11" {
		t.Errorf("unexpected first statute group: %+v", statuteGroups[0])
	}
	if len(statuteGroups[0].Judgments) != 1 {
		t.Errorf("expected 1 judgment in Arbitration sec 11, got %d", len(statuteGroups[0].Judgments))
	}

	// 2. NI Act Section 138 (2 judgments)
	if statuteGroups[1].StatuteName != "Negotiable Instruments Act 1881" || statuteGroups[1].SectionCited != "Section 138" {
		t.Errorf("unexpected second statute group: %+v", statuteGroups[1])
	}
	if len(statuteGroups[1].Judgments) != 2 {
		t.Errorf("expected 2 judgments in NI Act sec 138, got %d", len(statuteGroups[1].Judgments))
	}

	// 3. NI Act Section 141 (1 judgment)
	if statuteGroups[2].StatuteName != "Negotiable Instruments Act 1881" || statuteGroups[2].SectionCited != "Section 141" {
		t.Errorf("unexpected third statute group: %+v", statuteGroups[2])
	}
}

func TestSplitIntoParts_BoundaryRanges(t *testing.T) {
	items := make([]digest.DigestItem, 0, 10)
	for i := 1; i <= 10; i++ {
		citNum := i
		items = append(items, digest.DigestItem{
			JudgmentID:     fmt.Sprintf("j-%d", i),
			CaseTitle:      fmt.Sprintf("Case %d", i),
			Citation:       fmt.Sprintf("2026 LiveLaw (SC) %d", i),
			CitationNumber: &citNum,
		})
	}

	d := &digest.Digest{
		ID:        "digest-parent",
		Title:     "Supreme Court Annual Digest 2026",
		Slug:      "supreme-court-annual-digest-2026",
		Court:     "Supreme Court of India",
		CourtCode: "SC",
		Period:    digest.PeriodAnnual,
		Items:     items,
	}

	idCounter := 100
	idGen := func() string {
		idCounter++
		return fmt.Sprintf("digest-part-%d", idCounter)
	}

	// Split into 3 parts: items 1..4 (Part 1), 5..8 (Part 2), 9..10 (Part 3)
	parts, err := digest.SplitIntoParts(d, 3, idGen)
	if err != nil {
		t.Fatalf("failed splitting into parts: %v", err)
	}

	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}

	// Part 1: Citations 1 to 4
	if parts[0].PartNumber != 1 || parts[0].TotalParts != 3 {
		t.Errorf("part 1 metadata wrong: part %d of %d", parts[0].PartNumber, parts[0].TotalParts)
	}
	if *parts[0].CitationRangeStart != 1 || *parts[0].CitationRangeEnd != 4 {
		t.Errorf("part 1 citation range wrong: %d to %d", *parts[0].CitationRangeStart, *parts[0].CitationRangeEnd)
	}
	if len(parts[0].Items) != 4 {
		t.Errorf("part 1 items count wrong: %d", len(parts[0].Items))
	}

	// Part 2: Citations 5 to 8
	if parts[1].PartNumber != 2 || parts[1].TotalParts != 3 {
		t.Errorf("part 2 metadata wrong")
	}
	if *parts[1].CitationRangeStart != 5 || *parts[1].CitationRangeEnd != 8 {
		t.Errorf("part 2 citation range wrong: %d to %d", *parts[1].CitationRangeStart, *parts[1].CitationRangeEnd)
	}

	// Part 3: Citations 9 to 10
	if parts[2].PartNumber != 3 || parts[2].TotalParts != 3 {
		t.Errorf("part 3 metadata wrong")
	}
	if *parts[2].CitationRangeStart != 9 || *parts[2].CitationRangeEnd != 10 {
		t.Errorf("part 3 citation range wrong: %d to %d", *parts[2].CitationRangeStart, *parts[2].CitationRangeEnd)
	}
}

func TestDigestValidation(t *testing.T) {
	now := time.Now()
	citStart := 50
	citEnd := 10

	d := &digest.Digest{
		Title:              "",
		CourtCode:          "SC",
		PeriodStart:        now,
		PeriodEnd:          now.Add(24 * time.Hour),
		CitationRangeStart: &citStart,
		CitationRangeEnd:   &citEnd,
	}

	if err := d.Validate(); err == nil {
		t.Errorf("expected error for empty title")
	}

	d.Title = "Valid Title"
	if err := d.Validate(); err != digest.ErrCitationRangeInvalid {
		t.Errorf("expected ErrCitationRangeInvalid, got: %v", err)
	}
}

func TestGenerateSlug(t *testing.T) {
	slug := digest.GenerateSlug("Supreme Court Monthly Digest: January 2026!")
	if !strings.Contains(slug, "supreme-court-monthly-digest-january-2026") {
		t.Errorf("unexpected slug: %s", slug)
	}
}
