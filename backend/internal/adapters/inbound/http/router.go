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
	Logger           *slog.Logger
	DBHealthChecker  ports.DBHealthChecker
	Clock            ports.Clock
	TokenService     ports.TokenService
	AuthUseCases     *commands.AuthUseCases
	UserQueries      *queries.UserQueries
	LiveCaseUseCases *commands.LiveCaseUseCases
	LiveCaseQueries  *queries.LiveCaseQueries
	LiveEngine       ports.LiveEnginePort
	EnableSSE        bool
	RateLimiter      *IPRateLimiter
}

// NewRouter builds the HTTP handler with all routes, rate limiters, and middlewares.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// 1. System Health
	healthHandler := NewHealthHandler(cfg.DBHealthChecker, cfg.Clock)
	mux.Handle("GET /healthz", healthHandler)

	// Rate limiter (default 10 requests per minute per IP for sensitive auth endpoints)
	limiter := cfg.RateLimiter
	if limiter == nil {
		limiter = NewIPRateLimiter(10, 1*time.Minute)
	}

	// 2. Auth & User endpoints
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

	// 3. Live Case Public endpoints
	if cfg.LiveCaseQueries != nil {
		liveHandler := NewLiveCasePublicHandler(cfg.LiveCaseQueries, cfg.LiveEngine)
		mux.HandleFunc("GET /v1/cases/live", liveHandler.ListLiveCases)
		mux.HandleFunc("GET /v1/cases/{id}", liveHandler.GetCase)
		mux.HandleFunc("GET /v1/cases/{id}/updates", liveHandler.GetUpdates)
		mux.HandleFunc("GET /v1/cases/{id}/updates/latest", liveHandler.GetLatestUpdates)
		if cfg.EnableSSE {
			mux.HandleFunc("GET /v1/cases/{id}/stream", liveHandler.StreamUpdates)
		}
	}

	// 4. Live Case CMS endpoints (Staff only)
	if cfg.LiveCaseUseCases != nil && cfg.LiveCaseQueries != nil {
		cmsCaseHandler := NewCMSCaseHandler(cfg.LiveCaseUseCases, cfg.LiveCaseQueries)

		// Case administration (Editor, Admin)
		mux.HandleFunc("POST /v1/cms/cases", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsCaseHandler.CreateCase))
		mux.HandleFunc("PATCH /v1/cms/cases/{id}", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsCaseHandler.UpdateCase))
		mux.HandleFunc("POST /v1/cms/cases/{id}/live", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsCaseHandler.SetCaseLive))
		mux.HandleFunc("POST /v1/cms/cases/{id}/assign", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsCaseHandler.AssignReporter))
		mux.HandleFunc("DELETE /v1/cms/cases/{id}/assign/{userId}", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsCaseHandler.UnassignReporter))
		mux.HandleFunc("GET /v1/cms/cases/{id}/reporters", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsCaseHandler.ListReporters))

		// Live updates posting, correction, retraction (Assigned Reporter, Editor, Admin)
		mux.HandleFunc("POST /v1/cms/cases/{id}/updates", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsCaseHandler.PublishUpdate))
		mux.HandleFunc("POST /v1/cms/cases/{id}/updates/{updateId}/correct", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsCaseHandler.CorrectUpdate))
		mux.HandleFunc("POST /v1/cms/cases/{id}/updates/{updateId}/retract", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsCaseHandler.RetractUpdate))
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
