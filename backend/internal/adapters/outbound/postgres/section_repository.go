package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/content"
)

// SQLSectionRepository implements ports.SectionRepository using PostgreSQL.
type SQLSectionRepository struct {
	db *sql.DB
}

// NewSQLSectionRepository creates a new SQLSectionRepository instance.
func NewSQLSectionRepository(db *sql.DB) *SQLSectionRepository {
	return &SQLSectionRepository{db: db}
}

// Create inserts a new section into the database.
func (r *SQLSectionRepository) Create(ctx context.Context, s *content.Section) error {
	query := `
		INSERT INTO sections (
			id, slug, name, description, display_order, parent_id, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
	`

	now := time.Now().UTC()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now
	}

	_, err := r.db.ExecContext(ctx, query,
		s.ID,
		s.Slug,
		s.Name,
		s.Description,
		s.DisplayOrder,
		s.ParentID,
		s.IsActive,
		s.CreatedAt.UTC(),
		s.UpdatedAt.UTC(),
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ports.ErrSectionSlugExists
		}
		return fmt.Errorf("failed inserting section: %w", err)
	}

	return nil
}

// GetByID retrieves a section by its UUID.
func (r *SQLSectionRepository) GetByID(ctx context.Context, id string) (*content.Section, error) {
	query := `
		SELECT id, slug, name, description, display_order, parent_id, is_active, created_at, updated_at
		FROM sections
		WHERE id = $1;
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanSection(row)
}

// GetBySlug retrieves a section by its URL slug.
func (r *SQLSectionRepository) GetBySlug(ctx context.Context, slug string) (*content.Section, error) {
	query := `
		SELECT id, slug, name, description, display_order, parent_id, is_active, created_at, updated_at
		FROM sections
		WHERE slug = $1;
	`
	row := r.db.QueryRowContext(ctx, query, slug)
	return r.scanSection(row)
}

// Update modifies an existing section.
func (r *SQLSectionRepository) Update(ctx context.Context, s *content.Section) error {
	query := `
		UPDATE sections SET
			slug = $1,
			name = $2,
			description = $3,
			display_order = $4,
			parent_id = $5,
			is_active = $6,
			updated_at = $7
		WHERE id = $8;
	`

	s.UpdatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx, query,
		s.Slug,
		s.Name,
		s.Description,
		s.DisplayOrder,
		s.ParentID,
		s.IsActive,
		s.UpdatedAt.UTC(),
		s.ID,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ports.ErrSectionSlugExists
		}
		return fmt.Errorf("failed updating section: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ports.ErrSectionNotFound
	}

	return nil
}

// Delete removes a section by ID.
func (r *SQLSectionRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM sections WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed deleting section: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ports.ErrSectionNotFound
	}
	return nil
}

// List returns sections ordered by display_order ASC, name ASC.
func (r *SQLSectionRepository) List(ctx context.Context, activeOnly bool) ([]content.Section, error) {
	var query string
	var rows *sql.Rows
	var err error

	if activeOnly {
		query = `
			SELECT id, slug, name, description, display_order, parent_id, is_active, created_at, updated_at
			FROM sections
			WHERE is_active = true
			ORDER BY display_order ASC, name ASC;
		`
		rows, err = r.db.QueryContext(ctx, query)
	} else {
		query = `
			SELECT id, slug, name, description, display_order, parent_id, is_active, created_at, updated_at
			FROM sections
			ORDER BY display_order ASC, name ASC;
		`
		rows, err = r.db.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed listing sections: %w", err)
	}
	defer rows.Close()

	var sections []content.Section
	for rows.Next() {
		var s content.Section
		err := rows.Scan(
			&s.ID,
			&s.Slug,
			&s.Name,
			&s.Description,
			&s.DisplayOrder,
			&s.ParentID,
			&s.IsActive,
			&s.CreatedAt,
			&s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed scanning section row: %w", err)
		}
		sections = append(sections, s)
	}

	if sections == nil {
		sections = []content.Section{}
	}

	return sections, nil
}

func (r *SQLSectionRepository) scanSection(row *sql.Row) (*content.Section, error) {
	var s content.Section
	err := row.Scan(
		&s.ID,
		&s.Slug,
		&s.Name,
		&s.Description,
		&s.DisplayOrder,
		&s.ParentID,
		&s.IsActive,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ports.ErrSectionNotFound
		}
		return nil, fmt.Errorf("failed scanning section: %w", err)
	}
	return &s, nil
}
