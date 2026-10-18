package digest

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidPeriod       = errors.New("invalid digest period")
	ErrInvalidDigestType   = errors.New("invalid digest type")
	ErrInvalidStatus       = errors.New("invalid digest status")
	ErrCitationRangeInvalid= errors.New("citation range start must be less than or equal to end")
	ErrInvalidPartCount    = errors.New("parts count must be greater than 1")
	ErrDigestEmpty         = errors.New("cannot publish an empty digest with no judgment items")
	ErrAlreadyPublished    = errors.New("digest is already published")
)

type DigestType string

const (
	DigestTypeComprehensive DigestType = "digest"
	DigestTypeRoundUp       DigestType = "round_up"
)

type DigestPeriod string

const (
	PeriodWeekly    DigestPeriod = "weekly"
	PeriodMonthly   DigestPeriod = "monthly"
	PeriodQuarterly DigestPeriod = "quarterly"
	PeriodAnnual    DigestPeriod = "annual"
	PeriodThematic  DigestPeriod = "thematic"
)

type DigestStatus string

const (
	StatusDraft     DigestStatus = "draft"
	StatusPublished DigestStatus = "published"
	StatusArchived  DigestStatus = "archived"
)

type AccessLevel string

const (
	AccessLevelFree    AccessLevel = "free"
	AccessLevelPremium AccessLevel = "premium"
)

// Digest represents a curated legal compilation of judgments over a specific citation range or timeframe.
type Digest struct {
	ID                 string       `json:"id"`
	Title              string       `json:"title"`
	Slug               string       `json:"slug"`
	Court              string       `json:"court"`
	CourtCode          string       `json:"court_code"`
	DigestType         DigestType   `json:"digest_type"`
	Period             DigestPeriod `json:"period"`
	PeriodStart        time.Time    `json:"period_start"`
	PeriodEnd          time.Time    `json:"period_end"`
	CitationRangeStart *int         `json:"citation_range_start,omitempty"`
	CitationRangeEnd   *int         `json:"citation_range_end,omitempty"`
	CitationPrefix     string       `json:"citation_prefix,omitempty"`
	IsPart             bool         `json:"is_part"`
	PartNumber         int          `json:"part_number"`
	TotalParts         int          `json:"total_parts"`
	Intro              string       `json:"intro,omitempty"`
	Conclusion         string       `json:"conclusion,omitempty"`
	ArticleID          *string      `json:"article_id,omitempty"`
	Status             DigestStatus `json:"status"`
	AccessLevel        AccessLevel  `json:"access_level"`
	CreatedBy          *string      `json:"created_by,omitempty"`
	PublishedAt        *time.Time   `json:"published_at,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
	Items              []DigestItem `json:"items,omitempty"`
}

// DigestItem represents a judgment entry within a digest.
type DigestItem struct {
	ID           string    `json:"id"`
	DigestID     string    `json:"digest_id"`
	JudgmentID   string    `json:"judgment_id"`
	DisplayOrder int       `json:"display_order"`
	CustomTitle  string    `json:"custom_title,omitempty"`
	Blurb        string    `json:"blurb,omitempty"`
	Commentary   string    `json:"commentary,omitempty"`
	SubjectTag   string    `json:"subject_tag,omitempty"`
	CreatedAt    time.Time `json:"created_at"`

	// Populated for rendering
	CaseTitle      string   `json:"case_title,omitempty"`
	Citation       string   `json:"citation,omitempty"`
	CitationNumber *int     `json:"citation_number,omitempty"`
	JudgmentDate   string   `json:"judgment_date,omitempty"`
	Bench          []string `json:"bench,omitempty"`
	StatutesCited  []string `json:"statutes_cited,omitempty"`
	Headnote       string   `json:"headnote,omitempty"`
	Held           string   `json:"held,omitempty"`
}

// NominalIndexEntry models a case in the alphabetical Nominal Index.
type NominalIndexEntry struct {
	CaseTitle      string `json:"case_title"`
	Citation       string `json:"citation"`
	JudgmentID     string `json:"judgment_id"`
	CitationNumber int    `json:"citation_number"`
	Date           string `json:"date"`
}

// SubjectIndexGroup models judgments organized under a legal topic.
type SubjectIndexGroup struct {
	Subject   string              `json:"subject"`
	Judgments []NominalIndexEntry `json:"judgments"`
}

// StatuteIndexGroup models judgments citing a specific act and section.
type StatuteIndexGroup struct {
	StatuteName  string              `json:"statute_name"`
	SectionCited string              `json:"section_cited"`
	Judgments    []NominalIndexEntry `json:"judgments"`
}

// AssembledIndexes bundles the 3 standard legal indexes.
type AssembledIndexes struct {
	NominalIndex []NominalIndexEntry `json:"nominal_index"`
	SubjectIndex []SubjectIndexGroup `json:"subject_index"`
	StatuteIndex []StatuteIndexGroup `json:"statute_index"`
}

// GenerateSlug produces a clean URL slug from title.
func GenerateSlug(title string) string {
	slug := strings.ToLower(title)
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 80 {
		slug = slug[:80]
		slug = strings.TrimRight(slug, "-")
	}
	return slug
}

// Validate validates digest properties.
func (d *Digest) Validate() error {
	if strings.TrimSpace(d.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(d.CourtCode) == "" {
		return errors.New("court code is required")
	}
	if d.PeriodStart.After(d.PeriodEnd) {
		return errors.New("period start cannot be after period end")
	}
	if d.CitationRangeStart != nil && d.CitationRangeEnd != nil {
		if *d.CitationRangeStart > *d.CitationRangeEnd {
			return ErrCitationRangeInvalid
		}
	}
	if d.DigestType == "" {
		d.DigestType = DigestTypeComprehensive
	}
	if d.Status == "" {
		d.Status = StatusDraft
	}
	if d.AccessLevel == "" {
		d.AccessLevel = AccessLevelPremium
	}
	return nil
}

// BuildNominalIndex creates an alphabetically sorted list by case title.
func BuildNominalIndex(items []DigestItem) []NominalIndexEntry {
	entries := make([]NominalIndexEntry, 0, len(items))
	for _, it := range items {
		title := it.CaseTitle
		if it.CustomTitle != "" {
			title = it.CustomTitle
		}
		num := 0
		if it.CitationNumber != nil {
			num = *it.CitationNumber
		}
		entries = append(entries, NominalIndexEntry{
			CaseTitle:      title,
			Citation:       it.Citation,
			JudgmentID:     it.JudgmentID,
			CitationNumber: num,
			Date:           it.JudgmentDate,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].CaseTitle) < strings.ToLower(entries[j].CaseTitle)
	})
	return entries
}

// BuildSubjectIndex groups judgments by subject tags.
func BuildSubjectIndex(items []DigestItem) []SubjectIndexGroup {
	groupMap := make(map[string][]NominalIndexEntry)

	for _, it := range items {
		tag := strings.TrimSpace(it.SubjectTag)
		if tag == "" {
			tag = "General / Miscellaneous"
		}
		title := it.CaseTitle
		if it.CustomTitle != "" {
			title = it.CustomTitle
		}
		num := 0
		if it.CitationNumber != nil {
			num = *it.CitationNumber
		}

		entry := NominalIndexEntry{
			CaseTitle:      title,
			Citation:       it.Citation,
			JudgmentID:     it.JudgmentID,
			CitationNumber: num,
			Date:           it.JudgmentDate,
		}
		groupMap[tag] = append(groupMap[tag], entry)
	}

	subjects := make([]string, 0, len(groupMap))
	for s := range groupMap {
		subjects = append(subjects, s)
	}
	sort.Strings(subjects)

	res := make([]SubjectIndexGroup, 0, len(subjects))
	for _, s := range subjects {
		judgs := groupMap[s]
		sort.Slice(judgs, func(i, j int) bool {
			return strings.ToLower(judgs[i].CaseTitle) < strings.ToLower(judgs[j].CaseTitle)
		})
		res = append(res, SubjectIndexGroup{
			Subject:   s,
			Judgments: judgs,
		})
	}
	return res
}

// BuildStatuteIndex groups judgments by act and section citations.
func BuildStatuteIndex(items []DigestItem) []StatuteIndexGroup {
	type key struct {
		statute string
		section string
	}
	groupMap := make(map[key][]NominalIndexEntry)

	for _, it := range items {
		title := it.CaseTitle
		if it.CustomTitle != "" {
			title = it.CustomTitle
		}
		num := 0
		if it.CitationNumber != nil {
			num = *it.CitationNumber
		}
		entry := NominalIndexEntry{
			CaseTitle:      title,
			Citation:       it.Citation,
			JudgmentID:     it.JudgmentID,
			CitationNumber: num,
			Date:           it.JudgmentDate,
		}

		if len(it.StatutesCited) == 0 {
			k := key{statute: "Constitution / Inherent Powers", section: "General"}
			groupMap[k] = append(groupMap[k], entry)
		} else {
			for _, st := range it.StatutesCited {
				parts := strings.Split(st, "-")
				statute := strings.TrimSpace(parts[0])
				section := "General"
				if len(parts) > 1 {
					section = strings.TrimSpace(parts[1])
				}
				k := key{statute: statute, section: section}
				groupMap[k] = append(groupMap[k], entry)
			}
		}
	}

	res := make([]StatuteIndexGroup, 0, len(groupMap))
	for k, judgs := range groupMap {
		sort.Slice(judgs, func(i, j int) bool {
			return strings.ToLower(judgs[i].CaseTitle) < strings.ToLower(judgs[j].CaseTitle)
		})
		res = append(res, StatuteIndexGroup{
			StatuteName:  k.statute,
			SectionCited: k.section,
			Judgments:    judgs,
		})
	}

	sort.Slice(res, func(i, j int) bool {
		if res[i].StatuteName == res[j].StatuteName {
			return res[i].SectionCited < res[j].SectionCited
		}
		return res[i].StatuteName < res[j].StatuteName
	})
	return res
}

// SplitIntoParts divides a digest's items into N parts evenly with continuous citation ranges.
func SplitIntoParts(d *Digest, numParts int, idGenerator func() string) ([]*Digest, error) {
	if numParts <= 1 {
		return nil, ErrInvalidPartCount
	}
	if len(d.Items) == 0 {
		return nil, ErrDigestEmpty
	}

	totalItems := len(d.Items)
	chunkSize := (totalItems + numParts - 1) / numParts

	parts := make([]*Digest, 0, numParts)
	for i := 0; i < numParts; i++ {
		startIdx := i * chunkSize
		if startIdx >= totalItems {
			break
		}
		endIdx := startIdx + chunkSize
		if endIdx > totalItems {
			endIdx = totalItems
		}

		chunkItems := d.Items[startIdx:endIdx]
		partNum := i + 1
		partTitle := fmt.Sprintf("%s (Part %d of %d)", d.Title, partNum, numParts)
		partSlug := fmt.Sprintf("%s-part-%d", d.Slug, partNum)

		var rangeStart, rangeEnd *int
		if len(chunkItems) > 0 {
			if chunkItems[0].CitationNumber != nil {
				s := *chunkItems[0].CitationNumber
				rangeStart = &s
			}
			if chunkItems[len(chunkItems)-1].CitationNumber != nil {
				e := *chunkItems[len(chunkItems)-1].CitationNumber
				rangeEnd = &e
			}
		}

		subItems := make([]DigestItem, len(chunkItems))
		copy(subItems, chunkItems)
		for j := range subItems {
			subItems[j].ID = idGenerator()
			subItems[j].DisplayOrder = j + 1
		}

		partDigest := &Digest{
			ID:                 idGenerator(),
			Title:              partTitle,
			Slug:               partSlug,
			Court:              d.Court,
			CourtCode:          d.CourtCode,
			DigestType:         d.DigestType,
			Period:             d.Period,
			PeriodStart:        d.PeriodStart,
			PeriodEnd:          d.PeriodEnd,
			CitationRangeStart: rangeStart,
			CitationRangeEnd:   rangeEnd,
			CitationPrefix:     d.CitationPrefix,
			IsPart:             true,
			PartNumber:         partNum,
			TotalParts:         numParts,
			Intro:              d.Intro,
			Conclusion:         d.Conclusion,
			Status:             StatusDraft,
			AccessLevel:        d.AccessLevel,
			CreatedBy:          d.CreatedBy,
			CreatedAt:          time.Now().UTC(),
			UpdatedAt:          time.Now().UTC(),
			Items:              subItems,
		}
		parts = append(parts, partDigest)
	}

	return parts, nil
}
