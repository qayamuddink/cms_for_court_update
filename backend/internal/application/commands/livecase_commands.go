package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/identity"
	"github.com/livelaw/backend/internal/domain/livecase"
)

var (
	ErrCaseNotFound            = errors.New("court case not found")
	ErrUpdateNotFound          = errors.New("live update not found")
	ErrCaseClosed              = errors.New("cannot post live updates to a closed case")
	ErrVerdictAlreadyPosted    = errors.New("a verdict update may be posted only once per case")
	ErrCannotModifyRetracted   = errors.New("cannot edit or correct an update after retraction")
	ErrUpdateAlreadyRetracted  = errors.New("update is already retracted")
	ErrReporterNotAssigned     = errors.New("reporter is not assigned to this case")
	ErrInvalidReporterAccount  = errors.New("assigned user must be a reporter")
)

// LiveCaseUseCases coordinates case management and live courtroom reporting operations.
type LiveCaseUseCases struct {
	caseRepo       ports.CourtCaseRepository
	updateRepo     ports.LiveUpdateRepository
	assignmentRepo ports.CaseAssignmentRepository
	outboxRepo     ports.OutboxRepository
	txMgr          ports.TransactionManager
	liveEngine     ports.LiveEnginePort
	userRepo       ports.UserRepository
	idGen          ports.IDGenerator
	clock          ports.Clock
}

// NewLiveCaseUseCases creates a new LiveCaseUseCases instance.
func NewLiveCaseUseCases(
	caseRepo ports.CourtCaseRepository,
	updateRepo ports.LiveUpdateRepository,
	assignmentRepo ports.CaseAssignmentRepository,
	outboxRepo ports.OutboxRepository,
	txMgr ports.TransactionManager,
	liveEngine ports.LiveEnginePort,
	userRepo ports.UserRepository,
	idGen ports.IDGenerator,
	clock ports.Clock,
) *LiveCaseUseCases {
	return &LiveCaseUseCases{
		caseRepo:       caseRepo,
		updateRepo:     updateRepo,
		assignmentRepo: assignmentRepo,
		outboxRepo:     outboxRepo,
		txMgr:          txMgr,
		liveEngine:     liveEngine,
		userRepo:       userRepo,
		idGen:          idGen,
		clock:          clock,
	}
}

// CreateCaseCommand carries input for creating a court case.
type CreateCaseCommand struct {
	ActorRole         identity.SystemRole
	CourtName         string
	CaseNumber        string
	CaseType          string
	Title             string
	BenchJudges       []string
	PresidingJudge    string
	PetitionerParties []string
	RespondentParties []string
	NextHearingAt     *time.Time
}

// CreateCase creates a new court case record.
func (uc *LiveCaseUseCases) CreateCase(ctx context.Context, cmd CreateCaseCommand) (*livecase.CourtCase, error) {
	if cmd.ActorRole != identity.RoleEditor && cmd.ActorRole != identity.RoleAdmin {
		return nil, ErrForbidden
	}

	if cmd.CourtName == "" || cmd.CaseNumber == "" || cmd.Title == "" || cmd.PresidingJudge == "" {
		return nil, errors.New("court_name, case_number, title, and presiding_judge are required")
	}

	now := uc.clock.Now()
	c := &livecase.CourtCase{
		ID:                uc.idGen.NewID(),
		CourtName:         cmd.CourtName,
		CaseNumber:        cmd.CaseNumber,
		CaseType:          cmd.CaseType,
		Title:             cmd.Title,
		BenchJudges:       cmd.BenchJudges,
		PresidingJudge:    cmd.PresidingJudge,
		PetitionerParties: cmd.PetitionerParties,
		RespondentParties: cmd.RespondentParties,
		Status:            livecase.CaseStatusListed,
		NextHearingAt:     cmd.NextHearingAt,
		LastSequenceNo:    0,
		IsLive:            false,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := uc.caseRepo.Create(ctx, c); err != nil {
		return nil, err
	}

	if uc.outboxRepo != nil {
		_ = uc.outboxRepo.Save(ctx, &ports.OutboxEvent{
			ID:            uc.idGen.NewID(),
			AggregateType: "court_case",
			AggregateID:   c.ID,
			EventType:     "case.created",
			Payload:       []byte(fmt.Sprintf(`{"case_id":"%s"}`, c.ID)),
			SchemaVersion: 1,
			CreatedAt:     now.Format(time.RFC3339),
		})
	}

	return c, nil
}

// UpdateCaseCommand carries updates for court case details.
type UpdateCaseCommand struct {
	ActorRole         identity.SystemRole
	CaseID            string
	CourtName         string
	CaseNumber        string
	CaseType          string
	Title             string
	BenchJudges       []string
	PresidingJudge    string
	PetitionerParties []string
	RespondentParties []string
	Status            livecase.CaseStatus
	NextHearingAt     *time.Time
}

// UpdateCase modifies case metadata and hearing status.
func (uc *LiveCaseUseCases) UpdateCase(ctx context.Context, cmd UpdateCaseCommand) (*livecase.CourtCase, error) {
	if cmd.ActorRole != identity.RoleEditor && cmd.ActorRole != identity.RoleAdmin {
		return nil, ErrForbidden
	}

	c, err := uc.caseRepo.GetByID(ctx, cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrCaseNotFound
	}

	if !livecase.IsValidCaseStatus(cmd.Status) {
		return nil, errors.New("invalid case status")
	}

	c.CourtName = cmd.CourtName
	c.CaseNumber = cmd.CaseNumber
	c.CaseType = cmd.CaseType
	c.Title = cmd.Title
	c.BenchJudges = cmd.BenchJudges
	c.PresidingJudge = cmd.PresidingJudge
	c.PetitionerParties = cmd.PetitionerParties
	c.RespondentParties = cmd.RespondentParties
	c.Status = cmd.Status
	c.NextHearingAt = cmd.NextHearingAt
	c.UpdatedAt = uc.clock.Now()

	if err := uc.caseRepo.Update(ctx, c); err != nil {
		return nil, err
	}

	if uc.outboxRepo != nil {
		_ = uc.outboxRepo.Save(ctx, &ports.OutboxEvent{
			ID:            uc.idGen.NewID(),
			AggregateType: "court_case",
			AggregateID:   c.ID,
			EventType:     "case.updated",
			Payload:       []byte(fmt.Sprintf(`{"case_id":"%s"}`, c.ID)),
			SchemaVersion: 1,
			CreatedAt:     c.UpdatedAt.Format(time.RFC3339),
		})
	}

	return c, nil
}

// SetCaseLive sets the is_live status of a case.
func (uc *LiveCaseUseCases) SetCaseLive(ctx context.Context, actorRole identity.SystemRole, caseID string, isLive bool) (*livecase.CourtCase, error) {
	if actorRole != identity.RoleEditor && actorRole != identity.RoleAdmin {
		return nil, ErrForbidden
	}

	now := uc.clock.Now()
	if err := uc.caseRepo.SetLive(ctx, caseID, isLive, now); err != nil {
		return nil, err
	}

	c, err := uc.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return nil, err
	}

	if uc.outboxRepo != nil {
		_ = uc.outboxRepo.Save(ctx, &ports.OutboxEvent{
			ID:            uc.idGen.NewID(),
			AggregateType: "court_case",
			AggregateID:   caseID,
			EventType:     "case.live_status_changed",
			Payload:       []byte(fmt.Sprintf(`{"case_id":"%s","is_live":%t}`, caseID, isLive)),
			SchemaVersion: 1,
			CreatedAt:     now.Format(time.RFC3339),
		})
	}

	return c, nil
}

// AssignReporter assigns a reporter to report live on a court case.
func (uc *LiveCaseUseCases) AssignReporter(ctx context.Context, actorRole identity.SystemRole, caseID, targetUserID string) error {
	if actorRole != identity.RoleEditor && actorRole != identity.RoleAdmin {
		return ErrForbidden
	}

	c, err := uc.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return err
	}
	if c == nil {
		return ErrCaseNotFound
	}

	targetUser, err := uc.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if targetUser == nil {
		return ErrUserNotFound
	}
	if targetUser.SystemRole != identity.RoleReporter {
		return ErrInvalidReporterAccount
	}

	assignment := &livecase.CaseAssignment{
		ID:          uc.idGen.NewID(),
		CourtCaseID: caseID,
		UserID:      targetUserID,
		CreatedAt:   uc.clock.Now(),
	}

	return uc.assignmentRepo.Assign(ctx, assignment)
}

// UnassignReporter removes a reporter assignment from a case.
func (uc *LiveCaseUseCases) UnassignReporter(ctx context.Context, actorRole identity.SystemRole, caseID, targetUserID string) error {
	if actorRole != identity.RoleEditor && actorRole != identity.RoleAdmin {
		return ErrForbidden
	}

	return uc.assignmentRepo.Unassign(ctx, caseID, targetUserID)
}

// PublishLiveUpdateCommand carries parameters for appending a live courtroom update.
type PublishLiveUpdateCommand struct {
	ActorRole      identity.SystemRole
	ReporterUserID string
	CaseID         string
	UpdateType     livecase.UpdateType
	SpeakerLabel   *string
	Content        string
	EventTime      time.Time
	IdempotencyKey *string
}

// PublishLiveUpdate appends an update to a case within an atomic transaction.
// Enforces:
// 1. Reporter assignment or editor/admin authorization
// 2. Content sanitization and validation
// 3. Row-level case lock (FOR UPDATE)
// 4. Idempotency replay with no duplicate sequence increment
// 5. Case closure check
// 6. Single verdict rule
// 7. Strictly consecutive sequence numbering (last_sequence_no + 1)
// 8. Transactional outbox event creation
func (uc *LiveCaseUseCases) PublishLiveUpdate(ctx context.Context, cmd PublishLiveUpdateCommand) (*livecase.LiveUpdate, error) {
	// Authorization check: editor/admin or assigned reporter
	if cmd.ActorRole != identity.RoleEditor && cmd.ActorRole != identity.RoleAdmin {
		if cmd.ActorRole != identity.RoleReporter {
			return nil, ErrForbidden
		}
		isAssigned, err := uc.assignmentRepo.IsReporterAssigned(ctx, cmd.CaseID, cmd.ReporterUserID)
		if err != nil {
			return nil, fmt.Errorf("failed checking reporter assignment: %w", err)
		}
		if !isAssigned {
			return nil, ErrReporterNotAssigned
		}
	}

	// Validate & sanitize content
	sanitizedContent, err := livecase.ValidateLiveUpdateParams(cmd.Content, cmd.UpdateType)
	if err != nil {
		return nil, err
	}

	// Check idempotency prior to locking if key is present
	if cmd.IdempotencyKey != nil && *cmd.IdempotencyKey != "" {
		existing, err := uc.updateRepo.GetByIdempotencyKey(ctx, cmd.CaseID, *cmd.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil // Idempotent return without duplicate
		}
	}

	eventTime := cmd.EventTime
	if eventTime.IsZero() {
		eventTime = uc.clock.Now()
	}

	// Begin atomic transaction
	tx, err := uc.txMgr.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed starting transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// 1. Lock case row
	courtCase, err := uc.txMgr.LockCaseForUpdate(ctx, tx, cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if courtCase == nil {
		return nil, ErrCaseNotFound
	}

	// Check idempotency again inside lock to prevent concurrent identical requests
	if cmd.IdempotencyKey != nil && *cmd.IdempotencyKey != "" {
		existing, err := uc.updateRepo.GetByIdempotencyKey(ctx, cmd.CaseID, *cmd.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}

	// 2. Reject posting to a closed case
	if err := courtCase.CanPostUpdate(); err != nil {
		return nil, ErrCaseClosed
	}

	// 3. Verdict rule: may be posted only once per case
	if cmd.UpdateType == livecase.UpdateTypeVerdict {
		hasVerdict, err := uc.updateRepo.HasVerdict(ctx, cmd.CaseID)
		if err != nil {
			return nil, err
		}
		if hasVerdict {
			return nil, ErrVerdictAlreadyPosted
		}
	}

	// 4. Sequence number = last_sequence_no + 1
	nextSeq := courtCase.LastSequenceNo + 1
	now := uc.clock.Now()

	update := &livecase.LiveUpdate{
		ID:               uc.idGen.NewID(),
		CourtCaseID:      cmd.CaseID,
		SequenceNo:       nextSeq,
		UpdateType:       cmd.UpdateType,
		SpeakerLabel:     cmd.SpeakerLabel,
		Content:          sanitizedContent,
		ReporterID:       cmd.ReporterUserID,
		Status:           livecase.UpdateStatusPublished,
		CorrectsUpdateID: nil,
		EventTime:        eventTime,
		CreatedAt:        now,
		IdempotencyKey:   cmd.IdempotencyKey,
	}

	// 5. Insert update row
	if err := uc.updateRepo.CreateInTx(ctx, tx, update); err != nil {
		return nil, err
	}

	// 6. Update case counter
	if err := uc.txMgr.UpdateCaseSequence(ctx, tx, cmd.CaseID, nextSeq, now); err != nil {
		return nil, err
	}

	// 7. Write versioned JSON envelope into outbox_events
	outboxEvent, err := uc.createOutboxEvent(update)
	if err != nil {
		return nil, err
	}
	if err := uc.outboxRepo.SaveInTx(ctx, tx, outboxEvent); err != nil {
		return nil, err
	}

	// 8. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	tx = nil // Prevent rollback in defer

	// Notify in-process engine
	_ = uc.liveEngine.PublishUpdate(ctx, *update)

	return update, nil
}

// CorrectLiveUpdateCommand carries input for creating a corrected live update row.
type CorrectLiveUpdateCommand struct {
	ActorRole        identity.SystemRole
	ReporterUserID   string
	CaseID           string
	TargetUpdateID   string
	UpdateType       livecase.UpdateType
	SpeakerLabel     *string
	Content          string
	IdempotencyKey   *string
}

// CorrectLiveUpdate appends a new row with status 'corrected' referencing the old update.
// Rows are never edited in place.
func (uc *LiveCaseUseCases) CorrectLiveUpdate(ctx context.Context, cmd CorrectLiveUpdateCommand) (*livecase.LiveUpdate, error) {
	// Authorization check
	if cmd.ActorRole != identity.RoleEditor && cmd.ActorRole != identity.RoleAdmin {
		if cmd.ActorRole != identity.RoleReporter {
			return nil, ErrForbidden
		}
		isAssigned, err := uc.assignmentRepo.IsReporterAssigned(ctx, cmd.CaseID, cmd.ReporterUserID)
		if err != nil {
			return nil, err
		}
		if !isAssigned {
			return nil, ErrReporterNotAssigned
		}
	}

	// Validate content
	sanitizedContent, err := livecase.ValidateLiveUpdateParams(cmd.Content, cmd.UpdateType)
	if err != nil {
		return nil, err
	}

	tx, err := uc.txMgr.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// 1. Lock case
	courtCase, err := uc.txMgr.LockCaseForUpdate(ctx, tx, cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if courtCase == nil {
		return nil, ErrCaseNotFound
	}

	if err := courtCase.CanPostUpdate(); err != nil {
		return nil, ErrCaseClosed
	}

	// 2. Fetch target update
	targetUpdate, err := uc.updateRepo.GetByID(ctx, cmd.TargetUpdateID)
	if err != nil {
		return nil, err
	}
	if targetUpdate == nil || targetUpdate.CourtCaseID != cmd.CaseID {
		return nil, ErrUpdateNotFound
	}

	// 3. Rule: No edits after retraction
	if err := livecase.CanCorrectUpdate(targetUpdate); err != nil {
		return nil, ErrCannotModifyRetracted
	}
	isRetracted, err := uc.updateRepo.IsRetracted(ctx, cmd.CaseID, targetUpdate.ID)
	if err != nil {
		return nil, err
	}
	if isRetracted {
		return nil, ErrCannotModifyRetracted
	}

	nextSeq := courtCase.LastSequenceNo + 1
	now := uc.clock.Now()

	update := &livecase.LiveUpdate{
		ID:               uc.idGen.NewID(),
		CourtCaseID:      cmd.CaseID,
		SequenceNo:       nextSeq,
		UpdateType:       cmd.UpdateType,
		SpeakerLabel:     cmd.SpeakerLabel,
		Content:          sanitizedContent,
		ReporterID:       cmd.ReporterUserID,
		Status:           livecase.UpdateStatusCorrected,
		CorrectsUpdateID: &targetUpdate.ID,
		EventTime:        now,
		CreatedAt:        now,
		IdempotencyKey:   cmd.IdempotencyKey,
	}

	if err := uc.updateRepo.CreateInTx(ctx, tx, update); err != nil {
		return nil, err
	}

	if err := uc.txMgr.UpdateCaseSequence(ctx, tx, cmd.CaseID, nextSeq, now); err != nil {
		return nil, err
	}

	outboxEvent, err := uc.createOutboxEvent(update)
	if err != nil {
		return nil, err
	}
	if err := uc.outboxRepo.SaveInTx(ctx, tx, outboxEvent); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil

	_ = uc.liveEngine.PublishUpdate(ctx, *update)
	return update, nil
}

// RetractLiveUpdateCommand carries input for retracting an update.
type RetractLiveUpdateCommand struct {
	ActorRole      identity.SystemRole
	ReporterUserID string
	CaseID         string
	TargetUpdateID string
	Reason         string
	IdempotencyKey *string
}

// RetractLiveUpdate appends a new row with status 'retracted' referencing the retracted update.
// Rows are never edited in place.
func (uc *LiveCaseUseCases) RetractLiveUpdate(ctx context.Context, cmd RetractLiveUpdateCommand) (*livecase.LiveUpdate, error) {
	if cmd.ActorRole != identity.RoleEditor && cmd.ActorRole != identity.RoleAdmin {
		if cmd.ActorRole != identity.RoleReporter {
			return nil, ErrForbidden
		}
		isAssigned, err := uc.assignmentRepo.IsReporterAssigned(ctx, cmd.CaseID, cmd.ReporterUserID)
		if err != nil {
			return nil, err
		}
		if !isAssigned {
			return nil, ErrReporterNotAssigned
		}
	}

	tx, err := uc.txMgr.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	courtCase, err := uc.txMgr.LockCaseForUpdate(ctx, tx, cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if courtCase == nil {
		return nil, ErrCaseNotFound
	}

	if err := courtCase.CanPostUpdate(); err != nil {
		return nil, ErrCaseClosed
	}

	targetUpdate, err := uc.updateRepo.GetByID(ctx, cmd.TargetUpdateID)
	if err != nil {
		return nil, err
	}
	if targetUpdate == nil || targetUpdate.CourtCaseID != cmd.CaseID {
		return nil, ErrUpdateNotFound
	}

	if err := livecase.CanRetractUpdate(targetUpdate); err != nil {
		return nil, ErrUpdateAlreadyRetracted
	}
	isRetracted, err := uc.updateRepo.IsRetracted(ctx, cmd.CaseID, targetUpdate.ID)
	if err != nil {
		return nil, err
	}
	if isRetracted {
		return nil, ErrUpdateAlreadyRetracted
	}

	nextSeq := courtCase.LastSequenceNo + 1
	now := uc.clock.Now()

	reasonText := cmd.Reason
	if reasonText == "" {
		reasonText = "Update retracted by reporting desk"
	}

	update := &livecase.LiveUpdate{
		ID:               uc.idGen.NewID(),
		CourtCaseID:      cmd.CaseID,
		SequenceNo:       nextSeq,
		UpdateType:       targetUpdate.UpdateType,
		SpeakerLabel:     targetUpdate.SpeakerLabel,
		Content:          "[Retracted] " + reasonText,
		ReporterID:       cmd.ReporterUserID,
		Status:           livecase.UpdateStatusRetracted,
		CorrectsUpdateID: &targetUpdate.ID,
		EventTime:        now,
		CreatedAt:        now,
		IdempotencyKey:   cmd.IdempotencyKey,
	}

	if err := uc.updateRepo.CreateInTx(ctx, tx, update); err != nil {
		return nil, err
	}

	if err := uc.txMgr.UpdateCaseSequence(ctx, tx, cmd.CaseID, nextSeq, now); err != nil {
		return nil, err
	}

	outboxEvent, err := uc.createOutboxEvent(update)
	if err != nil {
		return nil, err
	}
	if err := uc.outboxRepo.SaveInTx(ctx, tx, outboxEvent); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil

	_ = uc.liveEngine.PublishUpdate(ctx, *update)
	return update, nil
}

// OutboxPayloadEnvelope represents the versioned JSON envelope for live updates.
type OutboxPayloadEnvelope struct {
	SchemaVersion    int        `json:"schema_version"`
	EventID          string     `json:"event_id"`
	EventType        string     `json:"event_type"`
	CaseID           string     `json:"case_id"`
	SequenceNo       int64      `json:"sequence_no"`
	UpdateID         string     `json:"update_id"`
	UpdateType       string     `json:"update_type"`
	SpeakerLabel     *string    `json:"speaker_label,omitempty"`
	Content          string     `json:"content"`
	Status           string     `json:"status"`
	CorrectsUpdateID *string    `json:"corrects_update_id,omitempty"`
	EventTime        time.Time  `json:"event_time"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (uc *LiveCaseUseCases) createOutboxEvent(u *livecase.LiveUpdate) (*ports.OutboxEvent, error) {
	eventID := uc.idGen.NewID()
	eventType := "live_update.published"
	switch u.Status {
	case livecase.UpdateStatusCorrected:
		eventType = "live_update.corrected"
	case livecase.UpdateStatusRetracted:
		eventType = "live_update.retracted"
	}

	envelope := OutboxPayloadEnvelope{
		SchemaVersion:    1,
		EventID:          eventID,
		EventType:        eventType,
		CaseID:           u.CourtCaseID,
		SequenceNo:       u.SequenceNo,
		UpdateID:         u.ID,
		UpdateType:       string(u.UpdateType),
		SpeakerLabel:     u.SpeakerLabel,
		Content:          u.Content,
		Status:           string(u.Status),
		CorrectsUpdateID: u.CorrectsUpdateID,
		EventTime:        u.EventTime,
		CreatedAt:        u.CreatedAt,
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed encoding outbox payload: %w", err)
	}

	return &ports.OutboxEvent{
		ID:            eventID,
		AggregateType: "court_case",
		AggregateID:   u.CourtCaseID,
		EventType:     eventType,
		Payload:       payload,
		SchemaVersion: 1,
		CreatedAt:     u.CreatedAt.Format(time.RFC3339),
	}, nil
}
