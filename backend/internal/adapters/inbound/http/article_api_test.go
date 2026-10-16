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
	"github.com/livelaw/backend/internal/adapters/outbound/filestore"
	"github.com/livelaw/backend/internal/adapters/outbound/hasher"
	"github.com/livelaw/backend/internal/adapters/outbound/idgen"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	"github.com/livelaw/backend/internal/adapters/outbound/token"
	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/content"
	"github.com/livelaw/backend/migrations"
)

type articleTestEnv struct {
	router         http.Handler
	db             *sql.DB
	fileStore      *filestore.MinIOFileStore
	adminToken     string
	editorToken    string
	reporterToken  string
	readerToken    string
	reporterUserID string
}

func setupArticleTestEnv(t *testing.T) *articleTestEnv {
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
	tokenService := token.NewJWTService("phase-5-test-jwt-secret-key-32-bytes-long", "test", 15*time.Minute, 7*24*time.Hour, clock)

	// Seed canonical users
	seeder := postgres.NewSeeder(userRepo, pwHasher, idGenerator, clock, logger)
	if err := seeder.SeedUsers(ctx); err != nil {
		t.Fatalf("failed seeding users: %v", err)
	}

	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		s3Endpoint = "http://localhost:9000"
	}

	fileStore, err := filestore.NewMinIOFileStore(filestore.Config{
		Endpoint:        s3Endpoint,
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		BucketName:      "livelaw-files",
		UseSSL:          false,
	})
	if err != nil {
		t.Fatalf("failed initializing MinIO: %v", err)
	}
	_ = fileStore.EnsureBucket(ctx)

	articleRepo := postgres.NewSQLArticleRepository(db)
	caseRepo := postgres.NewSQLCourtCaseRepository(db)
	entitlementChecker := entitlement.NewSQLSubscriptionChecker(db, userRepo)

	authUseCases := commands.NewAuthUseCases(userRepo, tokenRepo, pwHasher, tokenService, idGenerator, clock)
	userQueries := queries.NewUserQueries(userRepo)
	articleUseCases := commands.NewArticleUseCases(articleRepo, idGenerator, clock, fileStore)
	articleQueries := queries.NewArticleQueries(articleRepo, entitlementChecker, fileStore)
	liveCaseQueries := queries.NewLiveCaseQueries(caseRepo, nil, nil)
	liveCaseUseCases := commands.NewLiveCaseUseCases(caseRepo, nil, nil, nil, nil, nil, userRepo, idGenerator, clock)

	router := NewRouter(RouterConfig{
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
	})

	adminLogin := loginHelper(t, router, "admin@livelaw.in", "AdminPass123!")
	editorLogin := loginHelper(t, router, "editor@livelaw.in", "EditorPass123!")
	reporterLogin := loginHelper(t, router, "reporter@livelaw.in", "ReporterPass123!")
	readerLogin := loginHelper(t, router, "reader@livelaw.in", "ReaderPass123!")

	return &articleTestEnv{
		router:         router,
		db:             db,
		fileStore:      fileStore,
		adminToken:     adminLogin.AccessToken,
		editorToken:    editorLogin.AccessToken,
		reporterToken:  reporterLogin.AccessToken,
		readerToken:    readerLogin.AccessToken,
		reporterUserID: reporterLogin.User.ID,
	}
}

func TestArticle_WorkflowTransitions(t *testing.T) {
	env := setupArticleTestEnv(t)

	// 1. Reporter creates a draft article
	draftPayload := map[string]any{
		"title":        "Supreme Court Ruling on Arbitral Awards",
		"summary":      "A five-judge constitutional bench holds that unstamped agreements are inadmissible.",
		"category":     "Arbitration Law",
		"article_type": "news",
		"access_level": "free",
		"tags":         []string{"arbitration", "supreme-court"},
	}
	body, _ := json.Marshal(draftPayload)
	req := httptest.NewRequest(http.MethodPost, "/v1/cms/articles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.reporterToken)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for article draft, got %d: %s", w.Code, w.Body.String())
	}

	var article content.Article
	if err := json.Unmarshal(w.Body.Bytes(), &article); err != nil {
		t.Fatalf("failed parsing created article: %v", err)
	}

	if article.Status != content.StatusDraft {
		t.Errorf("expected draft status, got %s", article.Status)
	}
	if article.Slug == "" {
		t.Errorf("expected generated slug, got empty")
	}

	// 2. Reporter tries to publish directly -> Forbidden (Editor/Admin only)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/publish", article.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.reporterToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden when reporter tries to publish, got %d", w.Code)
	}

	// 3. Reporter submits draft for review -> OK (status -> in_review)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/submit", article.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.reporterToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when submitting draft, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Editor tries to send back without comment -> Fails with 400
	sendBackEmpty, _ := json.Marshal(map[string]string{"comment": "  "})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/send-back", article.ID), bytes.NewReader(sendBackEmpty))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request when comment is empty, got %d", w.Code)
	}

	// 5. Editor sends back with comment -> OK (status -> draft)
	sendBackPayload, _ := json.Marshal(map[string]string{"comment": "Please cite the NN Global judgment properly."})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/send-back", article.ID), bytes.NewReader(sendBackPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for send-back, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Reporter resubmits -> OK
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/submit", article.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.reporterToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on re-submit, got %d", w.Code)
	}

	// 7. Editor approves and publishes -> OK (status -> published)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/publish", article.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on publish, got %d: %s", w.Code, w.Body.String())
	}

	var published content.Article
	_ = json.Unmarshal(w.Body.Bytes(), &published)
	if published.Status != content.StatusPublished {
		t.Errorf("expected status published, got %s", published.Status)
	}
	if published.PublishedAt == nil {
		t.Errorf("expected published_at timestamp to be set")
	}

	// 8. Editor archives article -> OK
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/archive", article.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on archive, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArticle_OptimisticLocking(t *testing.T) {
	env := setupArticleTestEnv(t)

	// Create initial draft (version 1)
	draftPayload, _ := json.Marshal(map[string]any{
		"title":    "Optimistic Locking Test Article",
		"summary":  "Testing concurrent edits and conflict handling.",
		"category": "Corporate Law",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/cms/articles", bytes.NewReader(draftPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	var article content.Article
	_ = json.Unmarshal(w.Body.Bytes(), &article)
	if article.Version != 1 {
		t.Fatalf("expected initial version 1, got %d", article.Version)
	}

	// Editor 1 updates with expected_version: 1 -> OK (becomes version 2)
	update1, _ := json.Marshal(map[string]any{
		"expected_version": 1,
		"title":            "Optimistic Locking - Editor 1 Edit",
		"summary":          "Testing concurrent edits.",
		"category":         "Corporate Law",
	})
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/v1/cms/articles/%s", article.ID), bytes.NewReader(update1))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for update 1, got %d: %s", w.Code, w.Body.String())
	}

	// Editor 2 attempts to save with stale expected_version: 1 -> 409 Conflict
	updateStale, _ := json.Marshal(map[string]any{
		"expected_version": 1,
		"title":            "Optimistic Locking - Editor 2 Stale Edit",
		"summary":          "Overwriting concurrently.",
		"category":         "Corporate Law",
	})
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/v1/cms/articles/%s", article.ID), bytes.NewReader(updateStale))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for stale update, got %d: %s", w.Code, w.Body.String())
	}

	// Editor 2 refreshes and updates with expected_version: 2 -> OK (becomes version 3)
	updateFresh, _ := json.Marshal(map[string]any{
		"expected_version": 2,
		"title":            "Optimistic Locking - Editor 2 Fresh Edit",
		"summary":          "Overwriting after refresh.",
		"category":         "Corporate Law",
	})
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/v1/cms/articles/%s", article.ID), bytes.NewReader(updateFresh))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for fresh update, got %d: %s", w.Code, w.Body.String())
	}

	var updated content.Article
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Version != 3 {
		t.Errorf("expected version 3, got %d", updated.Version)
	}
}

func TestArticle_UploadURLAndMinIOAttachment(t *testing.T) {
	env := setupArticleTestEnv(t)

	// 1. Request upload URL for PDF
	uploadReq, _ := json.Marshal(map[string]any{
		"filename":     "judgment_order_2026.pdf",
		"content_type": "application/pdf",
		"size_bytes":   2048576,
		"purpose":      "judgment_pdf",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/cms/uploads/presigned-url", bytes.NewReader(uploadReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.reporterToken)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for presigned upload URL, got %d: %s", w.Code, w.Body.String())
	}

	var res commands.UploadURLResult
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed decoding upload url response: %v", err)
	}
	if res.UploadURL == "" || res.StorageKey == "" {
		t.Errorf("expected non-empty upload_url and storage_key, got %+v", res)
	}

	// 2. Upload sample PDF bytes directly into MinIO using the storage adapter
	pdfContent := []byte("%PDF-1.4 official judgment order text content for testing")
	ctx := context.Background()
	err := env.fileStore.PutObject(ctx, res.StorageKey, bytes.NewReader(pdfContent), int64(len(pdfContent)), "application/pdf")
	if err != nil {
		t.Fatalf("failed putting test PDF directly into MinIO: %v", err)
	}

	// 3. Create draft article and attach PDF metadata
	draftPayload, _ := json.Marshal(map[string]any{
		"title":        "High Court Judgment on Bail Rights",
		"summary":      "Bail granted in money laundering case.",
		"category":     "Criminal Law",
		"article_type": "judgment_summary",
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/cms/articles", bytes.NewReader(draftPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	var article content.Article
	_ = json.Unmarshal(w.Body.Bytes(), &article)

	attachPayload, _ := json.Marshal(map[string]any{
		"storage_key": res.StorageKey,
		"size_bytes":  len(pdfContent),
		"checksum":    "sha256-dummy-hash",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/attachment", article.ID), bytes.NewReader(attachPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on attachment, got %d: %s", w.Code, w.Body.String())
	}

	var attached content.Article
	_ = json.Unmarshal(w.Body.Bytes(), &attached)
	if attached.PDFStorageKey == nil || *attached.PDFStorageKey != res.StorageKey {
		t.Errorf("expected attached storage key %s, got %v", res.StorageKey, attached.PDFStorageKey)
	}

	// 4. Test invalid file type rejection
	invalidReq, _ := json.Marshal(map[string]any{
		"filename":     "malicious.exe",
		"content_type": "application/x-dosexec",
		"size_bytes":   100,
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/cms/uploads/presigned-url", bytes.NewReader(invalidReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.reporterToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for executable file, got %d", w.Code)
	}
}

func TestArticle_KeysetPaginationFeed(t *testing.T) {
	env := setupArticleTestEnv(t)

	// Publish 5 distinct articles
	for i := 1; i <= 5; i++ {
		payload, _ := json.Marshal(map[string]any{
			"title":        fmt.Sprintf("Keyset Feed Test Article Number %d", i),
			"summary":      fmt.Sprintf("Summary content for article %d", i),
			"category":     "Constitutional Law",
			"article_type": "news",
		})
		req := httptest.NewRequest(http.MethodPost, "/v1/cms/articles", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.editorToken)
		w := httptest.NewRecorder()
		env.router.ServeHTTP(w, req)

		var art content.Article
		_ = json.Unmarshal(w.Body.Bytes(), &art)

		// Publish immediately
		pubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/publish", art.ID), nil)
		pubReq.Header.Set("Authorization", "Bearer "+env.editorToken)
		pw := httptest.NewRecorder()
		env.router.ServeHTTP(pw, pubReq)
		time.Sleep(10 * time.Millisecond) // Ensure unique timestamps
	}

	// Page 1: limit 2
	req := httptest.NewRequest(http.MethodGet, "/v1/articles?category=Constitutional+Law&limit=2", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for feed page 1, got %d", w.Code)
	}

	var page1 queries.FeedResponse
	_ = json.Unmarshal(w.Body.Bytes(), &page1)
	if len(page1.Articles) != 2 {
		t.Errorf("expected 2 articles in page 1, got %d", len(page1.Articles))
	}
	if page1.NextCursor == nil || *page1.NextCursor == "" {
		t.Fatalf("expected next_cursor in page 1")
	}

	// Page 2: with cursor
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/articles?category=Constitutional+Law&limit=2&cursor=%s", *page1.NextCursor), nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	var page2 queries.FeedResponse
	_ = json.Unmarshal(w.Body.Bytes(), &page2)
	if len(page2.Articles) != 2 {
		t.Errorf("expected 2 articles in page 2, got %d", len(page2.Articles))
	}
	if page2.NextCursor == nil {
		t.Fatalf("expected next_cursor in page 2")
	}

	// Ensure page 1 and page 2 articles are disjoint
	page1IDs := map[string]bool{page1.Articles[0].ID: true, page1.Articles[1].ID: true}
	for _, a := range page2.Articles {
		if page1IDs[a.ID] {
			t.Errorf("duplicate article %s found across pages", a.ID)
		}
	}
}

func TestJudgment_CreateAndEndToEndPublicAccess(t *testing.T) {
	env := setupArticleTestEnv(t)

	// 1. Create a court case first
	casePayload, _ := json.Marshal(map[string]any{
		"court_name":         "Supreme Court of India",
		"case_number":        fmt.Sprintf("CIVIL-APPEAL-%d", time.Now().UnixNano()),
		"case_type":          "Civil Appeal",
		"title":              "Mineral Rights Taxation Constitutional Bench",
		"presiding_judge":    "Chief Justice DY Chandrachud",
		"bench_judges":       []string{"CJI", "Justice Gavai", "Justice Surya Kant"},
		"petitioner_parties": []string{"State of Mineral Resources"},
		"respondent_parties": []string{"Mining Companies Union"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/cms/cases", bytes.NewReader(casePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for test case, got %d: %s", w.Code, w.Body.String())
	}

	var courtCase map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &courtCase)
	caseID := courtCase["id"].(string)

	// 2. Put sample PDF into MinIO
	pdfKey := "judgments/2026/10/sc-appeal-8888.pdf"
	pdfBytes := []byte("%PDF-1.4 Official Judgment Bench Order - Mineral Rights Validated.")
	_ = env.fileStore.PutObject(context.Background(), pdfKey, bytes.NewReader(pdfBytes), int64(len(pdfBytes)), "application/pdf")

	// 3. Create Judgment Summary linked to case
	size := int64(len(pdfBytes))
	checksum := "sha256-test-checksum"
	citation := "2026 INSC 888"
	judgmentPayload, _ := json.Marshal(map[string]any{
		"court_case_id":   caseID,
		"title":           "SC Holds States Have Power to Tax Mineral Rights",
		"summary":         "In a 8:1 majority judgment, the Supreme Court held that royalty is not tax.",
		"access_level":    "free",
		"court_name":      "Supreme Court of India",
		"citation":        citation,
		"pdf_storage_key": pdfKey,
		"pdf_size_bytes":  size,
		"pdf_checksum":    checksum,
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/cms/judgments", bytes.NewReader(judgmentPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for judgment summary, got %d: %s", w.Code, w.Body.String())
	}

	var judgment content.Article
	_ = json.Unmarshal(w.Body.Bytes(), &judgment)

	// Publish the judgment summary
	pubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/publish", judgment.ID), nil)
	pubReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	pw := httptest.NewRecorder()
	env.router.ServeHTTP(pw, pubReq)

	// 4. Public reader accesses judgment by court case: GET /v1/cases/{id}/judgment
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/cases/%s/judgment", caseID), nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /v1/cases/{id}/judgment, got %d: %s", w.Code, w.Body.String())
	}

	var caseJudgment queries.JudgmentDetailResponse
	_ = json.Unmarshal(w.Body.Bytes(), &caseJudgment)
	if caseJudgment.PDFDownloadURL == nil || !strings.Contains(*caseJudgment.PDFDownloadURL, "http") {
		t.Errorf("expected signed PDF download URL, got %v", caseJudgment.PDFDownloadURL)
	}

	// 5. Public reader accesses judgment by ID: GET /v1/judgments/{id}
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/judgments/%s", judgment.ID), nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /v1/judgments/{id}, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Public reader accesses article by slug: GET /v1/articles/{slug}
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/articles/%s", judgment.Slug), nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /v1/articles/{slug}, got %d: %s", w.Code, w.Body.String())
	}

	var slugDetail queries.ArticleDetailResponse
	_ = json.Unmarshal(w.Body.Bytes(), &slugDetail)
	if slugDetail.IsLocked {
		t.Errorf("expected free article to not be locked")
	}
}
