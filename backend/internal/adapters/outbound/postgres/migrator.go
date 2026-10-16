package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Migration represents a single migration file pair.
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

// Migrator manages applying and reverting database migrations.
type Migrator struct {
	db *sql.DB
	fs fs.FS
}

// NewMigrator creates a new Migrator instance.
func NewMigrator(db *sql.DB, migrationFS fs.FS) *Migrator {
	return &Migrator{
		db: db,
		fs: migrationFS,
	}
}

// EnsureSchemaTable creates the schema_migrations table if it doesn't exist.
func (m *Migrator) EnsureSchemaTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')
		);
	`
	_, err := m.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to ensure schema_migrations table: %w", err)
	}
	return nil
}

// LoadMigrations parses all .up.sql and .down.sql files from the filesystem.
func (m *Migrator) LoadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(m.fs, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	migrationsMap := make(map[int]*Migration)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if !strings.HasSuffix(filename, ".sql") {
			continue
		}

		parts := strings.SplitN(filename, "_", 2)
		if len(parts) < 2 {
			continue
		}

		ver, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		baseName := strings.TrimSuffix(strings.TrimSuffix(parts[1], ".up.sql"), ".down.sql")
		if _, exists := migrationsMap[ver]; !exists {
			migrationsMap[ver] = &Migration{
				Version: ver,
				Name:    baseName,
			}
		}

		content, err := fs.ReadFile(m.fs, filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
		}

		if strings.HasSuffix(filename, ".up.sql") {
			migrationsMap[ver].UpSQL = string(content)
		} else if strings.HasSuffix(filename, ".down.sql") {
			migrationsMap[ver].DownSQL = string(content)
		}
	}

	var migrations []Migration
	for _, mig := range migrationsMap {
		migrations = append(migrations, *mig)
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// GetAppliedVersions returns all versions currently recorded in schema_migrations.
func (m *Migrator) GetAppliedVersions(ctx context.Context) (map[int]time.Time, error) {
	if err := m.EnsureSchemaTable(ctx); err != nil {
		return nil, err
	}

	rows, err := m.db.QueryContext(ctx, `SELECT version, applied_at FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]time.Time)
	for rows.Next() {
		var ver int
		var appliedAt time.Time
		if err := rows.Scan(&ver, &appliedAt); err != nil {
			return nil, fmt.Errorf("failed to scan migration row: %w", err)
		}
		applied[ver] = appliedAt
	}

	return applied, rows.Err()
}

// Up applies all pending migrations in ascending order.
func (m *Migrator) Up(ctx context.Context) error {
	applied, err := m.GetAppliedVersions(ctx)
	if err != nil {
		return err
	}

	migrations, err := m.LoadMigrations()
	if err != nil {
		return err
	}

	for _, mig := range migrations {
		if _, ok := applied[mig.Version]; ok {
			continue
		}

		if mig.UpSQL == "" {
			return fmt.Errorf("missing up SQL for migration version %d", mig.Version)
		}

		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", mig.Version, err)
		}

		if _, err := tx.ExecContext(ctx, mig.UpSQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed executing migration %d up (%s): %w", mig.Version, mig.Name, err)
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, mig.Version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed recording migration %d in schema_migrations: %w", mig.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", mig.Version, err)
		}
	}

	return nil
}

// Down rolls back the most recently applied migration.
func (m *Migrator) Down(ctx context.Context) error {
	applied, err := m.GetAppliedVersions(ctx)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		return nil // No migrations to revert
	}

	migrations, err := m.LoadMigrations()
	if err != nil {
		return err
	}

	// Sort descending for rollback
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version > migrations[j].Version
	})

	for _, mig := range migrations {
		if _, ok := applied[mig.Version]; !ok {
			continue
		}

		if mig.DownSQL == "" {
			return fmt.Errorf("missing down SQL for migration version %d", mig.Version)
		}

		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration rollback %d: %w", mig.Version, err)
		}

		if _, err := tx.ExecContext(ctx, mig.DownSQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed executing migration %d down: %w", mig.Version, err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, mig.Version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed deleting migration %d from schema_migrations: %w", mig.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit rollback %d: %w", mig.Version, err)
		}

		// Rolled back 1 migration
		break
	}

	return nil
}

// DownAll rolls back all applied migrations in descending order.
func (m *Migrator) DownAll(ctx context.Context) error {
	for {
		applied, err := m.GetAppliedVersions(ctx)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			break
		}
		if err := m.Down(ctx); err != nil {
			return err
		}
	}
	return nil
}

// MigrationStatus reports status of a migration.
type MigrationStatus struct {
	Version   int
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

// Status returns the list of all migrations and their current applied state.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	applied, err := m.GetAppliedVersions(ctx)
	if err != nil {
		return nil, err
	}

	migrations, err := m.LoadMigrations()
	if err != nil {
		return nil, err
	}

	var statuses []MigrationStatus
	for _, mig := range migrations {
		var appliedAt *time.Time
		isApplied := false
		if t, ok := applied[mig.Version]; ok {
			isApplied = true
			appliedAt = &t
		}
		statuses = append(statuses, MigrationStatus{
			Version:   mig.Version,
			Name:      filepath.Base(mig.Name),
			Applied:   isApplied,
			AppliedAt: appliedAt,
		})
	}

	return statuses, nil
}
