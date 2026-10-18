package queries

import (
	"context"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/digest"
)

// DigestDetailView holds full digest data along with assembled legal indexes.
type DigestDetailView struct {
	Digest       *digest.Digest             `json:"digest"`
	NominalIndex []digest.NominalIndexEntry `json:"nominal_index"`
	SubjectIndex []digest.SubjectIndexGroup `json:"subject_index"`
	StatuteIndex []digest.StatuteIndexGroup `json:"statute_index"`
}

// DigestQueries provides read-only queries for digests.
type DigestQueries struct {
	digestRepo ports.DigestRepository
}

// NewDigestQueries creates a new DigestQueries handler.
func NewDigestQueries(digestRepo ports.DigestRepository) *DigestQueries {
	return &DigestQueries{digestRepo: digestRepo}
}

// GetByID returns the digest along with assembled indexes.
func (q *DigestQueries) GetByID(ctx context.Context, id string) (*DigestDetailView, error) {
	d, err := q.digestRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	nominal := digest.BuildNominalIndex(d.Items)
	subjects := digest.BuildSubjectIndex(d.Items)
	statutes := digest.BuildStatuteIndex(d.Items)

	return &DigestDetailView{
		Digest:       d,
		NominalIndex: nominal,
		SubjectIndex: subjects,
		StatuteIndex: statutes,
	}, nil
}

// GetBySlug returns a published digest along with assembled indexes.
func (q *DigestQueries) GetBySlug(ctx context.Context, slug string) (*DigestDetailView, error) {
	d, err := q.digestRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	nominal := digest.BuildNominalIndex(d.Items)
	subjects := digest.BuildSubjectIndex(d.Items)
	statutes := digest.BuildStatuteIndex(d.Items)

	return &DigestDetailView{
		Digest:       d,
		NominalIndex: nominal,
		SubjectIndex: subjects,
		StatuteIndex: statutes,
	}, nil
}

// ListDigests returns matching digests and total count.
func (q *DigestQueries) List(ctx context.Context, filter ports.DigestFilter) ([]digest.Digest, int, error) {
	return q.digestRepo.List(ctx, filter)
}
