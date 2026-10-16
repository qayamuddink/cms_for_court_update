package commands

import (
	"context"
	"errors"
	"strings"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
	"github.com/livelaw/backend/internal/domain/identity"
)

// SectionUseCases coordinates taxonomy section management.
type SectionUseCases struct {
	sectionRepo ports.SectionRepository
	idGen       ports.IDGenerator
	clock       ports.Clock
}

// NewSectionUseCases creates a new SectionUseCases instance.
func NewSectionUseCases(sectionRepo ports.SectionRepository, idGen ports.IDGenerator, clock ports.Clock) *SectionUseCases {
	return &SectionUseCases{
		sectionRepo: sectionRepo,
		idGen:       idGen,
		clock:       clock,
	}
}

// CreateSectionCommand holds parameters for creating a new section.
type CreateSectionCommand struct {
	ActorRole    identity.SystemRole
	Name         string
	Slug         string
	Description  *string
	DisplayOrder int
	ParentID     *string
	IsActive     *bool
}

// CreateSection creates a new taxonomy section.
func (uc *SectionUseCases) CreateSection(ctx context.Context, cmd CreateSectionCommand) (*content.Section, error) {
	if cmd.ActorRole != identity.RoleAdmin && cmd.ActorRole != identity.RoleEditor {
		return nil, content.ErrUnauthorizedWorkflow
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, errors.New("section name is required")
	}

	slug := strings.TrimSpace(cmd.Slug)
	if slug == "" {
		slug = content.GenerateSlug(name)
	} else {
		slug = content.GenerateSlug(slug)
	}

	isActive := true
	if cmd.IsActive != nil {
		isActive = *cmd.IsActive
	}

	now := uc.clock.Now().UTC()
	sec := &content.Section{
		ID:           uc.idGen.NewID(),
		Slug:         slug,
		Name:         name,
		Description:  cmd.Description,
		DisplayOrder: cmd.DisplayOrder,
		ParentID:     cmd.ParentID,
		IsActive:     isActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := uc.sectionRepo.Create(ctx, sec); err != nil {
		return nil, err
	}

	return sec, nil
}

// UpdateSectionCommand holds parameters for modifying an existing section.
type UpdateSectionCommand struct {
	ActorRole    identity.SystemRole
	ID           string
	Name         string
	Slug         string
	Description  *string
	DisplayOrder int
	ParentID     *string
	IsActive     bool
}

// UpdateSection updates a taxonomy section.
func (uc *SectionUseCases) UpdateSection(ctx context.Context, cmd UpdateSectionCommand) (*content.Section, error) {
	if cmd.ActorRole != identity.RoleAdmin && cmd.ActorRole != identity.RoleEditor {
		return nil, content.ErrUnauthorizedWorkflow
	}

	sec, err := uc.sectionRepo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, errors.New("section name is required")
	}

	slug := strings.TrimSpace(cmd.Slug)
	if slug == "" {
		slug = content.GenerateSlug(name)
	} else {
		slug = content.GenerateSlug(slug)
	}

	sec.Name = name
	sec.Slug = slug
	sec.Description = cmd.Description
	sec.DisplayOrder = cmd.DisplayOrder
	sec.ParentID = cmd.ParentID
	sec.IsActive = cmd.IsActive
	sec.UpdatedAt = uc.clock.Now().UTC()

	if err := uc.sectionRepo.Update(ctx, sec); err != nil {
		return nil, err
	}

	return sec, nil
}

// DeleteSection removes a section (Admin only).
func (uc *SectionUseCases) DeleteSection(ctx context.Context, id string, actorRole identity.SystemRole) error {
	if actorRole != identity.RoleAdmin {
		return content.ErrUnauthorizedWorkflow
	}
	return uc.sectionRepo.Delete(ctx, id)
}

// ListSections retrieves sections.
func (uc *SectionUseCases) ListSections(ctx context.Context, activeOnly bool) ([]content.Section, error) {
	return uc.sectionRepo.List(ctx, activeOnly)
}

// GetSectionBySlug retrieves a section by its slug.
func (uc *SectionUseCases) GetSectionBySlug(ctx context.Context, slug string) (*content.Section, error) {
	return uc.sectionRepo.GetBySlug(ctx, slug)
}
