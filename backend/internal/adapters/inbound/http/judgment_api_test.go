package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/livelaw/backend/internal/adapters/outbound/entitlement"
	"github.com/livelaw/backend/internal/adapters/outbound/extractor"
	"github.com/livelaw/backend/internal/adapters/outbound/filestore"
	"github.com/livelaw/backend/internal/adapters/outbound/hasher"
	"github.com/livelaw/backend/internal/adapters/outbound/idgen"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	"github.com/livelaw/backend/internal/adapters/outbound/scanner"
	"github.com/livelaw/backend/internal/adapters/outbound/token"
	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/judgment"
	"github.com/livelaw/backend/migrations"
)

type judgmentTestEnv struct {
	router           http.Handler
	db               *sql.DB
	judgmentRepo     ports.JudgmentRepository
	judgmentUseCases *commands.JudgmentUseCases
	judgmentQueries  *queries.JudgmentQueryUseCases
	fileStore        *filestore.MinIOFileStore
	extractor        ports.PDFExtractor
	scanner          ports.VirusScanner
	worker           *commands.PDFExtractionWorker
	editorToken      string
	readerToken      string
	subscriberToken  string
	subscriberUserID string
}

func setupJudgmentTestEnv(t *testing.T) *judgmentTestEnv {
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
	subRepo := postgres.NewSQLSubscriptionRepository(db)
	pwHasher := hasher.NewBcryptHasher(4)
	tokenService := token.NewJWTService("phase-10-test-jwt-secret-key-32-bytes-long", "test", 15*time.Minute, 7*24*time.Hour, clock)

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

	judgmentRepo := postgres.NewSQLJudgmentRepository(db)
	virusScanner := scanner.NewStubScanner("")
	pdfExtractor := extractor.NewStandardPDFExtractor(nil)
	entitlementChecker := entitlement.NewSQLSubscriptionChecker(db, userRepo)

	authUseCases := commands.NewAuthUseCases(userRepo, tokenRepo, pwHasher, tokenService, idGenerator, clock, subRepo)
	userQueries := queries.NewUserQueries(userRepo)
	judgmentUseCases := commands.NewJudgmentUseCases(judgmentRepo, fileStore, virusScanner, pdfExtractor, idGenerator, clock)
	judgmentQueries := queries.NewJudgmentQueryUseCases(judgmentRepo, entitlementChecker, fileStore, clock)
	worker := commands.NewPDFExtractionWorker(judgmentRepo, fileStore, pdfExtractor, idGenerator, clock, 1*time.Second)

	router := NewRouter(RouterConfig{
		Logger:           logger,
		DBHealthChecker:  dbHealthChecker,
		Clock:            clock,
		TokenService:     tokenService,
		AuthUseCases:     authUseCases,
		UserQueries:      userQueries,
		JudgmentUseCases: judgmentUseCases,
		JudgmentQueries:  judgmentQueries,
	})

	editorLogin := loginHelper(t, router, "editor@livelaw.in", "EditorPass123!")
	readerLogin := loginHelper(t, router, "reader@livelaw.in", "ReaderPass123!")

	// Create active subscriber user
	subEmail := fmt.Sprintf("subscriber-%d@test.com", time.Now().UnixNano())
	regBody, _ := json.Marshal(map[string]string{
		"email":        subEmail,
		"password":     "ReaderPass123!",
		"full_name":    "Subscriber User",
		"display_role": "lawyer",
	})
	regReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	router.ServeHTTP(regW, regReq)

	subLogin := loginHelper(t, router, subEmail, "ReaderPass123!")
	subUserID := subLogin.User.ID

	// Insert active subscription in DB
	provSubID := fmt.Sprintf("sub_prov_%s_%d", subUserID[:8], time.Now().UnixNano())
	_, err = db.ExecContext(ctx, `
		INSERT INTO subscriptions (id, user_id, tier, status, started_at, current_period_end, auto_renew, payment_provider, provider_subscription_id, price_amount_minor, currency, created_at, updated_at)
		VALUES ($1, $2, 'annual', 'active', NOW() - INTERVAL '1 day', NOW() + INTERVAL '364 days', true, 'razorpay', $3, 999900, 'INR', NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, idGenerator.NewID(), subUserID, provSubID)
	if err != nil {
		t.Fatalf("failed inserting test subscription: %v", err)
	}

	return &judgmentTestEnv{
		router:           router,
		db:               db,
		judgmentRepo:     judgmentRepo,
		judgmentUseCases: judgmentUseCases,
		judgmentQueries:  judgmentQueries,
		fileStore:        fileStore,
		extractor:        pdfExtractor,
		scanner:          virusScanner,
		worker:           worker,
		editorToken:      editorLogin.AccessToken,
		readerToken:      readerLogin.AccessToken,
		subscriberToken:  subLogin.AccessToken,
		subscriberUserID: subUserID,
	}
}

// Test 1: Atomic, gapless sequential citation allocation under concurrency (50 parallel workers)
func TestJudgment_ConcurrencyCitationAllocation(t *testing.T) {
	env := setupJudgmentTestEnv(t)
	ctx := context.Background()

	testCourtCode := "SC"
	testYear := 2026
	concurrency := 50

	// Reset citation counter for this test court and year
	_, _ = env.db.ExecContext(ctx, "DELETE FROM citation_counters WHERE court_code = $1 AND year = $2", testCourtCode, testYear)

	var wg sync.WaitGroup
	citations := make([]string, concurrency)
	numbers := make([]int, concurrency)
	errorsList := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cit, num, err := env.judgmentRepo.AllocateCitation(ctx, testCourtCode, testYear, "CourtDesk")
			citations[idx] = cit
			numbers[idx] = num
			errorsList[idx] = err
		}(i)
	}

	wg.Wait()

	// Verify all 50 completed without error
	for i, err := range errorsList {
		if err != nil {
			t.Fatalf("worker %d failed to allocate citation: %v", i, err)
		}
	}

	// Verify all 50 citation numbers are unique and gapless from 1 to 50
	sort.Ints(numbers)
	for i := 0; i < concurrency; i++ {
		expected := i + 1
		if numbers[i] != expected {
			t.Errorf("gapless check failed at position %d: expected %d, got %d", i, expected, numbers[i])
		}
	}

	// Check citation format
	for _, cit := range citations {
		expectedPrefix := fmt.Sprintf("%d CourtDesk (%s) ", testYear, testCourtCode)
		if !strings.HasPrefix(cit, expectedPrefix) {
			t.Errorf("citation %q does not match expected format %s<N>", cit, expectedPrefix)
		}
	}
}

// Test 2: PDF Upload Rules & Security (Magic bytes check, size limit, deduplication, virus scan)
func TestJudgment_PDFSecurityAndDeduplication(t *testing.T) {
	env := setupJudgmentTestEnv(t)

	// 1. Create a draft judgment
	draftPayload, _ := json.Marshal(map[string]any{
		"case_title":    "State of Maharashtra v. ABC Corp",
		"case_number":   "Civil Appeal No. 101/2026",
		"court":         "Supreme Court of India",
		"court_code":    "SC",
		"bench":         []string{"Hon'ble Mr. Justice DY Chandrachud", "Hon'ble Mr. Justice Sanjiv Khanna"},
		"judgment_date": "2026-03-15",
		"appellant":     "State of Maharashtra",
		"respondent":    "ABC Corp",
		"advocates":     []string{"Harish Salve", "Mukul Rohatgi"},
		"summary":       "Supreme Court clarifies statutory scope under arbitration legislation.",
		"access_level":  "free",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/cms/judgments", bytes.NewReader(draftPayload))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for judgment draft, got %d: %s", w.Code, w.Body.String())
	}

	var createdResp struct {
		Judgment judgment.Judgment `json:"judgment"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &createdResp)
	judgmentID := createdResp.Judgment.ID

	// 2. Magic bytes rejection: upload invalid file with fake .pdf name
	fakePDF := []byte("THIS IS A TEXT FILE RENAMED TO .PDF WITHOUT MAGIC BYTES")
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/judgments/%s/pdf?filename=fake.pdf", judgmentID), bytes.NewReader(fakePDF))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	req.Header.Set("Content-Type", "application/pdf")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for fake PDF, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "INVALID_PDF") {
		t.Errorf("expected INVALID_PDF error code, got: %s", w.Body.String())
	}

	// 3. Virus scan rejection: upload EICAR test string
	eicarPayload := []byte("%PDF-1.4\nX5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*\n%%EOF")
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/judgments/%s/pdf?filename=eicar.pdf", judgmentID), bytes.NewReader(eicarPayload))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	req.Header.Set("Content-Type", "application/pdf")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 Unprocessable Entity for infected file, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "INFECTED_FILE") {
		t.Errorf("expected INFECTED_FILE error code, got: %s", w.Body.String())
	}

	// 4. Valid PDF upload
	validPDF := []byte(fmt.Sprintf("%%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R >>\nstream\nBT\n/F1 12 Tf\n(Official Supreme Court Judgment Body Text %d) Tj\nET\nendstream\nendobj\n%%%%EOF", time.Now().UnixNano()))
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/judgments/%s/pdf?filename=judgment.pdf", judgmentID), bytes.NewReader(validPDF))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	req.Header.Set("Content-Type", "application/pdf")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid PDF upload, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Deduplication check: create a second judgment and upload the exact same PDF
	draft2Payload, _ := json.Marshal(map[string]any{
		"case_title":    "Union of India v. XYZ Ltd",
		"case_number":   "Civil Appeal No. 102/2026",
		"court":         "Supreme Court of India",
		"court_code":    "SC",
		"judgment_date": "2026-03-16",
		"appellant":     "Union of India",
		"respondent":    "XYZ Ltd",
		"summary":       "Another case attempting to upload identical judgment PDF.",
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/cms/judgments", bytes.NewReader(draft2Payload))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	var created2 struct {
		Judgment judgment.Judgment `json:"judgment"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created2)

	// Attempt duplicate upload
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/judgments/%s/pdf?filename=duplicate.pdf", created2.Judgment.ID), bytes.NewReader(validPDF))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for duplicate PDF checksum, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "DUPLICATE_PDF") {
		t.Errorf("expected DUPLICATE_PDF error code, got: %s", w.Body.String())
	}
}

// Test 3: Publishing, Headnotes, Statutes, and Background Text Extraction Worker
func TestJudgment_PublishAndExtractionWorker(t *testing.T) {
	env := setupJudgmentTestEnv(t)
	ctx := context.Background()

	// 1. Create rich judgment with headnotes, issues, held, and statutes
	draftPayload, _ := json.Marshal(map[string]any{
		"case_title":    "Tata Consultancy v. Cyber Law Commission",
		"case_number":   "W.P.(C) 4500/2026",
		"court":         "Delhi High Court",
		"court_code":    "DHC",
		"bench":         []string{"Hon'ble Ms. Justice Prathiba M. Singh"},
		"judgment_date": "2026-04-10",
		"appellant":     "Tata Consultancy",
		"respondent":    "Cyber Law Commission",
		"advocates":     []string{"Senior Adv. Abhishek Manu Singhvi"},
		"subject_tags":  []string{"Arbitration", "Intellectual Property"},
		"issues":        "Whether interim measures can be granted post-arbitral award.",
		"held":          "Section 9 relief is available prior to enforcement of award under Section 36.",
		"headnote":      "Arbitration and Conciliation Act 1996 - Section 9 and Section 34 - Scope of post-award protection.",
		"summary":       "Delhi High Court grants interim protection pending section 34 challenge.",
		"key_takeaways": []string{"Post-award interim relief permissible", "Balance of convenience favors protection"},
		"statutes": []map[string]any{
			{
				"statute_name":   "Arbitration and Conciliation Act, 1996",
				"sections_cited": []string{"Section 9", "Section 34", "Section 36"},
			},
			{
				"statute_name":   "Commercial Courts Act, 2015",
				"sections_cited": []string{"Section 13"},
			},
		},
		"access_level": "free",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/cms/judgments", bytes.NewReader(draftPayload))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var created struct {
		Judgment judgment.Judgment `json:"judgment"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	jID := created.Judgment.ID

	// 2. Upload valid PDF
	pdfBytes := []byte(fmt.Sprintf("%%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R >>\nstream\nBT\n/F1 12 Tf\n(Delhi High Court Judgment: Section 9 Arbitration Relief Granted %d) Tj\nET\nendstream\nendobj\n%%%%EOF", time.Now().UnixNano()))

	// Upload via multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "dhc_judgment.pdf")
	_, _ = part.Write(pdfBytes)
	_ = writer.Close()

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/judgments/%s/pdf", jID), &body)
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on PDF upload, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Publish judgment
	pubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/judgments/%s/publish", jID), nil)
	pubReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, pubReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on publish, got %d: %s", w.Code, w.Body.String())
	}

	var pubResp struct {
		Citation string            `json:"citation"`
		Judgment judgment.Judgment `json:"judgment"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &pubResp)

	if pubResp.Citation == "" {
		t.Fatalf("expected allocated citation, got empty")
	}
	expectedCitationPrefix := "2026 CourtDesk (DHC) "
	if !strings.HasPrefix(pubResp.Citation, expectedCitationPrefix) {
		t.Errorf("expected citation starting with %q, got %q", expectedCitationPrefix, pubResp.Citation)
	}

	// 4. Run background PDF extraction worker
	processed, err := env.worker.ProcessPendingExtractions(ctx)
	if err != nil {
		t.Fatalf("extraction worker error: %v", err)
	}
	if processed < 1 {
		t.Errorf("expected at least 1 judgment extracted, got %d", processed)
	}

	// Verify extracted pages
	pagesReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/judgments/%s/pages", jID), nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, pagesReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /pages, got %d: %s", w.Code, w.Body.String())
	}

	var pagesResp struct {
		Pages []judgment.JudgmentPage `json:"pages"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &pagesResp)

	if len(pagesResp.Pages) == 0 {
		t.Errorf("expected extracted pages in judgment_pages, got 0")
	}

	// 5. Lookup by official citation: GET /v1/citations/{citation}
	citReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/citations/%s", url.PathEscape(pubResp.Citation)), nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, citReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on citation lookup, got %d: %s", w.Code, w.Body.String())
	}

	var detail queries.JudgmentDetail
	_ = json.Unmarshal(w.Body.Bytes(), &detail)
	if detail.Judgment.ID != jID {
		t.Errorf("expected judgment ID %s, got %s", jID, detail.Judgment.ID)
	}
	if len(detail.Statutes) != 2 {
		t.Errorf("expected 2 statutes cited, got %d", len(detail.Statutes))
	}
}

// Test 4: Entitlement Gating on Premium Judgments & Audit Log Trail
func TestJudgment_EntitlementGatedDownloadAndAudit(t *testing.T) {
	env := setupJudgmentTestEnv(t)

	// Create a Premium Judgment
	draftPayload, _ := json.Marshal(map[string]any{
		"case_title":    "Confidential v. Commercial Union",
		"case_number":   "Arbitration Appeal 55/2026",
		"court":         "Supreme Court of India",
		"court_code":    "SC",
		"judgment_date": "2026-05-01",
		"appellant":     "Confidential Appellant",
		"respondent":    "Commercial Union",
		"summary":       "Landmark premium commercial arbitration decision.",
		"access_level":  "premium",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/cms/judgments", bytes.NewReader(draftPayload))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	var created struct {
		Judgment judgment.Judgment `json:"judgment"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	jID := created.Judgment.ID

	// Attach PDF
	pdfBytes := []byte(fmt.Sprintf("%%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n(Premium Judgment Content %d)\n%%%%EOF", time.Now().UnixNano()))
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/judgments/%s/pdf?filename=premium.pdf", jID), bytes.NewReader(pdfBytes))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	// 1. Anonymous reader download attempt on premium judgment -> 403 Forbidden
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/judgments/%s/download", jID), nil)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for anonymous reader on premium judgment, got %d", w.Code)
	}

	// 2. Free reader download attempt -> 403 Forbidden
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/judgments/%s/download", jID), nil)
	req.Header.Set("Authorization", "Bearer "+env.readerToken)
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for free reader on premium judgment, got %d", w.Code)
	}

	// 3. Active subscriber download attempt -> 200 OK with signed URL
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/judgments/%s/download", jID), nil)
	req.Header.Set("Authorization", "Bearer "+env.subscriberToken)
	req.Header.Set("User-Agent", "LiveLaw-Subscriber-App/1.0")
	req.RemoteAddr = "192.168.1.100:12345"
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for subscriber download, got %d: %s", w.Code, w.Body.String())
	}

	var dlResp queries.DownloadResponse
	_ = json.Unmarshal(w.Body.Bytes(), &dlResp)
	if dlResp.DownloadURL == "" || !strings.Contains(dlResp.DownloadURL, "http") {
		t.Errorf("expected valid presigned download URL, got %q", dlResp.DownloadURL)
	}

	// 4. Verify audit trail logged in PostgreSQL judgment_downloads
	var downloadCount int
	var loggedUserAgent string
	err := env.db.QueryRowContext(context.Background(), `
		SELECT COUNT(*), COALESCE(MAX(user_agent), '')
		FROM judgment_downloads
		WHERE judgment_id = $1 AND user_id = $2
	`, jID, env.subscriberUserID).Scan(&downloadCount, &loggedUserAgent)

	if err != nil {
		t.Fatalf("failed querying judgment_downloads audit table: %v", err)
	}

	if downloadCount < 1 {
		t.Errorf("expected at least 1 download logged in judgment_downloads, got %d", downloadCount)
	}
	if !strings.Contains(loggedUserAgent, "LiveLaw-Subscriber-App") {
		t.Errorf("expected logged User-Agent to contain 'LiveLaw-Subscriber-App', got %q", loggedUserAgent)
	}
}
