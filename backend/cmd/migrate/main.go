package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	"github.com/livelaw/backend/migrations"
)

func main() {
	var dbURL string
	var timeout time.Duration

	flag.StringVar(&dbURL, "database-url", "", "PostgreSQL connection string (or via DATABASE_URL env)")
	flag.DurationVar(&timeout, "timeout", 30*time.Second, "Database operation timeout")
	flag.Parse()

	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		dbURL = "postgres://livelaw:livelaw_secret@localhost:5432/livelaw?sslmode=disable"
	}

	command := "up"
	if len(flag.Args()) > 0 {
		command = flag.Args()[0]
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}

	migrator := postgres.NewMigrator(db, migrations.FS)

	switch command {
	case "up":
		log.Println("Applying migrations...")
		if err := migrator.Up(ctx); err != nil {
			log.Fatalf("migration up error: %v", err)
		}
		log.Println("Migrations applied successfully.")

	case "down":
		log.Println("Rolling back last migration...")
		if err := migrator.Down(ctx); err != nil {
			log.Fatalf("migration down error: %v", err)
		}
		log.Println("Rollback completed successfully.")

	case "down-all":
		log.Println("Rolling back all migrations...")
		if err := migrator.DownAll(ctx); err != nil {
			log.Fatalf("migration down-all error: %v", err)
		}
		log.Println("All migrations rolled back successfully.")

	case "reset":
		log.Println("Resetting all migrations...")
		if err := migrator.DownAll(ctx); err != nil {
			log.Fatalf("migration reset down error: %v", err)
		}
		if err := migrator.Up(ctx); err != nil {
			log.Fatalf("migration reset up error: %v", err)
		}
		log.Println("Migrations reset successfully.")

	case "status":
		statuses, err := migrator.Status(ctx)
		if err != nil {
			log.Fatalf("migration status error: %v", err)
		}
		fmt.Printf("%-8s | %-35s | %-10s | %s\n", "VERSION", "NAME", "APPLIED", "APPLIED AT")
		fmt.Println("-------------------------------------------------------------------------------")
		for _, s := range statuses {
			appliedStr := "NO"
			appliedAtStr := "-"
			if s.Applied {
				appliedStr = "YES"
				if s.AppliedAt != nil {
					appliedAtStr = s.AppliedAt.Format(time.RFC3339)
				}
			}
			fmt.Printf("%-8d | %-35s | %-10s | %s\n", s.Version, s.Name, appliedStr, appliedAtStr)
		}

	default:
		log.Fatalf("unknown command %q (expected: up, down, down-all, reset, status)", command)
	}
}
