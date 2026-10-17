package judgment

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidCaseTitle     = errors.New("case title cannot be empty")
	ErrInvalidCaseNumber    = errors.New("case number cannot be empty")
	ErrInvalidCourt         = errors.New("court name cannot be empty")
	ErrInvalidAppellant     = errors.New("appellant cannot be empty")
	ErrInvalidRespondent    = errors.New("respondent cannot be empty")
	ErrInvalidSummary       = errors.New("judgment summary cannot be empty")
	ErrAlreadyPublished     = errors.New("judgment is already published")
	ErrInvalidPDFMagicBytes = errors.New("file is not a valid PDF document (missing %PDF- magic bytes)")
	ErrPDFTooLarge          = errors.New("PDF exceeds maximum allowed upload size")
	ErrDuplicateChecksum    = errors.New("a judgment with this identical PDF checksum already exists")
	ErrNoPDFAttached        = errors.New("cannot publish judgment without an attached official PDF")
)

type AccessLevel string

const (
	AccessLevelFree    AccessLevel = "free"
	AccessLevelPremium AccessLevel = "premium"
)

type JudgmentStatus string

const (
	StatusDraft     JudgmentStatus = "draft"
	StatusPublished JudgmentStatus = "published"
	StatusArchived  JudgmentStatus = "archived"
)

type ExtractionStatus string

const (
	ExtractionPending      ExtractionStatus = "pending"
	ExtractionProcessing   ExtractionStatus = "processing"
	ExtractionCompleted    ExtractionStatus = "completed"
	ExtractionOCRCompleted ExtractionStatus = "ocr_completed"
	ExtractionFailed       ExtractionStatus = "failed"
)

// StatuteCited represents a structured statute and section reference.
type StatuteCited struct {
	ID            string   `json:"id"`
	JudgmentID    string   `json:"judgment_id"`
	StatuteName   string   `json:"statute_name"`
	SectionsCited []string `json:"sections_cited"`
}

// JudgmentPage stores extracted page text and OCR flags.
type JudgmentPage struct {
	ID          string    `json:"id"`
	JudgmentID  string    `json:"judgment_id"`
	PageNumber  int       `json:"page_number"`
	TextContent string    `json:"text_content"`
	HasOCR      bool      `json:"has_ocr"`
	CreatedAt   time.Time `json:"created_at"`
}

// JudgmentDownload records subscriber/reader download audit logs.
type JudgmentDownload struct {
	ID           string    `json:"id"`
	JudgmentID   string    `json:"judgment_id"`
	UserID       *string   `json:"user_id,omitempty"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	DownloadedAt time.Time `json:"downloaded_at"`
	Watermarked  bool      `json:"watermarked"`
}

// CitationCounter holds the atomic sequential counter for a court and year.
type CitationCounter struct {
	CourtCode     string    `json:"court_code"`
	Year          int       `json:"year"`
	CurrentNumber int       `json:"current_number"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Judgment represents an official court judgment record with unique citation and headnotes.
type Judgment struct {
	ID                string           `json:"id"`
	Citation          *string          `json:"citation,omitempty"`
	CitationYear      *int             `json:"citation_year,omitempty"`
	CitationCourtCode *string          `json:"citation_court_code,omitempty"`
	CitationNumber    *int             `json:"citation_number,omitempty"`
	CaseTitle         string           `json:"case_title"`
	CaseNumber        string           `json:"case_number"`
	Court             string           `json:"court"`
	CourtCode         string           `json:"court_code"`
	Bench             []string         `json:"bench"`
	JudgmentDate      time.Time        `json:"judgment_date"`
	DecisionDate      *time.Time       `json:"decision_date,omitempty"`
	Appellant         string           `json:"appellant"`
	Respondent        string           `json:"respondent"`
	Advocates         []string         `json:"advocates"`
	StatutesCited     []StatuteCited   `json:"statutes_cited"`
	SubjectTags       []string         `json:"subject_tags"`
	Issues            string           `json:"issues"`
	Held              string           `json:"held"`
	Headnote          string           `json:"headnote"`
	Summary           string           `json:"summary"`
	KeyTakeaways      []string         `json:"key_takeaways"`
	CourtCaseID       *string          `json:"court_case_id,omitempty"`
	ArticleID         *string          `json:"article_id,omitempty"`
	AccessLevel       AccessLevel      `json:"access_level"`
	Status            JudgmentStatus   `json:"status"`
	PDFStorageKey     *string          `json:"pdf_storage_key,omitempty"`
	PDFFilename       *string          `json:"pdf_filename,omitempty"`
	PDFSizeBytes      *int64           `json:"pdf_size_bytes,omitempty"`
	PDFChecksum       *string          `json:"pdf_checksum,omitempty"`
	PDFPageCount      *int             `json:"pdf_page_count,omitempty"`
	PDFThumbnailKey   *string          `json:"pdf_thumbnail_key,omitempty"`
	PDFPreviewKey     *string          `json:"pdf_preview_key,omitempty"`
	ExtractionStatus  ExtractionStatus `json:"extraction_status"`
	CreatedBy         string           `json:"created_by"`
	PublishedAt       *time.Time       `json:"published_at,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

// NewJudgment initializes a new judgment draft with validation.
func NewJudgment(
	id string,
	caseTitle, caseNumber, court, courtCode string,
	bench []string,
	judgmentDate time.Time,
	decisionDate *time.Time,
	appellant, respondent string,
	advocates []string,
	statutes []StatuteCited,
	subjectTags []string,
	issues, held, headnote, summary string,
	keyTakeaways []string,
	courtCaseID, articleID *string,
	accessLevel AccessLevel,
	createdBy string,
	now time.Time,
) (*Judgment, error) {
	if strings.TrimSpace(caseTitle) == "" {
		return nil, ErrInvalidCaseTitle
	}
	if strings.TrimSpace(caseNumber) == "" {
		return nil, ErrInvalidCaseNumber
	}
	if strings.TrimSpace(court) == "" {
		return nil, ErrInvalidCourt
	}
	if strings.TrimSpace(appellant) == "" {
		return nil, ErrInvalidAppellant
	}
	if strings.TrimSpace(respondent) == "" {
		return nil, ErrInvalidRespondent
	}
	if strings.TrimSpace(summary) == "" {
		return nil, ErrInvalidSummary
	}

	normCourtCode := strings.ToUpper(strings.TrimSpace(courtCode))
	if normCourtCode == "" {
		normCourtCode = NormalizeCourtCode(court)
	}

	if accessLevel != AccessLevelPremium {
		accessLevel = AccessLevelFree
	}

	cleanBench := cleanStringSlice(bench)
	cleanAdvocates := cleanStringSlice(advocates)
	cleanTags := cleanStringSlice(subjectTags)
	cleanTakeaways := cleanStringSlice(keyTakeaways)

	return &Judgment{
		ID:               id,
		CaseTitle:        strings.TrimSpace(caseTitle),
		CaseNumber:       strings.TrimSpace(caseNumber),
		Court:            strings.TrimSpace(court),
		CourtCode:        normCourtCode,
		Bench:            cleanBench,
		JudgmentDate:     judgmentDate,
		DecisionDate:     decisionDate,
		Appellant:        strings.TrimSpace(appellant),
		Respondent:       strings.TrimSpace(respondent),
		Advocates:        cleanAdvocates,
		StatutesCited:    statutes,
		SubjectTags:      cleanTags,
		Issues:           strings.TrimSpace(issues),
		Held:             strings.TrimSpace(held),
		Headnote:         strings.TrimSpace(headnote),
		Summary:          strings.TrimSpace(summary),
		KeyTakeaways:     cleanTakeaways,
		CourtCaseID:      courtCaseID,
		ArticleID:        articleID,
		AccessLevel:      accessLevel,
		Status:           StatusDraft,
		ExtractionStatus: ExtractionPending,
		CreatedBy:        createdBy,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// Publish assigns the official atomic sequential citation number and transitions the status to published.
func (j *Judgment) Publish(
	brandPrefix string,
	citationYear int,
	courtCode string,
	citationNumber int,
	now time.Time,
) error {
	if j.Status == StatusPublished {
		return ErrAlreadyPublished
	}

	courtCode = strings.ToUpper(strings.TrimSpace(courtCode))
	if courtCode == "" {
		courtCode = j.CourtCode
	}

	formatted := FormatCitation(citationYear, brandPrefix, courtCode, citationNumber)
	j.Citation = &formatted
	j.CitationYear = &citationYear
	j.CitationCourtCode = &courtCode
	j.CitationNumber = &citationNumber
	j.Status = StatusPublished
	j.PublishedAt = &now
	j.UpdatedAt = now
	return nil
}

// AttachPDF validates and attaches an official uploaded PDF to the judgment.
func (j *Judgment) AttachPDF(
	storageKey, filename string,
	sizeBytes int64,
	checksum string,
	pageCount int,
	thumbnailKey, previewKey *string,
	now time.Time,
) error {
	if strings.TrimSpace(storageKey) == "" {
		return errors.New("PDF storage key cannot be empty")
	}
	if sizeBytes <= 0 {
		return errors.New("PDF size must be greater than zero")
	}
	if strings.TrimSpace(checksum) == "" {
		return errors.New("PDF checksum cannot be empty")
	}

	j.PDFStorageKey = &storageKey
	j.PDFFilename = &filename
	j.PDFSizeBytes = &sizeBytes
	j.PDFChecksum = &checksum
	j.PDFPageCount = &pageCount
	j.PDFThumbnailKey = thumbnailKey
	j.PDFPreviewKey = previewKey
	j.ExtractionStatus = ExtractionPending
	j.UpdatedAt = now
	return nil
}

// SetExtractionStatus updates the document text extraction/OCR pipeline state.
func (j *Judgment) SetExtractionStatus(status ExtractionStatus, now time.Time) {
	j.ExtractionStatus = status
	j.UpdatedAt = now
}

// FormatCitation generates standard unique legal citations: [YEAR] [BRAND PREFIX] ([COURT CODE]) [N].
// Example: 2026 CourtDesk (SC) 1
func FormatCitation(year int, brandPrefix, courtCode string, number int) string {
	cleanPrefix := strings.TrimSpace(brandPrefix)
	if cleanPrefix == "" {
		cleanPrefix = "CourtDesk"
	}
	cleanCourt := strings.ToUpper(strings.TrimSpace(courtCode))
	if cleanCourt == "" {
		cleanCourt = "SC"
	}
	return fmt.Sprintf("%d %s (%s) %d", year, cleanPrefix, cleanCourt, number)
}

// NormalizeCourtCode maps full court names to standard short codes.
func NormalizeCourtCode(courtName string) string {
	lower := strings.ToLower(strings.TrimSpace(courtName))
	switch {
	case strings.Contains(lower, "supreme court"):
		return "SC"
	case strings.Contains(lower, "delhi"):
		return "DEL"
	case strings.Contains(lower, "bombay") || strings.Contains(lower, "mumbai"):
		return "BOM"
	case strings.Contains(lower, "madras") || strings.Contains(lower, "chennai"):
		return "MAD"
	case strings.Contains(lower, "calcutta") || strings.Contains(lower, "kolkata"):
		return "CAL"
	case strings.Contains(lower, "karnataka") || strings.Contains(lower, "bengaluru"):
		return "KAR"
	case strings.Contains(lower, "kerala"):
		return "KER"
	case strings.Contains(lower, "allahabad"):
		return "ALL"
	case strings.Contains(lower, "gujarat"):
		return "GUJ"
	case strings.Contains(lower, "punjab") || strings.Contains(lower, "haryana"):
		return "P&H"
	case strings.Contains(lower, "nclat"):
		return "NCLAT"
	case strings.Contains(lower, "nclt"):
		return "NCLT"
	default:
		return "HC"
	}
}

// VerifyPDFMagicBytes verifies true PDF format by examining header magic bytes (%PDF-).
func VerifyPDFMagicBytes(data []byte) bool {
	if len(data) < 5 {
		return false
	}
	return bytes.HasPrefix(data, []byte("%PDF-"))
}

// ValidatePDFSize checks whether size is within max configured limit.
func ValidatePDFSize(sizeBytes int64, maxSizeBytes int64) error {
	if sizeBytes <= 0 {
		return errors.New("file size must be greater than zero")
	}
	if maxSizeBytes > 0 && sizeBytes > maxSizeBytes {
		return fmt.Errorf("%w: %d bytes exceeds limit of %d bytes", ErrPDFTooLarge, sizeBytes, maxSizeBytes)
	}
	return nil
}

func cleanStringSlice(items []string) []string {
	if items == nil {
		return []string{}
	}
	res := make([]string, 0, len(items))
	for _, it := range items {
		t := strings.TrimSpace(it)
		if t != "" {
			res = append(res, t)
		}
	}
	return res
}
