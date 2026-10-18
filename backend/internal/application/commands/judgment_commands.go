package commands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/identity"
	"github.com/livelaw/backend/internal/domain/judgment"
)

var (
	ErrDuplicatePDF       = errors.New("a judgment with this identical PDF has already been uploaded")
	ErrInfectedFile       = errors.New("uploaded file failed security scan")
	ErrPDFUploadFailed    = errors.New("failed to upload PDF document")
	ErrUnauthorizedAction = errors.New("unauthorized to perform this editorial action")
)

const (
	DefaultMaxPDFSizeBytes int64 = 100 * 1024 * 1024 // 100 MB
)

// StatuteInput holds statute and sections for a judgment.
type StatuteInput struct {
	StatuteName   string   `json:"statute_name"`
	SectionsCited []string `json:"sections_cited"`
}

// CreateJudgmentCommand holds data to create a new judgment draft.
type CreateJudgmentCommand struct {
	ActorID      string
	ActorRole    identity.SystemRole
	CaseTitle    string
	CaseNumber   string
	Court        string
	CourtCode    string
	Bench        []string
	JudgmentDate time.Time
	DecisionDate *time.Time
	Appellant    string
	Respondent   string
	Advocates    []string
	SubjectTags  []string
	Issues       string
	Held         string
	Headnote     string
	Summary      string
	KeyTakeaways []string
	Statutes     []StatuteInput
	CourtCaseID  *string
	ArticleID    *string
	AccessLevel  judgment.AccessLevel
}

// UpdateJudgmentCommand holds data to update an existing judgment.
type UpdateJudgmentCommand struct {
	ActorID      string
	ActorRole    identity.SystemRole
	JudgmentID   string
	CaseTitle    string
	CaseNumber   string
	Court        string
	CourtCode    string
	Bench        []string
	JudgmentDate time.Time
	DecisionDate *time.Time
	Appellant    string
	Respondent   string
	Advocates    []string
	SubjectTags  []string
	Issues       string
	Held         string
	Headnote     string
	Summary      string
	KeyTakeaways []string
	Statutes     []StatuteInput
	CourtCaseID  *string
	ArticleID    *string
	AccessLevel  judgment.AccessLevel
}

// AttachJudgmentPDFCommand holds payload for uploading and attaching a judgment PDF.
type AttachJudgmentPDFCommand struct {
	ActorID    string
	ActorRole  identity.SystemRole
	JudgmentID string
	Filename   string
	Data       []byte
	Size       int64
}

// PublishJudgmentCommand holds parameters to allocate citation and publish.
type PublishJudgmentCommand struct {
	ActorID     string
	ActorRole   identity.SystemRole
	JudgmentID  string
	BrandPrefix string
}

// JudgmentUseCases coordinates judgment creation, PDF security, atomic citation allocation, and publishing.
type JudgmentUseCases struct {
	judgmentRepo ports.JudgmentRepository
	fileStore    ports.FileStore
	scanner      ports.VirusScanner
	extractor    ports.PDFExtractor
	idGen        ports.IDGenerator
	clock        ports.Clock
	brandPrefix  string
	outboxRepo   ports.OutboxRepository
}

// NewJudgmentUseCases instantiates JudgmentUseCases.
func NewJudgmentUseCases(
	judgmentRepo ports.JudgmentRepository,
	fileStore ports.FileStore,
	scanner ports.VirusScanner,
	extractor ports.PDFExtractor,
	idGen ports.IDGenerator,
	clock ports.Clock,
) *JudgmentUseCases {
	brand := os.Getenv("CITATION_BRAND_PREFIX")
	if brand == "" {
		brand = "CourtDesk"
	}

	return &JudgmentUseCases{
		judgmentRepo: judgmentRepo,
		fileStore:    fileStore,
		scanner:      scanner,
		extractor:    extractor,
		idGen:        idGen,
		clock:        clock,
		brandPrefix:  brand,
	}
}

// SetOutboxRepository attaches an outbox repository to publish search and indexing events.
func (uc *JudgmentUseCases) SetOutboxRepository(repo ports.OutboxRepository) {
	uc.outboxRepo = repo
}

// CreateDraft creates a new judgment in draft status.
func (uc *JudgmentUseCases) CreateDraft(ctx context.Context, cmd CreateJudgmentCommand) (*judgment.Judgment, error) {
	if !isAuthorizedEditor(cmd.ActorRole) {
		return nil, ErrUnauthorizedAction
	}

	now := uc.clock.Now().UTC()
	id := uc.idGen.NewID()

	j, err := judgment.NewJudgment(
		id,
		cmd.CaseTitle,
		cmd.CaseNumber,
		cmd.Court,
		cmd.CourtCode,
		cmd.Bench,
		cmd.JudgmentDate,
		cmd.DecisionDate,
		cmd.Appellant,
		cmd.Respondent,
		cmd.Advocates,
		nil, // statutes persisted separately
		cmd.SubjectTags,
		cmd.Issues,
		cmd.Held,
		cmd.Headnote,
		cmd.Summary,
		cmd.KeyTakeaways,
		cmd.CourtCaseID,
		cmd.ArticleID,
		cmd.AccessLevel,
		cmd.ActorID,
		now,
	)
	if err != nil {
		return nil, err
	}

	// Persist judgment
	if err := uc.judgmentRepo.Create(ctx, j); err != nil {
		return nil, fmt.Errorf("failed to save judgment: %w", err)
	}

	// Persist statutes if provided
	if len(cmd.Statutes) > 0 {
		var statutes []judgment.StatuteCited
		for _, s := range cmd.Statutes {
			if strings.TrimSpace(s.StatuteName) != "" {
				statutes = append(statutes, judgment.StatuteCited{
					ID:            uc.idGen.NewID(),
					JudgmentID:    j.ID,
					StatuteName:   strings.TrimSpace(s.StatuteName),
					SectionsCited: s.SectionsCited,
				})
			}
		}
		if len(statutes) > 0 {
			if err := uc.judgmentRepo.AddStatutes(ctx, j.ID, statutes); err != nil {
				return nil, fmt.Errorf("failed to save statutes: %w", err)
			}
			j.StatutesCited = statutes
		}
	}

	return j, nil
}

// Update updates an existing judgment.
func (uc *JudgmentUseCases) Update(ctx context.Context, cmd UpdateJudgmentCommand) (*judgment.Judgment, error) {
	if !isAuthorizedEditor(cmd.ActorRole) {
		return nil, ErrUnauthorizedAction
	}

	j, err := uc.judgmentRepo.GetByID(ctx, cmd.JudgmentID)
	if err != nil {
		return nil, err
	}

	j.CaseTitle = cmd.CaseTitle
	j.CaseNumber = cmd.CaseNumber
	j.Court = cmd.Court
	j.CourtCode = judgment.NormalizeCourtCode(cmd.CourtCode)
	j.Bench = cmd.Bench
	j.JudgmentDate = cmd.JudgmentDate
	j.DecisionDate = cmd.DecisionDate
	j.Appellant = cmd.Appellant
	j.Respondent = cmd.Respondent
	j.Advocates = cmd.Advocates
	j.SubjectTags = cmd.SubjectTags
	j.Issues = cmd.Issues
	j.Held = cmd.Held
	j.Headnote = cmd.Headnote
	j.Summary = cmd.Summary
	j.KeyTakeaways = cmd.KeyTakeaways
	j.CourtCaseID = cmd.CourtCaseID
	j.ArticleID = cmd.ArticleID
	if cmd.AccessLevel != "" {
		j.AccessLevel = cmd.AccessLevel
	}
	j.UpdatedAt = uc.clock.Now().UTC()

	if err := uc.judgmentRepo.Update(ctx, j); err != nil {
		return nil, fmt.Errorf("failed to update judgment: %w", err)
	}

	// Replace statutes
	if len(cmd.Statutes) > 0 {
		var statutes []judgment.StatuteCited
		for _, s := range cmd.Statutes {
			if strings.TrimSpace(s.StatuteName) != "" {
				statutes = append(statutes, judgment.StatuteCited{
					ID:            uc.idGen.NewID(),
					JudgmentID:    j.ID,
					StatuteName:   strings.TrimSpace(s.StatuteName),
					SectionsCited: s.SectionsCited,
				})
			}
		}
		if err := uc.judgmentRepo.AddStatutes(ctx, j.ID, statutes); err != nil {
			return nil, fmt.Errorf("failed to update statutes: %w", err)
		}
		j.StatutesCited = statutes
	}

	return j, nil
}

// AttachPDF validates security rules (magic bytes, size, virus scan, duplicate checksum) and attaches the PDF.
func (uc *JudgmentUseCases) AttachPDF(ctx context.Context, cmd AttachJudgmentPDFCommand) (*judgment.Judgment, error) {
	if !isAuthorizedEditor(cmd.ActorRole) {
		return nil, ErrUnauthorizedAction
	}

	j, err := uc.judgmentRepo.GetByID(ctx, cmd.JudgmentID)
	if err != nil {
		return nil, err
	}

	// 1. Validate file size (max 100 MB)
	if err := judgment.ValidatePDFSize(cmd.Size, DefaultMaxPDFSizeBytes); err != nil {
		return nil, err
	}

	// 2. Validate magic bytes (%PDF-)
	if !judgment.VerifyPDFMagicBytes(cmd.Data) {
		return nil, judgment.ErrInvalidPDFMagicBytes
	}

	// 3. Compute SHA-256 and check for duplicates across existing judgments
	hash := sha256.Sum256(cmd.Data)
	checksum := hex.EncodeToString(hash[:])

	existing, err := uc.judgmentRepo.GetByChecksum(ctx, checksum)
	if err == nil && existing != nil && existing.ID != j.ID {
		return nil, fmt.Errorf("%w: duplicate of judgment %s (%s)", ErrDuplicatePDF, existing.ID, existing.CaseTitle)
	}

	// 4. Virus scan (e.g. EICAR detection / ClamAV)
	if uc.scanner != nil {
		clean, virusName, scanErr := uc.scanner.Scan(ctx, bytes.NewReader(cmd.Data))
		if scanErr != nil {
			return nil, fmt.Errorf("virus scan error: %w", scanErr)
		}
		if !clean {
			return nil, fmt.Errorf("%w: detected %s", ErrInfectedFile, virusName)
		}
	}

	// 5. Generate thumbnail and preview & estimate page count
	storageKey := fmt.Sprintf("judgments/%s/%s", j.ID, cmd.Filename)
	thumbKey := fmt.Sprintf("judgments/%s/thumbnail.svg", j.ID)
	previewKey := fmt.Sprintf("judgments/%s/preview.svg", j.ID)
	pageCount := 1

	var pThumbKey, pPreviewKey *string
	if uc.extractor != nil {
		thumb, prev, count, extErr := uc.extractor.GenerateThumbnailAndPreview(ctx, bytes.NewReader(cmd.Data), cmd.Size)
		if extErr == nil {
			pageCount = count
			if uc.fileStore != nil && len(thumb) > 0 {
				_ = uc.fileStore.PutObject(ctx, thumbKey, bytes.NewReader(thumb), int64(len(thumb)), "image/svg+xml")
				pThumbKey = &thumbKey
			}
			if uc.fileStore != nil && len(prev) > 0 {
				_ = uc.fileStore.PutObject(ctx, previewKey, bytes.NewReader(prev), int64(len(prev)), "image/svg+xml")
				pPreviewKey = &previewKey
			}
		}
	}

	// 6. Upload original PDF to object storage
	if uc.fileStore != nil {
		if err := uc.fileStore.PutObject(ctx, storageKey, bytes.NewReader(cmd.Data), cmd.Size, "application/pdf"); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPDFUploadFailed, err)
		}
	}

	// 7. Update judgment entity
	now := uc.clock.Now().UTC()
	if err := j.AttachPDF(storageKey, cmd.Filename, cmd.Size, checksum, pageCount, pThumbKey, pPreviewKey, now); err != nil {
		return nil, err
	}

	if err := uc.judgmentRepo.Update(ctx, j); err != nil {
		return nil, fmt.Errorf("failed to persist judgment pdf attachment: %w", err)
	}

	return j, nil
}

// Publish atomically allocates a sequential citation and marks the judgment published.
func (uc *JudgmentUseCases) Publish(ctx context.Context, cmd PublishJudgmentCommand) (*judgment.Judgment, error) {
	if !isAuthorizedEditor(cmd.ActorRole) {
		return nil, ErrUnauthorizedAction
	}

	j, err := uc.judgmentRepo.GetByID(ctx, cmd.JudgmentID)
	if err != nil {
		return nil, err
	}

	brand := cmd.BrandPrefix
	if brand == "" {
		brand = uc.brandPrefix
	}

	year := j.JudgmentDate.Year()
	now := uc.clock.Now().UTC()

	// Atomically allocate sequential citation if not already assigned
	if j.Citation == nil || *j.Citation == "" {
		_, num, err := uc.judgmentRepo.AllocateCitation(ctx, j.CourtCode, year, brand)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate citation: %w", err)
		}

		if err := j.Publish(brand, year, j.CourtCode, num, now); err != nil {
			return nil, err
		}
	} else {
		// Already has citation, just update published status
		j.Status = judgment.StatusPublished
		j.PublishedAt = &now
		j.UpdatedAt = now
	}

	if err := uc.judgmentRepo.Update(ctx, j); err != nil {
		return nil, fmt.Errorf("failed to persist published judgment: %w", err)
	}

	if uc.outboxRepo != nil {
		_ = uc.outboxRepo.Save(ctx, &ports.OutboxEvent{
			ID:            uc.idGen.NewID(),
			AggregateType: "judgment",
			AggregateID:   j.ID,
			EventType:     "judgment.published",
			Payload:       []byte(fmt.Sprintf(`{"judgment_id":"%s"}`, j.ID)),
			SchemaVersion: 1,
			CreatedAt:     now.Format(time.RFC3339),
		})
	}

	return j, nil
}

// Helper to extract reader to byte slice
func ReadAllPDF(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

func isAuthorizedEditor(role identity.SystemRole) bool {
	return role == identity.RoleAdmin ||
		role == identity.RoleEditor ||
		role == identity.RoleReporter
}
