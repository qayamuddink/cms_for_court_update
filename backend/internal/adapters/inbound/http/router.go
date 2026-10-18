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
	Logger               *slog.Logger
	DBHealthChecker      ports.DBHealthChecker
	Clock                ports.Clock
	TokenService         ports.TokenService
	AuthUseCases         *commands.AuthUseCases
	UserQueries          *queries.UserQueries
	LiveCaseUseCases     *commands.LiveCaseUseCases
	LiveCaseQueries      *queries.LiveCaseQueries
	ArticleUseCases      *commands.ArticleUseCases
	ArticleQueries       *queries.ArticleQueries
	SectionUseCases      *commands.SectionUseCases
	SubscriptionUseCases *commands.SubscriptionUseCases
	SubscriptionQueries  *queries.SubscriptionQueries
	JudgmentUseCases     *commands.JudgmentUseCases
	JudgmentQueries      *queries.JudgmentQueryUseCases
	SearchQueries        *queries.SearchQueries
	SearchIndexer        *commands.SearchIndexer
	DigestCommands       *commands.DigestCommands
	DigestQueries        *queries.DigestQueries
	FollowUseCases       *commands.FollowUseCases
	FollowQueries        *queries.FollowQueries
	LiveEngine           ports.LiveEnginePort
	EnableSSE            bool
	RateLimiter          *IPRateLimiter
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

	// 5. Taxonomy Sections Public & CMS endpoints
	if cfg.SectionUseCases != nil {
		sectionHandler := NewSectionHandler(cfg.SectionUseCases)
		mux.HandleFunc("GET /v1/sections", sectionHandler.ListPublic)
		mux.HandleFunc("GET /v1/sections/{slug}", sectionHandler.GetBySlug)

		mux.HandleFunc("GET /v1/cms/sections", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(sectionHandler.ListCMS))
		mux.HandleFunc("POST /v1/cms/sections", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(sectionHandler.Create))
		mux.HandleFunc("PUT /v1/cms/sections/{id}", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(sectionHandler.Update))
		mux.HandleFunc("DELETE /v1/cms/sections/{id}", RequireRoles(identity.RoleAdmin)(sectionHandler.Delete))
	}

	// 6. Articles & Judgments Public endpoints
	if cfg.ArticleQueries != nil {
		articlePublicHandler := NewArticlePublicHandler(cfg.ArticleQueries)
		mux.HandleFunc("GET /v1/articles", articlePublicHandler.ListFeed)
		mux.HandleFunc("GET /v1/articles/{slug}", articlePublicHandler.GetBySlug)
		mux.HandleFunc("GET /v1/articles/{id}/related", articlePublicHandler.GetRelated)
		mux.HandleFunc("GET /v1/top-stories", articlePublicHandler.ListTopStories)
		if cfg.JudgmentQueries == nil {
			mux.HandleFunc("GET /v1/judgments/{id}", articlePublicHandler.GetJudgment)
		}
		mux.HandleFunc("GET /v1/cases/{id}/judgment", articlePublicHandler.GetCaseJudgment)
	}

	// 7. Articles & Judgments CMS endpoints (Staff only)
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

		// Phase 9: Scheduling, Soft-delete, Restore, Revisions & Top Stories
		mux.HandleFunc("POST /v1/cms/articles/{id}/schedule", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.Schedule))
		mux.HandleFunc("DELETE /v1/cms/articles/{id}", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.SoftDelete))
		mux.HandleFunc("POST /v1/cms/articles/{id}/restore", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.Restore))
		mux.HandleFunc("GET /v1/cms/articles/{id}/revisions", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.ListRevisions))
		mux.HandleFunc("GET /v1/cms/articles/{id}/revisions/{version}", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.GetRevision))
		mux.HandleFunc("POST /v1/cms/articles/{id}/revisions/{version}/restore", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.RestoreRevision))
		mux.HandleFunc("PUT /v1/cms/top-stories", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.UpdateTopStories))

		// Judgments and Upload URLs
		if cfg.JudgmentUseCases == nil {
			mux.HandleFunc("POST /v1/cms/judgments", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.CreateJudgmentSummary))
		}
		mux.HandleFunc("POST /v1/cms/uploads/presigned-url", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(cmsArticleHandler.PresignUpload))
	}

	// 7b. Judgments, Citations, Headnotes & PDF Downloads (Phase 10)
	if cfg.JudgmentQueries != nil || cfg.JudgmentUseCases != nil {
		judgmentHandler := NewJudgmentHandler(cfg.JudgmentUseCases, cfg.JudgmentQueries, cfg.ArticleUseCases, cfg.ArticleQueries)

		if cfg.JudgmentQueries != nil {
			mux.HandleFunc("GET /v1/judgments", judgmentHandler.ListJudgments)
			mux.HandleFunc("GET /v1/judgments/{id}", judgmentHandler.GetJudgment)
			mux.HandleFunc("GET /v1/judgments/{id}/pages", judgmentHandler.GetJudgmentPages)
			mux.HandleFunc("GET /v1/judgments/{id}/download", judgmentHandler.GetDownloadURL)
			mux.HandleFunc("GET /v1/citations/{citation...}", judgmentHandler.GetJudgmentByCitation)
		}

		if cfg.JudgmentUseCases != nil {
			mux.HandleFunc("POST /v1/cms/judgments", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(judgmentHandler.CreateDraft))
			mux.HandleFunc("GET /v1/cms/judgments", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(judgmentHandler.ListCMS))
			mux.HandleFunc("GET /v1/cms/judgments/{id}", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(judgmentHandler.GetCMS))
			mux.HandleFunc("PUT /v1/cms/judgments/{id}", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(judgmentHandler.Update))
			mux.HandleFunc("POST /v1/cms/judgments/{id}/pdf", RequireRoles(identity.RoleReporter, identity.RoleEditor, identity.RoleAdmin)(judgmentHandler.AttachPDF))
			mux.HandleFunc("POST /v1/cms/judgments/{id}/publish", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(judgmentHandler.Publish))
		}
	}

	// 8. Subscriptions & Payment Webhooks
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

	// 9. Case Follows & Push Device Registration
	if cfg.FollowUseCases != nil && cfg.FollowQueries != nil {
		followHandler := NewFollowHandler(cfg.FollowUseCases, cfg.FollowQueries)
		mux.HandleFunc("POST /v1/cases/{id}/follow", RequireAuth(followHandler.FollowCase))
		mux.HandleFunc("DELETE /v1/cases/{id}/follow", RequireAuth(followHandler.UnfollowCase))
		mux.HandleFunc("GET /v1/cases/{id}/follow", followHandler.GetCaseFollowStatus)
		mux.HandleFunc("POST /v1/devices", RequireAuth(followHandler.RegisterDevice))
		mux.HandleFunc("DELETE /v1/devices", RequireAuth(followHandler.UnregisterDevice))
		mux.HandleFunc("POST /v1/devices/register", RequireAuth(followHandler.RegisterDevice))
	}

	// 10. Search, Autosuggest & Analytics endpoints (Phase 11)
	if cfg.SearchQueries != nil {
		searchHandler := NewSearchHandler(cfg.SearchQueries, cfg.SearchIndexer)
		mux.HandleFunc("GET /v1/search", searchHandler.Search)
		mux.HandleFunc("GET /v1/search/autosuggest", searchHandler.Autosuggest)
		mux.HandleFunc("GET /v1/cms/search/health", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(searchHandler.GetHealth))
		mux.HandleFunc("POST /v1/cms/search/reindex", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(searchHandler.Reindex))
		mux.HandleFunc("GET /v1/cms/search/analytics", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(searchHandler.GetAnalytics))
	}

	// 11. Digests & Round-ups endpoints (Phase 12)
	if cfg.DigestCommands != nil && cfg.DigestQueries != nil {
		digestHandler := NewDigestHandler(cfg.DigestCommands, cfg.DigestQueries)
		mux.HandleFunc("GET /v1/digests", digestHandler.ListPublicDigests)
		mux.HandleFunc("GET /v1/digests/{slug}", digestHandler.GetPublicDigest)

		mux.HandleFunc("GET /v1/cms/digests", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(digestHandler.ListCMSDigests))
		mux.HandleFunc("POST /v1/cms/digests", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(digestHandler.CreateDigest))
		mux.HandleFunc("GET /v1/cms/digests/{id}", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(digestHandler.GetCMSDigest))
		mux.HandleFunc("POST /v1/cms/digests/{id}/auto-assemble", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(digestHandler.AutoAssemble))
		mux.HandleFunc("POST /v1/cms/digests/{id}/split-parts", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(digestHandler.SplitDigest))
		mux.HandleFunc("POST /v1/cms/digests/{id}/publish", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(digestHandler.PublishDigest))
		mux.HandleFunc("POST /v1/cms/digests/generate-monthly", RequireRoles(identity.RoleEditor, identity.RoleAdmin)(digestHandler.GenerateMonthlyDrafts))
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
