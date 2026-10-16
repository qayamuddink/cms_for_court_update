package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/livelaw/backend/internal/domain/identity"
)

// SQLUserRepository implements ports.UserRepository using PostgreSQL.
type SQLUserRepository struct {
	db *sql.DB
}

// NewSQLUserRepository creates a new SQLUserRepository.
func NewSQLUserRepository(db *sql.DB) *SQLUserRepository {
	return &SQLUserRepository{db: db}
}

// Create inserts a new user record.
func (r *SQLUserRepository) Create(ctx context.Context, user *identity.User) error {
	query := `
		INSERT INTO users (
			id, email, password_hash, auth_provider, full_name,
			display_role, system_role, bar_council_number, phone,
			email_verified_at, status, last_login_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13, $14
		);
	`

	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		identity.NormalizeEmail(user.Email),
		user.PasswordHash,
		user.AuthProvider,
		user.FullName,
		string(user.DisplayRole),
		string(user.SystemRole),
		user.BarCouncilNumber,
		user.Phone,
		user.EmailVerifiedAt,
		string(user.Status),
		user.LastLoginAt,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

// GetByID retrieves a user by ID.
func (r *SQLUserRepository) GetByID(ctx context.Context, id string) (*identity.User, error) {
	query := `
		SELECT
			id, email, password_hash, auth_provider, full_name,
			display_role, system_role, bar_council_number, phone,
			email_verified_at, status, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1;
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanUser(row)
}

// GetByEmail retrieves a user by normalized lowercase email.
func (r *SQLUserRepository) GetByEmail(ctx context.Context, email string) (*identity.User, error) {
	query := `
		SELECT
			id, email, password_hash, auth_provider, full_name,
			display_role, system_role, bar_council_number, phone,
			email_verified_at, status, last_login_at, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1);
	`

	row := r.db.QueryRowContext(ctx, query, identity.NormalizeEmail(email))
	return r.scanUser(row)
}

// Update updates mutable user fields (full_name, display_role, system_role, status, bar_council_number, phone, updated_at).
func (r *SQLUserRepository) Update(ctx context.Context, user *identity.User) error {
	query := `
		UPDATE users SET
			full_name = $1,
			display_role = $2,
			system_role = $3,
			status = $4,
			bar_council_number = $5,
			phone = $6,
			updated_at = $7
		WHERE id = $8;
	`

	res, err := r.db.ExecContext(ctx, query,
		user.FullName,
		string(user.DisplayRole),
		string(user.SystemRole),
		string(user.Status),
		user.BarCouncilNumber,
		user.Phone,
		user.UpdatedAt,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("user not found")
	}

	return nil
}

// UpdateLastLogin records last login timestamp.
func (r *SQLUserRepository) UpdateLastLogin(ctx context.Context, id string, lastLogin time.Time) error {
	query := `UPDATE users SET last_login_at = $1 WHERE id = $2;`
	_, err := r.db.ExecContext(ctx, query, lastLogin, id)
	return err
}

// List retrieves users with keyset pagination (ordered by created_at DESC, id DESC).
func (r *SQLUserRepository) List(ctx context.Context, limit int, beforeCreatedAt *time.Time, beforeID *string) ([]identity.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var query string
	var args []any

	if beforeCreatedAt != nil && beforeID != nil {
		query = `
			SELECT
				id, email, password_hash, auth_provider, full_name,
				display_role, system_role, bar_council_number, phone,
				email_verified_at, status, last_login_at, created_at, updated_at
			FROM users
			WHERE (created_at, id) < ($1, $2)
			ORDER BY created_at DESC, id DESC
			LIMIT $3;
		`
		args = []any{*beforeCreatedAt, *beforeID, limit}
	} else {
		query = `
			SELECT
				id, email, password_hash, auth_provider, full_name,
				display_role, system_role, bar_council_number, phone,
				email_verified_at, status, last_login_at, created_at, updated_at
			FROM users
			ORDER BY created_at DESC, id DESC
			LIMIT $1;
		`
		args = []any{limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []identity.User
	for rows.Next() {
		u, err := r.scanUserFromRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}

	return users, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func (r *SQLUserRepository) scanUser(s scannable) (*identity.User, error) {
	var u identity.User
	var displayRole, systemRole, status string

	err := s.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.AuthProvider,
		&u.FullName,
		&displayRole,
		&systemRole,
		&u.BarCouncilNumber,
		&u.Phone,
		&u.EmailVerifiedAt,
		&status,
		&u.LastLoginAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found returns nil, nil
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	u.DisplayRole = identity.DisplayRole(displayRole)
	u.SystemRole = identity.SystemRole(systemRole)
	u.Status = identity.UserStatus(status)

	return &u, nil
}

func (r *SQLUserRepository) scanUserFromRows(rows *sql.Rows) (*identity.User, error) {
	return r.scanUser(rows)
}
