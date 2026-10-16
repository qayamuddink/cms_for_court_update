package livecase

import (
	"errors"
	"fmt"
	"html"
	"strings"
	"time"
	"unicode/utf8"
)

// CaseStatus represents the legal case lifecycle stage.
type CaseStatus string

const (
	CaseStatusListed   CaseStatus = "listed"
	CaseStatusHearing  CaseStatus = "hearing"
	CaseStatusReserved CaseStatus = "reserved"
	CaseStatusDecided  CaseStatus = "decided"
	CaseStatusDisposed CaseStatus = "disposed"
)

// UpdateType defines the category of a live hearing update.
type UpdateType string

const (
	UpdateTypeArgument       UpdateType = "argument"
	UpdateTypeJudicialRemark UpdateType = "judicial_remark"
	UpdateTypeOrder          UpdateType = "order"
	UpdateTypeBreak          UpdateType = "break"
	UpdateTypeAdjournment    UpdateType = "adjournment"
	UpdateTypeVerdict        UpdateType = "verdict"
)

// UpdateStatus defines live update publication status.
type UpdateStatus string

const (
	UpdateStatusPublished UpdateStatus = "published"
	UpdateStatusCorrected UpdateStatus = "corrected"
	UpdateStatusRetracted UpdateStatus = "retracted"
)

// CourtCase represents a judicial hearing case entity.
type CourtCase struct {
	ID                string     `json:"id"`
	CourtName         string     `json:"court_name"`
	CaseNumber        string     `json:"case_number"`
	CaseType          string     `json:"case_type"`
	Title             string     `json:"title"`
	BenchJudges       []string   `json:"bench_judges"`
	PresidingJudge    string     `json:"presiding_judge"`
	PetitionerParties []string   `json:"petitioner_parties"`
	RespondentParties []string   `json:"respondent_parties"`
	Status            CaseStatus `json:"status"`
	NextHearingAt     *time.Time `json:"next_hearing_at,omitempty"`
	LastSequenceNo    int64      `json:"last_sequence_no"`
	IsLive            bool       `json:"is_live"`
	JudgmentArticleID *string    `json:"judgment_article_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// LiveUpdate represents an append-only live court report update.
type LiveUpdate struct {
	ID               string       `json:"id"`
	CourtCaseID      string       `json:"court_case_id"`
	SequenceNo       int64        `json:"sequence_no"`
	UpdateType       UpdateType   `json:"update_type"`
	SpeakerLabel     *string      `json:"speaker_label,omitempty"`
	Content          string       `json:"content"`
	ReporterID       string       `json:"reporter_id"`
	Status           UpdateStatus `json:"status"`
	CorrectsUpdateID *string      `json:"corrects_update_id,omitempty"`
	EventTime        time.Time    `json:"event_time"`
	CreatedAt        time.Time    `json:"created_at"`
	IdempotencyKey   *string      `json:"idempotency_key,omitempty"`
}

// CaseAssignment maps a reporter allowed to post live updates to a specific case.
type CaseAssignment struct {
	ID          string    `json:"id"`
	CourtCaseID string    `json:"court_case_id"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// IsValidUpdateType checks if a given update type is recognized.
func IsValidUpdateType(t UpdateType) bool {
	switch t {
	case UpdateTypeArgument, UpdateTypeJudicialRemark, UpdateTypeOrder,
		UpdateTypeBreak, UpdateTypeAdjournment, UpdateTypeVerdict:
		return true
	default:
		return false
	}
}

// IsValidCaseStatus checks if a given case status is recognized.
func IsValidCaseStatus(s CaseStatus) bool {
	switch s {
	case CaseStatusListed, CaseStatusHearing, CaseStatusReserved,
		CaseStatusDecided, CaseStatusDisposed:
		return true
	default:
		return false
	}
}

// CanPostUpdate verifies whether a live update can be appended to this case.
func (c *CourtCase) CanPostUpdate() error {
	if c.Status == CaseStatusDisposed || c.Status == CaseStatusDecided {
		return errors.New("cannot post live updates to a closed case")
	}
	return nil
}

// SanitizeContent validates encoding, length, and sanitizes markup.
func SanitizeContent(content string) (string, error) {
	if !utf8.ValidString(content) {
		return "", errors.New("content contains invalid UTF-8 encoding")
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", errors.New("content cannot be empty")
	}

	if utf8.RuneCountInString(trimmed) > 5000 {
		return "", errors.New("content exceeds maximum limit of 5000 characters")
	}

	// Escape harmful HTML/script tags while preserving clean plain text
	sanitized := html.EscapeString(trimmed)
	return sanitized, nil
}

// ValidateLiveUpdateParams validates parameters before publishing an update.
func ValidateLiveUpdateParams(content string, updateType UpdateType) (string, error) {
	if !IsValidUpdateType(updateType) {
		return "", fmt.Errorf("invalid update type: %s", updateType)
	}

	return SanitizeContent(content)
}

// CanCorrectUpdate checks if the target update can be corrected.
func CanCorrectUpdate(target *LiveUpdate) error {
	if target == nil {
		return errors.New("target update not found")
	}
	if target.Status == UpdateStatusRetracted {
		return errors.New("cannot edit or correct an update after retraction")
	}
	return nil
}

// CanRetractUpdate checks if the target update can be retracted.
func CanRetractUpdate(target *LiveUpdate) error {
	if target == nil {
		return errors.New("target update not found")
	}
	if target.Status == UpdateStatusRetracted {
		return errors.New("update is already retracted")
	}
	return nil
}
