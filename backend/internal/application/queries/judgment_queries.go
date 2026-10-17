package queries

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/judgment"
)

var (
	ErrJudgmentNotFound     = errors.New("judgment not found")
	ErrPDFNotFound          = errors.New("no PDF is attached to this judgment")
	ErrSubscriptionRequired = errors.New("active subscription required to download premium judgments")
)

// JudgmentDetail wraps judgment domain model with related statutes.
type JudgmentDetail struct {
	Judgment judgment.Judgment       `json:"judgment"`
	Statutes []judgment.StatuteCited `json:"statutes"`
}

// DownloadRequest contains context for generating a secure download link.
type DownloadRequest struct {
	JudgmentID string
	UserID     *string
	IPAddress  string
	UserAgent  string
}

// DownloadResponse returns signed URL and metadata.
type DownloadResponse struct {
	DownloadURL      string `json:"download_url"`
	Citation         string `json:"citation"`
	Filename         string `json:"filename"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

// JudgmentQueryUseCases handles read-side operations and secure gated PDF downloads.
type JudgmentQueryUseCases struct {
	judgmentRepo ports.JudgmentRepository
	entitlement  ports.EntitlementChecker
	fileStore    ports.FileStore
	clock        ports.Clock
}

// NewJudgmentQueryUseCases creates a new JudgmentQueryUseCases instance.
func NewJudgmentQueryUseCases(
	judgmentRepo ports.JudgmentRepository,
	entitlement ports.EntitlementChecker,
	fileStore ports.FileStore,
	clock ports.Clock,
) *JudgmentQueryUseCases {
	return &JudgmentQueryUseCases{
		judgmentRepo: judgmentRepo,
		entitlement:  entitlement,
		fileStore:    fileStore,
		clock:        clock,
	}
}

// GetByID returns the judgment with statutes.
func (uc *JudgmentQueryUseCases) GetByID(ctx context.Context, id string) (*JudgmentDetail, error) {
	j, err := uc.judgmentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrJudgmentNotFound
	}

	statutes, _ := uc.judgmentRepo.GetStatutes(ctx, id)

	return &JudgmentDetail{
		Judgment: *j,
		Statutes: statutes,
	}, nil
}

// GetByCitation returns a judgment by its official citation.
func (uc *JudgmentQueryUseCases) GetByCitation(ctx context.Context, citation string) (*JudgmentDetail, error) {
	j, err := uc.judgmentRepo.GetByCitation(ctx, citation)
	if err != nil {
		return nil, ErrJudgmentNotFound
	}

	statutes, _ := uc.judgmentRepo.GetStatutes(ctx, j.ID)

	return &JudgmentDetail{
		Judgment: *j,
		Statutes: statutes,
	}, nil
}

// List lists judgments matching the given filter.
func (uc *JudgmentQueryUseCases) List(ctx context.Context, filter ports.JudgmentFilter) ([]judgment.Judgment, int, error) {
	return uc.judgmentRepo.List(ctx, filter)
}

// GetPages returns the page-by-page extracted text for a judgment.
func (uc *JudgmentQueryUseCases) GetPages(ctx context.Context, judgmentID string) ([]judgment.JudgmentPage, error) {
	return uc.judgmentRepo.GetPages(ctx, judgmentID)
}

// GetDownloadURL verifies reader entitlement for premium judgments, generates a presigned URL,
// and records an audit log entry with IP, User-Agent, and timestamp.
func (uc *JudgmentQueryUseCases) GetDownloadURL(ctx context.Context, req DownloadRequest) (*DownloadResponse, error) {
	j, err := uc.judgmentRepo.GetByID(ctx, req.JudgmentID)
	if err != nil {
		return nil, ErrJudgmentNotFound
	}

	if j.PDFStorageKey == nil || *j.PDFStorageKey == "" {
		return nil, ErrPDFNotFound
	}

	// 1. Entitlement check for premium judgments
	if j.AccessLevel == judgment.AccessLevelPremium {
		if uc.entitlement == nil {
			return nil, ErrSubscriptionRequired
		}
		allowed, err := uc.entitlement.HasJudgmentAccess(ctx, req.UserID, string(j.AccessLevel))
		if err != nil || !allowed {
			return nil, ErrSubscriptionRequired
		}
	}

	// 2. Presign download URL (15 minutes expiry)
	expiry := 15 * time.Minute
	downloadURL := ""
	if uc.fileStore != nil {
		url, err := uc.fileStore.PresignedGetURL(ctx, *j.PDFStorageKey, expiry)
		if err != nil {
			return nil, fmt.Errorf("failed to sign download URL: %w", err)
		}
		downloadURL = url
	}

	// 3. Log download audit record
	now := uc.clock.Now().UTC()
	audit := judgment.JudgmentDownload{
		UserID:       req.UserID,
		JudgmentID:   j.ID,
		DownloadedAt: now,
		IPAddress:    req.IPAddress,
		UserAgent:    req.UserAgent,
		Watermarked:  false, // Watermark flag ready
	}
	_ = uc.judgmentRepo.LogDownload(ctx, audit)

	citation := ""
	if j.Citation != nil {
		citation = *j.Citation
	}
	filename := "judgment.pdf"
	if j.PDFFilename != nil && *j.PDFFilename != "" {
		filename = *j.PDFFilename
	}

	return &DownloadResponse{
		DownloadURL:      downloadURL,
		Citation:         citation,
		Filename:         filename,
		ExpiresInSeconds: int(expiry.Seconds()),
	}, nil
}
