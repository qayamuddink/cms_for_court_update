package livecase

import (
	"errors"
	"time"
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
	ID                string
	CourtName         string
	CaseNumber        string
	CaseType          string
	Title             string
	BenchJudges       []string
	PresidingJudge    string
	PetitionerParties []string
	RespondentParties []string
	Status            CaseStatus
	NextHearingAt     *time.Time
	LastSequenceNo    int64
	IsLive            bool
	JudgmentArticleID *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// LiveUpdate represents an append-only live court report update.
type LiveUpdate struct {
	ID               string
	CourtCaseID      string
	SequenceNo       int64
	UpdateType       UpdateType
	SpeakerLabel     *string
	Content          string
	ReporterID       string
	Status           UpdateStatus
	CorrectsUpdateID *string
	EventTime        time.Time
	CreatedAt        time.Time
	IdempotencyKey   *string
}

// CanPostUpdate verifies whether a live update can be appended to this case.
func (c *CourtCase) CanPostUpdate() error {
	if c.Status == CaseStatusDisposed || c.Status == CaseStatusDecided {
		return errors.New("cannot post live updates to a closed case")
	}
	return nil
}
