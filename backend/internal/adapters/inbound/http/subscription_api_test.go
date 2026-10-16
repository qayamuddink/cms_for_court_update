package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/livelaw/backend/internal/adapters/outbound/entitlement"
	"github.com/livelaw/backend/internal/adapters/outbound/hasher"
	"github.com/livelaw/backend/internal/adapters/outbound/idgen"
	"github.com/livelaw/backend/internal/adapters/outbound/payment"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	"github.com/livelaw/backend/internal/adapters/outbound/push"
	"github.com/livelaw/backend/internal/adapters/outbound/token"
	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/billing"
	"github.com/livelaw/backend/internal/domain/content"
	"github.com/livelaw/backend/internal/domain/identity"
	"github.com/livelaw/backend/internal/domain/livecase"
	"github.com/livelaw/backend/migrations"
)

type subTestEnv struct {
	router         http.Handler
	db             *sql.DB
	pushSender     *push.StubPushSender
	pushDispatcher *commands.PushDispatcher
	authUseCases   *commands.AuthUseCases
	subUseCases    *commands.SubscriptionUseCases
	articleRepo    ports.ArticleRepository
	caseRepo       ports.CourtCaseRepository
	updateRepo     ports.LiveUpdateRepository
	adminToken     string
	editorToken    string
	readerToken    string
	readerUserID   string
	readerEmail    string
	adminUserID    string
}

func setupSubTestEnv(t *testing.T) *subTestEnv {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://livelaw:livelaw_secret@localhost:5432/livelaw?sslmode=disable"
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Skipf("skipping test: database connection error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping test: database unreachable: %v", err)
	}

	// Apply migrations
	migrator := postgres.NewMigrator(db, migrations.FS)
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("failed applying migrations: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clock := ports.RealClock{}
	idGenerator := idgen.NewUUIDv7Generator()
	dbHealthChecker := postgres.NewHealthChecker(db)
	userRepo := postgres.NewSQLUserRepository(db)
	tokenRepo := postgres.NewSQLRefreshTokenRepository(db)
	pwHasher := hasher.NewBcryptHasher(4)
	tokenService := token.NewJWTService("phase-6-test-secret-key-32-bytes-long", "test", 15*time.Minute, 7*24*time.Hour, clock)

	// Seed canonical users
	seeder := postgres.NewSeeder(userRepo, pwHasher, idGenerator, clock, logger)
	if err := seeder.SeedUsers(ctx); err != nil {
		t.Fatalf("failed seeding users: %v", err)
	}

	// Phase 6 Repositories
	caseRepo := postgres.NewSQLCourtCaseRepository(db)
	updateRepo := postgres.NewSQLLiveUpdateRepository(db)
	articleRepo := postgres.NewSQLArticleRepository(db)
	subRepo := postgres.NewSQLSubscriptionRepository(db)
	caseFollowRepo := postgres.NewSQLCaseFollowRepository(db)
	deviceTokenRepo := postgres.NewSQLDeviceTokenRepository(db)

	paymentGateway := payment.NewStubPaymentGateway("test_webhook_secret")
	pushSender := push.NewStubPushSender(logger)
	pushDispatcher := commands.NewPushDispatcher(caseFollowRepo, deviceTokenRepo, caseRepo, pushSender, logger)
	entitlementChecker := entitlement.NewSQLSubscriptionChecker(db, userRepo)

	sectionRepo := postgres.NewSQLSectionRepository(db)
	authUseCases := commands.NewAuthUseCases(userRepo, tokenRepo, pwHasher, tokenService, idGenerator, clock, subRepo)
	userQueries := queries.NewUserQueries(userRepo)
	articleQueries := queries.NewArticleQueries(articleRepo, sectionRepo, entitlementChecker, nil)
	subUseCases := commands.NewSubscriptionUseCases(subRepo, userRepo, paymentGateway, idGenerator, clock)
	subQueries := queries.NewSubscriptionQueries(subRepo)
	followUseCases := commands.NewFollowUseCases(caseFollowRepo, deviceTokenRepo, caseRepo, idGenerator, clock)
	followQueries := queries.NewFollowQueries(caseFollowRepo)

	router := NewRouter(RouterConfig{
		Logger:               logger,
		DBHealthChecker:      dbHealthChecker,
		Clock:                clock,
		TokenService:         tokenService,
		AuthUseCases:         authUseCases,
		UserQueries:          userQueries,
		ArticleQueries:       articleQueries,
		SubscriptionUseCases: subUseCases,
		SubscriptionQueries:  subQueries,
		FollowUseCases:       followUseCases,
		FollowQueries:        followQueries,
	})

	// Login admin
	adminRes, err := authUseCases.Login(ctx, commands.LoginCommand{
		Email:    "admin@livelaw.in",
		Password: "AdminPass123!",
	})
	if err != nil {
		t.Fatalf("failed to login admin: %v", err)
	}

	// Login editor
	editorRes, err := authUseCases.Login(ctx, commands.LoginCommand{
		Email:    "editor@livelaw.in",
		Password: "EditorPass123!",
	})
	if err != nil {
		t.Fatalf("failed to login editor: %v", err)
	}

	// Register unique reader
	readerEmail := fmt.Sprintf("reader_%s@test.com", idGenerator.NewID())
	readerRes, err := authUseCases.Register(ctx, commands.RegisterCommand{
		Email:       readerEmail,
		Password:    "ReaderPassword123!",
		FullName:    "Test Reader",
		DisplayRole: identity.DisplayPublic,
	})
	if err != nil {
		t.Fatalf("failed registering reader: %v", err)
	}

	return &subTestEnv{
		router:         router,
		db:             db,
		pushSender:     pushSender,
		pushDispatcher: pushDispatcher,
		authUseCases:   authUseCases,
		subUseCases:    subUseCases,
		articleRepo:    articleRepo,
		caseRepo:       caseRepo,
		updateRepo:     updateRepo,
		adminToken:     adminRes.AccessToken,
		editorToken:    editorRes.AccessToken,
		readerToken:    readerRes.AccessToken,
		readerUserID:   readerRes.User.ID,
		readerEmail:    readerEmail,
		adminUserID:    adminRes.User.ID,
	}
}

// TestPhase6_PaywallAndSubscriptions tests the complete paywall flow:
// 1. Free user is blocked from premium body
// 2. Webhook triggers subscription grant idempotently
// 3. User is granted full access after subscription
func TestPhase6_PaywallAndSubscriptions(t *testing.T) {
	env := setupSubTestEnv(t)
	ctx := context.Background()

	// 1. Create a published premium article directly in DB
	now := time.Now().UTC()
	artID := idgen.NewUUIDv7Generator().NewID()
	slug := fmt.Sprintf("premium-court-verdict-%d", now.UnixNano())
	premiumArt := &content.Article{
		ID:          artID,
		Slug:        slug,
		Title:       "Landmark Supreme Court Privacy Ruling",
		Summary:     "Supreme Court 9-judge bench issues privacy guidelines.",
		BodyBlocks:  []byte(`[{"type": "paragraph", "content": "Full confidential premium judgment analysis."}]`),
		ArticleType: content.ArticleTypeNews,
		AccessLevel: content.AccessLevelPremium,
		Status:      content.StatusPublished,
		AuthorID:    env.adminUserID,
		Category:    "Supreme Court",
		Tags:        []string{"privacy", "constitution"},
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := env.articleRepo.Create(ctx, premiumArt); err != nil {
		t.Fatalf("failed inserting premium article: %v", err)
	}

	// 2. Fetch as Unauthenticated / Free Reader -> Must be LOCKED
	req := httptest.NewRequest(http.MethodGet, "/v1/articles/"+slug, nil)
	req.Header.Set("Authorization", "Bearer "+env.readerToken)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// Check Cache-Control: private, no-store
	cacheControl := rec.Header().Get("Cache-Control")
	if cacheControl != "private, no-store" {
		t.Errorf("expected Cache-Control 'private, no-store', got '%s'", cacheControl)
	}

	var blockedResp struct {
		IsLocked   bool   `json:"is_locked"`
		BodyBlocks any    `json:"body_blocks"`
		Summary    string `json:"summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &blockedResp); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}

	if !blockedResp.IsLocked {
		t.Errorf("expected is_locked == true for free user, got false")
	}
	if blockedResp.BodyBlocks != nil {
		t.Errorf("expected body_blocks to be withheld (nil) for free user, got %v", blockedResp.BodyBlocks)
	}
	if blockedResp.Summary == "" {
		t.Errorf("expected summary teaser to be visible")
	}

	// 3. Process Payment Webhook to activate reader subscription
	providerSubID := fmt.Sprintf("sub_rzp_%d", now.UnixNano())
	webhookPayload := fmt.Sprintf(`{
		"event_id": "evt_test_1",
		"event_type": "subscription.created",
		"provider_subscription_id": "%s",
		"user_id": "%s",
		"tier": "pro",
		"price_amount_minor": 49900,
		"currency": "INR",
		"started_at": "%s",
		"current_period_end": "%s"
	}`, providerSubID, env.readerUserID, now.Format(time.RFC3339), now.AddDate(0, 1, 0).Format(time.RFC3339))

	whReq := httptest.NewRequest(http.MethodPost, "/v1/webhooks/payments", bytes.NewBufferString(webhookPayload))
	whReq.Header.Set("Content-Type", "application/json")
	whReq.Header.Set("X-Signature", "test_sig")
	whRec := httptest.NewRecorder()
	env.router.ServeHTTP(whRec, whReq)

	if whRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from webhook, got %d: %s", whRec.Code, whRec.Body.String())
	}

	// Idempotency: replay the same webhook payload
	whReq2 := httptest.NewRequest(http.MethodPost, "/v1/webhooks/payments", bytes.NewBufferString(webhookPayload))
	whReq2.Header.Set("Content-Type", "application/json")
	whReq2.Header.Set("X-Signature", "test_sig")
	whRec2 := httptest.NewRecorder()
	env.router.ServeHTTP(whRec2, whReq2)

	if whRec2.Code != http.StatusOK {
		t.Fatalf("expected 200 from idempotent replay webhook, got %d: %s", whRec2.Code, whRec2.Body.String())
	}

	// 4. Verify reader checks /v1/me/subscription
	meSubReq := httptest.NewRequest(http.MethodGet, "/v1/me/subscription", nil)
	meSubReq.Header.Set("Authorization", "Bearer "+env.readerToken)
	meSubRec := httptest.NewRecorder()
	env.router.ServeHTTP(meSubRec, meSubReq)

	if meSubRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /v1/me/subscription, got %d: %s", meSubRec.Code, meSubRec.Body.String())
	}

	var meSubResp struct {
		Tier       string `json:"tier"`
		IsEntitled bool   `json:"is_entitled"`
	}
	if err := json.Unmarshal(meSubRec.Body.Bytes(), &meSubResp); err != nil {
		t.Fatalf("failed unmarshaling me/subscription: %v", err)
	}
	if meSubResp.Tier != "pro" || !meSubResp.IsEntitled {
		t.Errorf("expected pro subscription, got %+v", meSubResp)
	}

	// 5. Re-login reader to generate new JWT with embedded entitlement claim
	loginRes, err := env.authUseCases.Login(ctx, commands.LoginCommand{
		Email:    env.readerEmail,
		Password: "ReaderPassword123!",
	})
	if err != nil {
		t.Fatalf("failed logging in after subscription: %v", err)
	}
	newReaderToken := loginRes.AccessToken

	// 6. Fetch premium article with new entitled token -> MUST BE UNLOCKED!
	reqEntitled := httptest.NewRequest(http.MethodGet, "/v1/articles/"+slug, nil)
	reqEntitled.Header.Set("Authorization", "Bearer "+newReaderToken)
	recEntitled := httptest.NewRecorder()
	env.router.ServeHTTP(recEntitled, reqEntitled)

	if recEntitled.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for entitled reader, got %d: %s", recEntitled.Code, recEntitled.Body.String())
	}

	var unlockedResp struct {
		IsLocked   bool `json:"is_locked"`
		BodyBlocks any  `json:"body_blocks"`
	}
	if err := json.Unmarshal(recEntitled.Body.Bytes(), &unlockedResp); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}

	if unlockedResp.IsLocked {
		t.Errorf("expected is_locked == false for subscribed user, got true")
	}
	if unlockedResp.BodyBlocks == nil {
		t.Errorf("expected full body_blocks to be delivered to subscribed user")
	}
}

// TestPhase6_ZeroDBEntitlement verifies that when a valid entitlement claim is inside JWT claims,
// HasArticleAccess grants access without requiring any database queries.
func TestPhase6_ZeroDBEntitlement(t *testing.T) {
	// Nil DB and Nil UserRepo: if a DB call were attempted, it would either query or error
	checker := entitlement.NewSQLSubscriptionChecker(nil, nil)

	futureExpiry := time.Now().Add(30 * 24 * time.Hour).Unix()
	claims := &ports.AuthClaims{
		UserID:     "user-123",
		Email:      "reader@test.com",
		SystemRole: "reader",
		Entitlement: ports.EntitlementClaim{
			Tier:      "pro",
			ExpiresAt: futureExpiry,
		},
	}

	ctx := ports.ContextWithAuthClaims(context.Background(), claims)
	userID := "user-123"
	premiumArticle := &content.Article{
		AccessLevel: content.AccessLevelPremium,
	}

	allowed, err := checker.HasArticleAccess(ctx, &userID, premiumArticle)
	if err != nil {
		t.Fatalf("expected nil error on fast-path check, got: %v", err)
	}
	if !allowed {
		t.Errorf("expected fast-path to allow access with zero database calls")
	}
}

// TestPhase6_CaseFollowsAndPushDispatch verifies that following a case and receiving
// a verdict update triggers push notification dispatch to followers.
func TestPhase6_CaseFollowsAndPushDispatch(t *testing.T) {
	env := setupSubTestEnv(t)
	ctx := context.Background()

	// 1. Create a court case
	now := time.Now().UTC()
	courtCase := &livecase.CourtCase{
		ID:                idgen.NewUUIDv7Generator().NewID(),
		CourtName:         "Supreme Court of India",
		CaseNumber:        fmt.Sprintf("WP-%d", now.UnixNano()),
		CaseType:          "Writ Petition",
		Title:             "Citizens for Right to Privacy v. Union of India",
		BenchJudges:       []string{"Justice A", "Justice B"},
		PresidingJudge:    "Chief Justice",
		PetitionerParties: []string{"Citizens Trust"},
		RespondentParties: []string{"Union of India"},
		Status:            livecase.CaseStatusHearing,
		LastSequenceNo:    0,
		IsLive:            true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := env.caseRepo.Create(ctx, courtCase); err != nil {
		t.Fatalf("failed inserting case: %v", err)
	}

	// 2. Register Device Push Token for reader
	devPayload := `{"platform": "android", "push_token": "fcm_test_device_token_abc"}`
	devReq := httptest.NewRequest(http.MethodPost, "/v1/devices", strings.NewReader(devPayload))
	devReq.Header.Set("Authorization", "Bearer "+env.readerToken)
	devReq.Header.Set("Content-Type", "application/json")
	devRec := httptest.NewRecorder()
	env.router.ServeHTTP(devRec, devReq)

	if devRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created from /v1/devices, got %d: %s", devRec.Code, devRec.Body.String())
	}

	// 3. Follow the court case for verdict & order alerts
	followPayload := `{"notify_on": ["verdict", "order"]}`
	followReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cases/%s/follow", courtCase.ID), strings.NewReader(followPayload))
	followReq.Header.Set("Authorization", "Bearer "+env.readerToken)
	followReq.Header.Set("Content-Type", "application/json")
	followRec := httptest.NewRecorder()
	env.router.ServeHTTP(followRec, followReq)

	if followRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from follow, got %d: %s", followRec.Code, followRec.Body.String())
	}

	// Check status endpoint
	statusReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/cases/%s/follow", courtCase.ID), nil)
	statusReq.Header.Set("Authorization", "Bearer "+env.readerToken)
	statusRec := httptest.NewRecorder()
	env.router.ServeHTTP(statusRec, statusReq)

	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from follow status, got %d", statusRec.Code)
	}
	var followStatus queries.CaseFollowStatusResponse
	_ = json.Unmarshal(statusRec.Body.Bytes(), &followStatus)
	if !followStatus.IsFollowing || followStatus.FollowerCount < 1 {
		t.Errorf("expected is_following true and count >= 1, got %+v", followStatus)
	}

	// 4. Simulate a verdict outbox event
	env.pushSender.Reset()
	verdictEvent := ports.OutboxEvent{
		ID:            "evt_verdict_1",
		AggregateType: "court_case",
		AggregateID:   courtCase.ID,
		EventType:     "live_update.published",
		Payload: []byte(fmt.Sprintf(`{
			"case_id": "%s",
			"update_id": "upd_1",
			"update_type": "verdict",
			"content": "FINAL VERDICT: The Court unanimously strikes down Section 66A.",
			"status": "published"
		}`, courtCase.ID)),
	}

	// Dispatch outbox event to push worker
	if err := env.pushDispatcher.Publish(ctx, verdictEvent); err != nil {
		t.Fatalf("push dispatcher failed: %v", err)
	}

	// Verify push notification was sent to follower
	sentMsgs := env.pushSender.GetSentMessages()
	if len(sentMsgs) != 1 {
		t.Fatalf("expected 1 push message sent to followers, got %d", len(sentMsgs))
	}

	msg := sentMsgs[0]
	if len(msg.DeviceTokens) != 1 || msg.DeviceTokens[0] != "fcm_test_device_token_abc" {
		t.Errorf("expected recipient fcm_test_device_token_abc, got %v", msg.DeviceTokens)
	}
	if !strings.Contains(msg.Title, "VERDICT") {
		t.Errorf("expected title to contain VERDICT, got '%s'", msg.Title)
	}
	if !strings.Contains(msg.Body, "strikes down Section 66A") {
		t.Errorf("expected body to contain verdict text, got '%s'", msg.Body)
	}

	// 5. Unfollow and ensure no subsequent notification sent
	unfollowReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/cases/%s/follow", courtCase.ID), nil)
	unfollowReq.Header.Set("Authorization", "Bearer "+env.readerToken)
	unfollowRec := httptest.NewRecorder()
	env.router.ServeHTTP(unfollowRec, unfollowReq)

	if unfollowRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from unfollow, got %d", unfollowRec.Code)
	}

	env.pushSender.Reset()
	_ = env.pushDispatcher.Publish(ctx, verdictEvent)
	if len(env.pushSender.GetSentMessages()) != 0 {
		t.Errorf("expected 0 push messages after unfollow, got %d", len(env.pushSender.GetSentMessages()))
	}
}

// TestPhase6_CMSAdminSubscriptions verifies admin endpoints to list, grant, and cancel subscriptions.
func TestPhase6_CMSAdminSubscriptions(t *testing.T) {
	env := setupSubTestEnv(t)

	// 1. Grant subscription manually via Admin
	grantPayload := fmt.Sprintf(`{
		"user_id": "%s",
		"tier": "institutional",
		"duration_days": 60
	}`, env.readerUserID)

	grantReq := httptest.NewRequest(http.MethodPost, "/v1/cms/subscriptions/grant", strings.NewReader(grantPayload))
	grantReq.Header.Set("Authorization", "Bearer "+env.adminToken)
	grantReq.Header.Set("Content-Type", "application/json")
	grantRec := httptest.NewRecorder()
	env.router.ServeHTTP(grantRec, grantReq)

	if grantRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created from grant subscription, got %d: %s", grantRec.Code, grantRec.Body.String())
	}

	var sub billing.Subscription
	if err := json.Unmarshal(grantRec.Body.Bytes(), &sub); err != nil {
		t.Fatalf("failed decoding granted sub: %v", err)
	}
	if sub.Tier != billing.TierInstitutional {
		t.Errorf("expected tier institutional, got %s", sub.Tier)
	}

	// 2. List subscriptions via CMS
	listReq := httptest.NewRequest(http.MethodGet, "/v1/cms/subscriptions?limit=10", nil)
	listReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	listRec := httptest.NewRecorder()
	env.router.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from list subscriptions, got %d: %s", listRec.Code, listRec.Body.String())
	}

	// 3. Cancel subscription via Admin
	cancelReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/subscriptions/%s/cancel", sub.ID), nil)
	cancelReq.Header.Set("Authorization", "Bearer "+env.adminToken)
	cancelRec := httptest.NewRecorder()
	env.router.ServeHTTP(cancelRec, cancelReq)

	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from cancel subscription, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}
}
