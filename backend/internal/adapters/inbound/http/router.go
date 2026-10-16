package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/identity"
)

// RouterConfig holds all dependencies required to build the HTTP router.
type RouterConfig struct {
	Logger          *slog.Logger
	DBHealthChecker ports.DBHealthChecker
	Clock           ports.Clock
	TokenService    ports.TokenService
	AuthUseCases    *commands.AuthUseCases
	UserQueries     *queries.UserQueries
	RateLimiter     *IPRateLimiter
}

// NewRouter builds the HTTP handler with all routes, rate limiters, and middlewares.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// 1. System Health
	healthHandler := NewHealthHandler(cfg.DBHealthChecker, cfg.Clock)
	mux.Handle("GET /healthz", healthHandler)

	// Rate limiter (default 10 requests per minute per IP for auth endpoints)
	limiter := cfg.RateLimiter
	if limiter == nil {
		limiter = NewIPRateLimiter(10, 1*time.Minute)
	}

	// 2. Auth endpoints
	if cfg.AuthUseCases != nil && cfg.UserQueries != nil {
		authHandler := NewAuthHandler(cfg.AuthUseCases, cfg.UserQueries)

		mux.HandleFunc("POST /v1/auth/register", limiter.Limit(authHandler.Register))
		mux.HandleFunc("POST /v1/auth/login", limiter.Limit(authHandler.Login))
		mux.HandleFunc("POST /v1/auth/refresh", authHandler.Refresh)
		mux.HandleFunc("POST /v1/auth/logout", authHandler.Logout)
		mux.HandleFunc("GET /v1/me", RequireAuth(authHandler.Me))

		cmsUserHandler := NewCMSUserHandler(cfg.AuthUseCases, cfg.UserQueries)
		mux.HandleFunc("POST /v1/cms/users", RequireRoles(identity.RoleAdmin)(cmsUserHandler.CreateStaff))
		mux.HandleFunc("GET /v1/cms/users", RequireRoles(identity.RoleAdmin)(cmsUserHandler.ListUsers))
		mux.HandleFunc("PATCH /v1/cms/users/{id}", RequireRoles(identity.RoleAdmin)(cmsUserHandler.UpdateUser))
	}

	// Global middleware chain (outer to inner execution)
	handler := http.Handler(mux)
	if cfg.TokenService != nil {
		handler = AuthenticateMiddleware(cfg.TokenService)(handler)
	}
	handler = RecoveryMiddleware(cfg.Logger)(handler)
	handler = LoggingMiddleware(cfg.Logger)(handler)
	handler = RequestIDMiddleware(handler)
	handler = CORSMiddleware(handler)

	return handler
}
