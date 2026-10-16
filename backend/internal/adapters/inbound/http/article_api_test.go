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

	sectionRepo := postgres.NewSQLSectionRepository(db)
	articleRepo := postgres.NewSQLArticleRepository(db)
	caseRepo := postgres.NewSQLCourtCaseRepository(db)
	entitlementChecker := entitlement.NewSQLSubscriptionChecker(db, userRepo)

	authUseCases := commands.NewAuthUseCases(userRepo, tokenRepo, pwHasher, tokenService, idGenerator, clock)
	userQueries := queries.NewUserQueries(userRepo)
	sectionUseCases := commands.NewSectionUseCases(sectionRepo, idGenerator, clock)
	articleUseCases := commands.NewArticleUseCases(articleRepo, idGenerator, clock, fileStore)
	articleQueries := queries.NewArticleQueries(articleRepo, sectionRepo, entitlementChecker, fileStore)
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
		SectionUseCases:  sectionUseCases,
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

func TestPhase9_Sections_CRUD_And_Filtering(t *testing.T) {
	env := setupArticleTestEnv(t)

	// 1. Public list sections
	req := httptest.NewRequest(http.MethodGet, "/v1/sections", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /v1/sections, got %d", w.Code)
	}

	var secList struct {
		Sections []content.Section `json:"sections"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &secList)
	if len(secList.Sections) < 10 {
		t.Fatalf("expected at least 10 seeded sections, got %d", len(secList.Sections))
	}

	// 2. CMS create new section
	createSecPayload := []byte(`{
		"name": "Commercial Courts",
		"slug": "commercial-courts",
		"description": "Commercial dispute resolution benches",
		"display_order": 110,
		"is_active": true
	}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/cms/sections", bytes.NewReader(createSecPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for section, got %d: %s", w.Code, w.Body.String())
	}

	var createdSec content.Section
	_ = json.Unmarshal(w.Body.Bytes(), &createdSec)
	if createdSec.Slug != "commercial-courts" {
		t.Errorf("expected slug 'commercial-courts', got %s", createdSec.Slug)
	}

	// 3. Update section
	updateSecPayload := []byte(`{
		"name": "Commercial & Admiralty Courts",
		"slug": "commercial-admiralty-courts",
		"description": "Updated description",
		"display_order": 115,
		"is_active": true
	}`)
	req = httptest.NewRequest(http.MethodPut, "/v1/cms/sections/"+createdSec.ID, bytes.NewReader(updateSecPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating section, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Delete section (Admin only)
	req = httptest.NewRequest(http.MethodDelete, "/v1/cms/sections/"+createdSec.ID, nil)
	req.Header.Set("Authorization", "Bearer "+env.adminToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deleting section, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPhase9_LongArticle_Revisions_Redirects_And_Scheduling(t *testing.T) {
	env := setupArticleTestEnv(t)

	// 1. Create a long illustrated article with headings
	bodyBlocks := []map[string]any{
		{
			"type": "heading",
			"attrs": map[string]any{"level": 2},
			"content": []any{map[string]any{"type": "text", "text": "1. Factual Matrix of the Case"}},
		},
		{
			"type": "paragraph",
			"content": []any{
				map[string]any{"type": "text", "text": strings.Repeat("detailed legal arguments constitutional questions bench observations statutory provisions ", 20)},
			},
		},
		{
			"type": "heading",
			"attrs": map[string]any{"level": 2},
			"content": []any{map[string]any{"type": "text", "text": "2. High Court Conflicting Interpretations"}},
		},
		{
			"type": "paragraph",
			"content": []any{
				map[string]any{"type": "text", "text": strings.Repeat("doctrine of basic structure judicial review precedents overruled ", 20)},
			},
		},
	}
	bodyJSON, _ := json.Marshal(bodyBlocks)

	articlePayload := map[string]any{
		"title":                  "Landmark Constitutional Bench Judgment on Article 19",
		"summary":                "Comprehensive analysis of speech freedoms and reasonable restrictions.",
		"body_blocks":            json.RawMessage(bodyJSON),
		"article_type":           "column",
		"access_level":           "free",
		"category":               "Constitutional Law",
		"primary_section_id":     "0192e240-0000-7000-8000-000000000001", // Supreme Court
		"featured_image_key":     "images/2026/01/sc-bench.jpg",
		"featured_image_caption": "Supreme Court of India Courtroom 1",
		"meta_title":             "Article 19 Landmark Ruling Analysis",
		"meta_description":       "Full in-depth analysis of the 5-judge bench decision.",
		"tags":                   []string{"article-19", "free-speech", "constitution"},
	}
	payloadBytes, _ := json.Marshal(articlePayload)

	req := httptest.NewRequest(http.MethodPost, "/v1/cms/articles", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for long article, got %d: %s", w.Code, w.Body.String())
	}

	var created content.Article
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.WordCount == 0 {
		t.Errorf("expected word count to be calculated, got 0")
	}
	if created.ReadingTimeMinutes < 1 {
		t.Errorf("expected reading time >= 1, got %d", created.ReadingTimeMinutes)
	}

	// 2. Test revision snapshot on update
	updatePayload := map[string]any{
		"expected_version": created.Version,
		"title":            "Landmark Supreme Court Ruling on Article 19 Free Speech",
		"summary":          "Updated summary with refined jurisprudential critique.",
		"body_blocks":      json.RawMessage(bodyJSON),
		"article_type":     "column",
		"access_level":     "free",
		"category":         "Constitutional Law",
	}
	upBytes, _ := json.Marshal(updatePayload)
	req = httptest.NewRequest(http.MethodPatch, "/v1/cms/articles/"+created.ID, bytes.NewReader(upBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating article, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Check revisions list
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/cms/articles/%s/revisions", created.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on revisions list, got %d: %s", w.Code, w.Body.String())
	}

	var revList struct {
		Revisions []content.ArticleRevision `json:"revisions"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &revList)
	if len(revList.Revisions) == 0 {
		t.Fatalf("expected at least 1 revision snapshot")
	}
	if revList.Revisions[0].Version != 1 {
		t.Errorf("expected revision version 1, got %d", revList.Revisions[0].Version)
	}

	// 4. Test revision restore
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/revisions/1/restore", created.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK restoring revision, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Publish article
	pubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/publish", created.ID), nil)
	pubReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	pw := httptest.NewRecorder()
	env.router.ServeHTTP(pw, pubReq)
	if pw.Code != http.StatusOK {
		t.Fatalf("expected 200 OK publishing article, got %d: %s", pw.Code, pw.Body.String())
	}

	// 6. Test 301 Redirect from old slug
	// Old slug was "landmark-constitutional-bench-judgment-on-article-19"
	// Current slug is "landmark-supreme-court-ruling-on-article-19-free-speech"
	oldSlug := "landmark-constitutional-bench-judgment-on-article-19"
	req = httptest.NewRequest(http.MethodGet, "/v1/articles/"+oldSlug, nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 Moved Permanently for old slug redirect, got %d: %s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "/v1/articles/") {
		t.Errorf("expected Location header with target slug, got: %s", location)
	}

	// 7. Verify public article detail response contains outline & reading time
	var pubArt content.Article
	_ = json.Unmarshal(pw.Body.Bytes(), &pubArt)

	req = httptest.NewRequest(http.MethodGet, "/v1/articles/"+pubArt.Slug, nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on active article slug, got %d", w.Code)
	}

	var detail queries.ArticleDetailResponse
	_ = json.Unmarshal(w.Body.Bytes(), &detail)
	if len(detail.Outline) != 2 {
		t.Errorf("expected 2 outline items, got %d", len(detail.Outline))
	}
	if detail.Outline[0].Text != "1. Factual Matrix of the Case" {
		t.Errorf("unexpected outline heading: %s", detail.Outline[0].Text)
	}

	// 8. Soft Delete and Restore
	req = httptest.NewRequest(http.MethodDelete, "/v1/cms/articles/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK soft deleting article, got %d", w.Code)
	}

	// Public access should now be 404
	req = httptest.NewRequest(http.MethodGet, "/v1/articles/"+pubArt.Slug, nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for soft-deleted article, got %d", w.Code)
	}

	// Restore
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/restore", created.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK restoring article, got %d", w.Code)
	}

	// Public access should now be 200 OK again
	req = httptest.NewRequest(http.MethodGet, "/v1/articles/"+pubArt.Slug, nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for restored article, got %d", w.Code)
	}
}

func TestPhase9_ScheduledPublishWorker_Execution(t *testing.T) {
	env := setupArticleTestEnv(t)

	// 1. Create a draft article
	articlePayload := map[string]any{
		"title":        "Scheduled Future Decision on Telecommunications Bill",
		"summary":      "A high-stakes regulatory challenge scheduled for tomorrow.",
		"article_type": "news",
		"access_level": "free",
		"category":     "Telecom",
	}
	payloadBytes, _ := json.Marshal(articlePayload)
	req := httptest.NewRequest(http.MethodPost, "/v1/cms/articles", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}

	var created content.Article
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	// 2. Schedule for 1 hour ago (past time simulates arrival of scheduled time)
	pastScheduledTime := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	// Direct repository update or worker check
	parsedTime, _ := time.Parse(time.RFC3339, pastScheduledTime)
	created.Status = content.StatusScheduled
	created.ScheduledAt = &parsedTime
	_ = env.db.QueryRow(`
		UPDATE articles SET status = 'scheduled', scheduled_at = $1 WHERE id = $2 RETURNING id;
	`, parsedTime, created.ID)

	// 3. Run worker processing
	worker := commands.NewScheduledPublisherWorker(postgres.NewSQLArticleRepository(env.db), ports.RealClock{}, 10*time.Second)
	publishedCount, err := worker.ProcessDueScheduledArticles(context.Background())
	if err != nil {
		t.Fatalf("failed processing due scheduled articles: %v", err)
	}
	if publishedCount < 1 {
		t.Errorf("expected at least 1 article published by worker, got %d", publishedCount)
	}

	// 4. Verify public slug endpoint now returns published article
	req = httptest.NewRequest(http.MethodGet, "/v1/articles/"+created.Slug, nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after scheduled publishing, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPhase9_SectionFeed_KeysetPagination(t *testing.T) {
	env := setupArticleTestEnv(t)
	taxSectionID := "0192e240-0000-7000-8000-000000000005" // Tax section from migration 11

	// Create 3 published articles in Tax section
	for i := 1; i <= 3; i++ {
		articlePayload := map[string]any{
			"title":              fmt.Sprintf("Taxation Tribunal Ruling Part %d", i),
			"summary":            fmt.Sprintf("Summary of tax tribunal ruling %d", i),
			"article_type":       "news",
			"access_level":       "free",
			"category":           "Taxation Law",
			"primary_section_id": taxSectionID,
		}
		pBytes, _ := json.Marshal(articlePayload)
		req := httptest.NewRequest(http.MethodPost, "/v1/cms/articles", bytes.NewReader(pBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.editorToken)
		w := httptest.NewRecorder()
		env.router.ServeHTTP(w, req)

		var art content.Article
		_ = json.Unmarshal(w.Body.Bytes(), &art)

		// Publish
		pubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/publish", art.ID), nil)
		pubReq.Header.Set("Authorization", "Bearer "+env.editorToken)
		pw := httptest.NewRecorder()
		env.router.ServeHTTP(pw, pubReq)
	}

	// Query feed filtered by section_slug=tax with limit=2
	req := httptest.NewRequest(http.MethodGet, "/v1/articles?section_slug=tax&limit=2", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on section feed, got %d", w.Code)
	}

	var page1 queries.FeedResponse
	_ = json.Unmarshal(w.Body.Bytes(), &page1)
	if len(page1.Articles) != 2 {
		t.Errorf("expected 2 articles on page 1, got %d", len(page1.Articles))
	}
	if page1.NextCursor == nil {
		t.Fatalf("expected next_cursor for keyset pagination")
	}

	// Query page 2 with cursor
	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/articles?section_slug=tax&limit=2&cursor=%s", *page1.NextCursor), nil)
	w2 := httptest.NewRecorder()
	env.router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on page 2, got %d", w2.Code)
	}

	var page2 queries.FeedResponse
	_ = json.Unmarshal(w2.Body.Bytes(), &page2)
	if len(page2.Articles) < 1 {
		t.Errorf("expected at least 1 article on page 2, got %d", len(page2.Articles))
	}
}

func TestPhase9_TopStories_Ordering(t *testing.T) {
	env := setupArticleTestEnv(t)

	// Create and publish 2 articles
	var ids []string
	for i := 1; i <= 2; i++ {
		articlePayload := map[string]any{
			"title":        fmt.Sprintf("Top Story Candidate %d", i),
			"summary":      fmt.Sprintf("Summary of candidate %d", i),
			"article_type": "top_story",
			"access_level": "free",
			"category":     "Breaking",
		}
		pBytes, _ := json.Marshal(articlePayload)
		req := httptest.NewRequest(http.MethodPost, "/v1/cms/articles", bytes.NewReader(pBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.editorToken)
		w := httptest.NewRecorder()
		env.router.ServeHTTP(w, req)

		var art content.Article
		_ = json.Unmarshal(w.Body.Bytes(), &art)
		ids = append(ids, art.ID)

		pubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/articles/%s/publish", art.ID), nil)
		pubReq.Header.Set("Authorization", "Bearer "+env.editorToken)
		pw := httptest.NewRecorder()
		env.router.ServeHTTP(pw, pubReq)
	}

	// Pin and order top stories: item 2 order 1, item 1 order 2
	orderPayload := map[string]any{
		"items": []map[string]any{
			{"article_id": ids[1], "top_story_order": 1, "is_top_story": true},
			{"article_id": ids[0], "top_story_order": 2, "is_top_story": true},
		},
	}
	ordBytes, _ := json.Marshal(orderPayload)
	req := httptest.NewRequest(http.MethodPut, "/v1/cms/top-stories", bytes.NewReader(ordBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating top stories, got %d: %s", w.Code, w.Body.String())
	}

	// Query public /v1/top-stories
	req = httptest.NewRequest(http.MethodGet, "/v1/top-stories", nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /v1/top-stories, got %d", w.Code)
	}

	var topResp struct {
		Stories []queries.ArticleFeedItem `json:"stories"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &topResp)
	if len(topResp.Stories) < 2 {
		t.Fatalf("expected at least 2 top stories, got %d", len(topResp.Stories))
	}
	if topResp.Stories[0].ID != ids[1] {
		t.Errorf("expected first top story to be ID %s, got %s", ids[1], topResp.Stories[0].ID)
	}
}


