package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/livelaw/backend/migrations"
)

func TestMigratorLoadMigrations(t *testing.T) {
	migrator := NewMigrator(nil, migrations.FS)
	migs, err := migrator.LoadMigrations()
	if err != nil {
		t.Fatalf("failed to load migrations from embed.FS: %v", err)
	}

	if len(migs) < 9 {
		t.Fatalf("expected at least 9 migrations, got %d", len(migs))
	}

	for i, m := range migs {
		expectedVer := i + 1
		if m.Version != expectedVer {
			t.Errorf("migration at index %d has version %d, expected %d", i, m.Version, expectedVer)
		}
		if m.UpSQL == "" {
			t.Errorf("migration %d (%s) has empty UpSQL", m.Version, m.Name)
		}
		if m.DownSQL == "" {
			t.Errorf("migration %d (%s) has empty DownSQL", m.Version, m.Name)
		}
	}
}

func TestMigratorIntegration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("Skipping postgres integration test: DATABASE_URL not set")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Skipping postgres integration test: database ping failed: %v", err)
	}

	migrator := NewMigrator(db, migrations.FS)

	// Test 1: Clean rollback of everything
	if err := migrator.DownAll(ctx); err != nil {
		t.Fatalf("failed to down-all migrations: %v", err)
	}

	// Test 2: Apply all migrations
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("failed to apply migrations up: %v", err)
	}

	// Verify all expected tables exist
	expectedTables := []string{
		"users",
		"court_cases",
		"articles",
		"live_updates",
		"subscriptions",
		"case_assignments",
		"outbox_events",
		"case_follows",
		"device_tokens",
		"refresh_tokens",
	}

	for _, table := range expectedTables {
		var exists bool
		query := `SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		);`
		if err := db.QueryRowContext(ctx, query, table).Scan(&exists); err != nil {
			t.Fatalf("failed to query table existence for %s: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %s to exist after migration up", table)
		}
	}

	// Test 3: Status check
	statuses, err := migrator.Status(ctx)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	for _, s := range statuses {
		if !s.Applied {
			t.Errorf("expected migration %d (%s) to be marked applied", s.Version, s.Name)
		}
	}

	// Test 4: Down all (clean teardown)
	if err := migrator.DownAll(ctx); err != nil {
		t.Fatalf("failed to down all migrations: %v", err)
	}

	// Verify tables dropped
	for _, table := range expectedTables {
		var exists bool
		query := `SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		);`
		if err := db.QueryRowContext(ctx, query, table).Scan(&exists); err != nil {
			t.Fatalf("failed to query table existence for %s: %v", table, err)
		}
		if exists {
			t.Errorf("expected table %s to be dropped after migration down-all", table)
		}
	}

	// Re-apply for clean ending state
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("failed to re-apply migrations up: %v", err)
	}
}
