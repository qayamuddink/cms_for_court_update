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
	LiveCaseQueries       *queries.LiveCaseQueries
	ArticleUseCases       *commands.ArticleUseCases
	ArticleQueries        *queries.ArticleQueries
	SubscriptionUseCases  *commands.SubscriptionUseCases
	SubscriptionQueries   *queries.SubscriptionQueries
	FollowUseCases        *commands.FollowUseCases
	FollowQueries         *queries.FollowQueries
	LiveEngine            ports.LiveEnginePort
	EnableSSE             bool
	RateLimiter           *IPRateLimiter
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

	// 5. Articles & Judgments Public endpoints
	if cfg.ArticleQueries != nil {
		articlePublicHandler := NewArticlePublicHandler(cfg.ArticleQueries)
		mux.HandleFunc("GET /v1/articles", articlePublicHandler.ListFeed)
		mux.HandleFunc("GET /v1/articles/{slug}", articlePublicHandler.GetBySlug)
		mux.HandleFunc("GET /v1/judgments/{id}", articlePublicHandler.GetJudgment)
		mux.HandleFunc("GET /v1/cases/{id}/judgment", articlePublicHandler.GetCaseJudgment)
	}

	// 6. Articles & Judgments CMS endpoints (Staff only)
	if cfg.ArticleUseCases != nil && cfg.ArticleQueries != nil {
		cmsArticleHandler := NewCMSArticleHandler(cfg.ArticleUseCases, cfg.ArticleQueries)

		// Articles CRUD and workflow
		mux.HandleFunc("POST /v1/cms/articles", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.CreateDraft))
		mux.HandleFunc("GET /v1/cms/articles", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.List))
		mux.HandleFunc("GET /v1/cms/articles/{id}", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.Get))
		mux.HandleFunc("PATCH /v1/cms/articles/{id}", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.UpdateDraft))
		mux.HandleFunc("POST /v1/cms/articles/{id}/submit", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.SubmitForReview))
		mux.HandleFunc("POST /v1/cms/articles/{id}/send-back", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.SendBack))
		mux.HandleFunc("POST /v1/cms/articles/{id}/publish", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.Publish))
		mux.HandleFunc("POST /v1/cms/articles/{id}/archive", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.Archive))
		mux.HandleFunc("POST /v1/cms/articles/{id}/attachment", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.AttachPDF))

		// Judgments and Upload URLs
		mux.HandleFunc("POST /v1/cms/judgments", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.CreateJudgment))
		mux.HandleFunc("POST /v1/cms/uploads/presigned-url", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.RequestUploadURL))
	}

	// 7. Subscriptions & Payment Webhooks
	if cfg.SubscriptionUseCases != nil {
		webhookHandler := NewPaymentWebhookHandler(cfg.SubscriptionUseCases)
		mux.HandleFunc("POST /v1/webhooks/payments", webhookHandler.HandleWebhook)
	}

	if cfg.SubscriptionQueries != nil {
		readerSubHandler := NewReaderSubscriptionHandler(cfg.SubscriptionQueries)
		mux.HandleFunc("GET /v1/me/subscription", RequireAuth(readerSubHandler.GetMySubscription))
	}

	if cfg.SubscriptionUseCases != nil && cfg.SubscriptionQueries != nil {
		cmsSubHandler := NewCMSSubscriptionHandler(cfg.SubscriptionUseCases, cfg.SubscriptionQueries)
		mux.HandleFunc("GET /v1/cms/subscriptions", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsSubHandler.ListSubscriptions))
		mux.HandleFunc("POST /v1/cms/subscriptions/grant", RequireRoles(identity.RoleAdmin)(cmsSubHandler.GrantSubscription))
		mux.HandleFunc("POST /v1/cms/subscriptions/{id}/cancel", RequireRoles(identity.RoleAdmin)(cmsSubHandler.CancelSubscription))
	}

	// 8. Case Follows & Devices
	if cfg.FollowUseCases != nil && cfg.FollowQueries != nil {
		followHandler := NewFollowHandler(cfg.FollowUseCases, cfg.FollowQueries)
		mux.HandleFunc("POST /v1/cases/{id}/follow", RequireAuth(followHandler.FollowCase))
		mux.HandleFunc("DELETE /v1/cases/{id}/follow", RequireAuth(followHandler.UnfollowCase))
		mux.HandleFunc("GET /v1/cases/{id}/follow", followHandler.GetCaseFollowStatus)

		mux.HandleFunc("POST /v1/devices", RequireAuth(followHandler.RegisterDevice))
		mux.HandleFunc("DELETE /v1/devices", RequireAuth(followHandler.UnregisterDevice))
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
