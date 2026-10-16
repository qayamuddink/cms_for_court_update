package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	liveenginev1 "github.com/livelaw/backend/contracts/proto/v1"
	grpcAdapter "github.com/livelaw/backend/internal/adapters/inbound/grpc"
	httpAdapter "github.com/livelaw/backend/internal/adapters/inbound/http"
	"github.com/livelaw/backend/internal/adapters/outbound/entitlement"
	"github.com/livelaw/backend/internal/adapters/outbound/filestore"
	"github.com/livelaw/backend/internal/adapters/outbound/hasher"
	"github.com/livelaw/backend/internal/adapters/outbound/idgen"
	"github.com/livelaw/backend/internal/adapters/outbound/liveengine"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	"github.com/livelaw/backend/internal/adapters/outbound/token"
	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/migrations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := getEnv("PORT", "8080")
	grpcPort := getEnv("GRPC_PORT", "9090")
	liveEngineMode := getEnv("LIVE_ENGINE_MODE", "inprocess")
	grpcAddr := getEnv("LIVE_ENGINE_GRPC_ADDR", "localhost:"+grpcPort)
	enableSSE := getEnv("ENABLE_SSE_STREAM", "true") == "true"
	dbURL := getEnv("DATABASE_URL", "postgres://livelaw:livelaw_secret@localhost:5432/livelaw?sslmode=disable")
	runMigrations := getEnv("RUN_MIGRATIONS", "true") == "true"
	seedData := getEnv("SEED_DATA", "true") == "true"
	jwtSecret := getEnv("JWT_SECRET", "dev-secret-super-long-key-for-livelaw-platform-signing-jwt-tokens")

	// S3 / MinIO configuration
	s3Endpoint := getEnv("S3_ENDPOINT", "http://localhost:9000")
	s3AccessKey := getEnv("S3_ACCESS_KEY", "minioadmin")
	s3SecretKey := getEnv("S3_SECRET_KEY", "minioadmin")
	s3Bucket := getEnv("S3_BUCKET", "livelaw-files")

	logger.Info("starting LiveLaw API server",
		slog.String("port", port),
		slog.String("grpc_port", grpcPort),
		slog.String("live_engine_mode", liveEngineMode),
		slog.Bool("enable_sse", enableSSE),
		slog.Bool("run_migrations", runMigrations),
		slog.Bool("seed_data", seedData),
		slog.String("s3_endpoint", s3Endpoint),
		slog.String("s3_bucket", s3Bucket),
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

	// MinIO object storage adapter
	fileStore, err := filestore.NewMinIOFileStore(filestore.Config{
		Endpoint:        s3Endpoint,
		AccessKeyID:     s3AccessKey,
		SecretAccessKey: s3SecretKey,
		BucketName:      s3Bucket,
		UseSSL:          false,
	})
	if err != nil {
		logger.Warn("failed to initialize MinIO file store, file operations may degrade", slog.Any("error", err))
	} else {
		initCtx, initCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := fileStore.EnsureBucket(initCtx); err != nil {
			logger.Warn("failed to ensure S3 bucket exists", slog.String("bucket", s3Bucket), slog.Any("error", err))
		} else {
			logger.Info("MinIO file storage connected and bucket verified", slog.String("bucket", s3Bucket))
		}
		initCancel()
	}

	// Core memory engine & gRPC server adapter
	memEngine := liveengine.NewMemoryLiveEngine(logger)
	grpcServer := grpc.NewServer()
	liveEngineServer := grpcAdapter.NewLiveEngineServer(memEngine, logger)
	liveenginev1.RegisterLiveEngineServiceServer(grpcServer, liveEngineServer)

	grpcListener, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Error("failed to bind gRPC port", slog.String("port", grpcPort), slog.Any("error", err))
		os.Exit(1)
	}
	go func() {
		logger.Info("gRPC LiveEngine server listening", slog.String("port", grpcPort))
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Error("gRPC server error", slog.Any("error", err))
		}
	}()

	// Wire LiveEnginePort according to LIVE_ENGINE_MODE (inprocess vs grpc)
	var liveEnginePort ports.LiveEnginePort
	var grpcConn *grpc.ClientConn
	if liveEngineMode == "grpc" {
		logger.Info("configuring LiveEnginePort using gRPC client", slog.String("addr", grpcAddr))
		conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Error("failed to connect to gRPC live engine", slog.String("addr", grpcAddr), slog.Any("error", err))
			os.Exit(1)
		}
		grpcConn = conn
		liveEnginePort = liveengine.NewGRPCLiveEngineClient(conn, logger)
	} else {
		logger.Info("configuring LiveEnginePort using in-process memory engine")
		liveEnginePort = liveengine.NewInProcessLiveEngine(memEngine)
	}

	// Repositories
	caseRepo := postgres.NewSQLCourtCaseRepository(db)
	updateRepo := postgres.NewSQLLiveUpdateRepository(db)
	assignmentRepo := postgres.NewSQLCaseAssignmentRepository(db)
	outboxRepo := postgres.NewSQLOutboxRepository(db)
	txMgr := postgres.NewSQLTransactionManager(db)
	eventPublisher := liveengine.NewLogEventPublisher(logger)
	articleRepo := postgres.NewSQLArticleRepository(db)
	entitlementChecker := entitlement.NewSQLSubscriptionChecker(db, userRepo)

	// Seed initial users if enabled
	if seedData {
		seeder := postgres.NewSeeder(userRepo, pwHasher, idGenerator, clock, logger)
		seedCtx, seedCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := seeder.SeedUsers(seedCtx); err != nil {
			logger.Warn("failed to run seeder on startup", slog.Any("error", err))
		}
		seedCancel()
	}

	// Start Outbox polling worker in background
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	outboxWorker := postgres.NewOutboxWorker(outboxRepo, eventPublisher, 200*time.Millisecond, clock, logger)
	go outboxWorker.Start(workerCtx)

	// Wire application use cases (domain and use cases NEVER change between modes!)
	authUseCases := commands.NewAuthUseCases(userRepo, tokenRepo, pwHasher, tokenService, idGenerator, clock)
	userQueries := queries.NewUserQueries(userRepo)
	liveCaseUseCases := commands.NewLiveCaseUseCases(caseRepo, updateRepo, assignmentRepo, outboxRepo, txMgr, liveEnginePort, userRepo, idGenerator, clock)
	liveCaseQueries := queries.NewLiveCaseQueries(caseRepo, updateRepo, assignmentRepo)
	articleUseCases := commands.NewArticleUseCases(articleRepo, idGenerator, clock, fileStore)
	articleQueries := queries.NewArticleQueries(articleRepo, entitlementChecker, fileStore)
	rateLimiter := httpAdapter.NewIPRateLimiter(20, 1*time.Minute)

	// Build inbound HTTP router
	router := httpAdapter.NewRouter(httpAdapter.RouterConfig{
		Logger:           logger,
		DBHealthChecker:  dbHealthChecker,
		Clock:            clock,
		TokenService:     tokenService,
		AuthUseCases:     authUseCases,
		UserQueries:      userQueries,
		LiveCaseUseCases: liveCaseUseCases,
		LiveCaseQueries:  liveCaseQueries,
		ArticleUseCases:  articleUseCases,
		ArticleQueries:   articleQueries,
		LiveEngine:       liveEnginePort,
		EnableSSE:        enableSSE,
		RateLimiter:      rateLimiter,
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
	logger.Info("shutting down servers gracefully...")
	workerCancel()           // Terminate outbox worker background loop
	grpcServer.GracefulStop() // Stop gRPC server
	if grpcConn != nil {
		_ = grpcConn.Close()
	}

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
