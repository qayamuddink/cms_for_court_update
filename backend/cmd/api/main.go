package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	httpAdapter "github.com/livelaw/backend/internal/adapters/inbound/http"
	"github.com/livelaw/backend/internal/adapters/outbound/hasher"
	"github.com/livelaw/backend/internal/adapters/outbound/idgen"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	"github.com/livelaw/backend/internal/adapters/outbound/token"
	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/migrations"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://livelaw:livelaw_secret@localhost:5432/livelaw?sslmode=disable")
	runMigrations := getEnv("RUN_MIGRATIONS", "true") == "true"
	seedData := getEnv("SEED_DATA", "true") == "true"
	jwtSecret := getEnv("JWT_SECRET", "dev-secret-super-long-key-for-livelaw-platform-signing-jwt-tokens")

	logger.Info("starting LiveLaw API server",
		slog.String("port", port),
		slog.Bool("run_migrations", runMigrations),
		slog.Bool("seed_data", seedData),
	)

	// Initialize database connection
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		logger.Error("failed to open database handle", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	// Tune connection pool for high concurrency
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(15 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Check database connectivity
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.PingContext(pingCtx); err != nil {
		logger.Warn("database ping failed on startup, server will report degraded health", slog.Any("error", err))
	} else {
		logger.Info("connected to PostgreSQL successfully")
	}
	pingCancel()

	// Run migrations if enabled
	if runMigrations {
		migrator := postgres.NewMigrator(db, migrations.FS)
		migCtx, migCancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := migrator.Up(migCtx); err != nil {
			logger.Error("failed to apply migrations on startup", slog.Any("error", err))
			os.Exit(1)
		}
		migCancel()
		logger.Info("database migrations verified and applied")
	}

	// Wire infrastructure adapters
	clock := ports.RealClock{}
	idGenerator := idgen.NewUUIDv7Generator()
	dbHealthChecker := postgres.NewHealthChecker(db)
	userRepo := postgres.NewSQLUserRepository(db)
	tokenRepo := postgres.NewSQLRefreshTokenRepository(db)
	pwHasher := hasher.NewBcryptHasher(12)
	tokenService := token.NewJWTService(jwtSecret, "livelaw-platform", 15*time.Minute, 7*24*time.Hour, clock)

	// Seed initial users if enabled
	if seedData {
		seeder := postgres.NewSeeder(userRepo, pwHasher, idGenerator, clock, logger)
		seedCtx, seedCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := seeder.SeedUsers(seedCtx); err != nil {
			logger.Warn("failed to run seeder on startup", slog.Any("error", err))
		}
		seedCancel()
	}

	// Wire application use cases
	authUseCases := commands.NewAuthUseCases(userRepo, tokenRepo, pwHasher, tokenService, idGenerator, clock)
	userQueries := queries.NewUserQueries(userRepo)
	rateLimiter := httpAdapter.NewIPRateLimiter(20, 1*time.Minute)

	// Build inbound HTTP router
	router := httpAdapter.NewRouter(httpAdapter.RouterConfig{
		Logger:          logger,
		DBHealthChecker: dbHealthChecker,
		Clock:           clock,
		TokenService:    tokenService,
		AuthUseCases:    authUseCases,
		UserQueries:     userQueries,
		RateLimiter:     rateLimiter,
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server shutdown handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("HTTP server listening", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-stop
	logger.Info("shutting down HTTP server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server forced to shutdown", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("server exited cleanly")
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
