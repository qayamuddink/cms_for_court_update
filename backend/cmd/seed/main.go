package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/livelaw/backend/internal/adapters/outbound/hasher"
	"github.com/livelaw/backend/internal/adapters/outbound/idgen"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	"github.com/livelaw/backend/internal/application/ports"
)

func main() {
	var dbURL string
	flag.StringVar(&dbURL, "database-url", "", "PostgreSQL connection string (or via DATABASE_URL env)")
	flag.Parse()

	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		dbURL = "postgres://livelaw:livelaw_secret@localhost:5432/livelaw?sslmode=disable"
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}

	userRepo := postgres.NewSQLUserRepository(db)
	pwHasher := hasher.NewBcryptHasher(12)
	idGenerator := idgen.NewUUIDv7Generator()
	clock := ports.RealClock{}

	seeder := postgres.NewSeeder(userRepo, pwHasher, idGenerator, clock, logger)

	logger.Info("seeding default accounts...")
	if err := seeder.SeedUsers(ctx); err != nil {
		log.Fatalf("failed to seed users: %v", err)
	}

	logger.Info("seeding completed successfully!")
}
