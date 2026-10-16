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
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/livelaw/backend/internal/adapters/outbound/hasher"
	"github.com/livelaw/backend/internal/adapters/outbound/idgen"
	"github.com/livelaw/backend/internal/adapters/outbound/liveengine"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	"github.com/livelaw/backend/internal/adapters/outbound/token"
	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/livecase"
	"github.com/livelaw/backend/migrations"
)

type testEnvironment struct {
	router         http.Handler
	db             *sql.DB
	outboxWorker   *postgres.OutboxWorker
	eventPublisher *liveengine.LogEventPublisher
	adminToken     string
	editorToken    string
	reporterToken  string
	readerToken    string
	reporterUserID string
}

func setupLiveCaseTestEnv(t *testing.T) *testEnvironment {
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
	tokenService := token.NewJWTService("phase-3-test-jwt-secret-key-32-bytes-long", "test", 15*time.Minute, 7*24*time.Hour, clock)

	// Seed canonical users
	seeder := postgres.NewSeeder(userRepo, pwHasher, idGenerator, clock, logger)
	if err := seeder.SeedUsers(ctx); err != nil {
		t.Fatalf("failed seeding users: %v", err)
	}

	caseRepo := postgres.NewSQLCourtCaseRepository(db)
	updateRepo := postgres.NewSQLLiveUpdateRepository(db)
	assignmentRepo := postgres.NewSQLCaseAssignmentRepository(db)
	outboxRepo := postgres.NewSQLOutboxRepository(db)
	txMgr := postgres.NewSQLTransactionManager(db)
	liveEngine := liveengine.NewInProcessLiveEngine(updateRepo, logger)
	eventPublisher := liveengine.NewLogEventPublisher(logger)

	outboxWorker := postgres.NewOutboxWorker(outboxRepo, eventPublisher, 50*time.Millisecond, clock, logger)

	authUseCases := commands.NewAuthUseCases(userRepo, tokenRepo, pwHasher, tokenService, idGenerator, clock)
	userQueries := queries.NewUserQueries(userRepo)
	liveCaseUseCases := commands.NewLiveCaseUseCases(caseRepo, updateRepo, assignmentRepo, outboxRepo, txMgr, liveEngine, userRepo, idGenerator, clock)
	liveCaseQueries := queries.NewLiveCaseQueries(caseRepo, updateRepo, assignmentRepo)

	router := NewRouter(RouterConfig{
		Logger:           logger,
		DBHealthChecker:  dbHealthChecker,
		Clock:            clock,
		TokenService:     tokenService,
		AuthUseCases:     authUseCases,
		UserQueries:      userQueries,
		LiveCaseUseCases: liveCaseUseCases,
		LiveCaseQueries:  liveCaseQueries,
		RateLimiter:      NewIPRateLimiter(1000, 1*time.Minute),
	})

	adminLogin := loginHelper(t, router, "admin@livelaw.in", "AdminPass123!")
	editorLogin := loginHelper(t, router, "editor@livelaw.in", "EditorPass123!")
	reporterLogin := loginHelper(t, router, "reporter@livelaw.in", "ReporterPass123!")
	readerLogin := loginHelper(t, router, "reader@livelaw.in", "ReaderPass123!")

	return &testEnvironment{
		router:         router,
		db:             db,
		outboxWorker:   outboxWorker,
		eventPublisher: eventPublisher,
		adminToken:     adminLogin.AccessToken,
		editorToken:    editorLogin.AccessToken,
		reporterToken:  reporterLogin.AccessToken,
		readerToken:    readerLogin.AccessToken,
		reporterUserID: reporterLogin.User.ID,
	}
}

func doRequest(t *testing.T, router http.Handler, method, path, token string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed marshaling body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func createTestCase(t *testing.T, env *testEnvironment, title string) livecase.CourtCase {
	t.Helper()
	reqBody := CreateCaseRequest{
		CourtName:         "Supreme Court of India",
		CaseNumber:        fmt.Sprintf("WP(C) %d/2026", time.Now().UnixNano()),
		CaseType:          "Writ Petition",
		Title:             title,
		BenchJudges:       []string{"Hon'ble CJI", "Justice A"},
		PresidingJudge:    "Hon'ble CJI",
		PetitionerParties: []string{"Citizen Alliance"},
		RespondentParties: []string{"Union of India"},
	}

	rec := doRequest(t, env.router, http.MethodPost, "/v1/cms/cases", env.editorToken, reqBody, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed creating test case (status %d): %s", rec.Code, rec.Body.String())
	}

	var created livecase.CourtCase
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("failed decoding created case: %v", err)
	}
	return created
}

func TestLiveCase_PublicEndpoints(t *testing.T) {
	env := setupLiveCaseTestEnv(t)
	c := createTestCase(t, env, "Public Live Case Test")

	// 1. Initially case is not live
	rec := doRequest(t, env.router, http.MethodGet, "/v1/cases/live", "", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Toggle live on
	setLiveRec := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/live", c.ID), env.editorToken, SetLiveRequest{IsLive: true}, nil)
	if setLiveRec.Code != http.StatusOK {
		t.Fatalf("failed toggling live on: %s", setLiveRec.Body.String())
	}

	// 3. Verify in live list
	recLive := doRequest(t, env.router, http.MethodGet, "/v1/cases/live", "", nil, nil)
	if recLive.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recLive.Code)
	}
	var liveResp LiveCasesResponse
	_ = json.NewDecoder(recLive.Body).Decode(&liveResp)
	found := false
	for _, lc := range liveResp.Cases {
		if lc.ID == c.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected case %s in live list", c.ID)
	}

	// 4. Get case details
	recGet := doRequest(t, env.router, http.MethodGet, fmt.Sprintf("/v1/cases/%s", c.ID), "", nil, nil)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for get case, got %d", recGet.Code)
	}
	var fetched livecase.CourtCase
	_ = json.NewDecoder(recGet.Body).Decode(&fetched)
	if fetched.ID != c.ID || fetched.Title != c.Title {
		t.Fatalf("mismatched case data: %+v", fetched)
	}

	// 5. Query updates
	recUpdates := doRequest(t, env.router, http.MethodGet, fmt.Sprintf("/v1/cases/%s/updates", c.ID), "", nil, nil)
	if recUpdates.Code != http.StatusOK {
		t.Fatalf("expected 200 for updates, got %d", recUpdates.Code)
	}
}

func TestLiveCase_RBAC_Updates(t *testing.T) {
	env := setupLiveCaseTestEnv(t)
	c := createTestCase(t, env, "RBAC Live Updates Test")

	updatePayload := PublishUpdatePayload{
		UpdateType:   livecase.UpdateTypeArgument,
		SpeakerLabel: stringPtr("Kapil Sibal, Senior Advocate"),
		Content:      "The petitioner argues that Article 19 safeguards apply fully to digital communications.",
	}

	// 1. Reader cannot post updates (403)
	recReader := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.readerToken, updatePayload, nil)
	if recReader.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for reader, got %d", recReader.Code)
	}

	// 2. Unassigned reporter cannot post updates (403)
	recUnassigned := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.reporterToken, updatePayload, nil)
	if recUnassigned.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for unassigned reporter, got %d: %s", recUnassigned.Code, recUnassigned.Body.String())
	}

	// 3. Editor assigns reporter to case
	recAssign := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/assign", c.ID), env.editorToken, AssignReporterRequest{UserID: env.reporterUserID}, nil)
	if recAssign.Code != http.StatusOK {
		t.Fatalf("expected 200 for assign, got %d: %s", recAssign.Code, recAssign.Body.String())
	}

	// Verify reporter in reporters list
	recListRep := doRequest(t, env.router, http.MethodGet, fmt.Sprintf("/v1/cms/cases/%s/reporters", c.ID), env.editorToken, nil, nil)
	if recListRep.Code != http.StatusOK {
		t.Fatalf("failed listing reporters: %d", recListRep.Code)
	}
	var repList ReportersResponse
	_ = json.NewDecoder(recListRep.Body).Decode(&repList)
	if len(repList.ReporterIDs) != 1 || repList.ReporterIDs[0] != env.reporterUserID {
		t.Fatalf("expected reporter %s in list, got %+v", env.reporterUserID, repList.ReporterIDs)
	}

	// 4. Assigned reporter can now post updates
	recPost := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.reporterToken, updatePayload, nil)
	if recPost.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for assigned reporter, got %d: %s", recPost.Code, recPost.Body.String())
	}
	var posted livecase.LiveUpdate
	_ = json.NewDecoder(recPost.Body).Decode(&posted)
	if posted.SequenceNo != 1 || posted.Status != livecase.UpdateStatusPublished {
		t.Fatalf("expected sequence 1 and published status, got seq %d status %s", posted.SequenceNo, posted.Status)
	}

	// 5. Editor unassigns reporter
	recUnassign := doRequest(t, env.router, http.MethodDelete, fmt.Sprintf("/v1/cms/cases/%s/assign/%s", c.ID, env.reporterUserID), env.editorToken, nil, nil)
	if recUnassign.Code != http.StatusOK {
		t.Fatalf("expected 200 for unassign, got %d", recUnassign.Code)
	}

	// 6. Reporter is now unassigned and rejected again
	recPostUnassignedAgain := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.reporterToken, updatePayload, nil)
	if recPostUnassignedAgain.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden after unassignment, got %d", recPostUnassignedAgain.Code)
	}
}

func TestLiveCase_Idempotency(t *testing.T) {
	env := setupLiveCaseTestEnv(t)
	c := createTestCase(t, env, "Idempotency Test Case")

	// Assign reporter
	_ = doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/assign", c.ID), env.editorToken, AssignReporterRequest{UserID: env.reporterUserID}, nil)

	idempKey := fmt.Sprintf("idemp-key-%d", time.Now().UnixNano())
	payload := PublishUpdatePayload{
		UpdateType:   livecase.UpdateTypeJudicialRemark,
		SpeakerLabel: stringPtr("CJI"),
		Content:      "We will examine the constitutional validity of the amendments.",
	}

	headers := map[string]string{"Idempotency-Key": idempKey}

	// First post
	rec1 := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.reporterToken, payload, headers)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first request failed: %d %s", rec1.Code, rec1.Body.String())
	}
	var u1 livecase.LiveUpdate
	_ = json.NewDecoder(rec1.Body).Decode(&u1)

	// Second post with identical idempotency key
	rec2 := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.reporterToken, payload, headers)
	if rec2.Code != http.StatusCreated && rec2.Code != http.StatusOK {
		t.Fatalf("replay request failed: %d %s", rec2.Code, rec2.Body.String())
	}
	var u2 livecase.LiveUpdate
	_ = json.NewDecoder(rec2.Body).Decode(&u2)

	if u1.ID != u2.ID || u1.SequenceNo != u2.SequenceNo {
		t.Fatalf("idempotency violated: update1=(%s, seq %d) vs update2=(%s, seq %d)", u1.ID, u1.SequenceNo, u2.ID, u2.SequenceNo)
	}

	// Verify only 1 update row exists in DB for this case
	var count int
	err := env.db.QueryRow("SELECT COUNT(*) FROM live_updates WHERE court_case_id = $1", c.ID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("expected exactly 1 live update row, got count %d, err: %v", count, err)
	}
}

func TestLiveCase_SingleVerdict(t *testing.T) {
	env := setupLiveCaseTestEnv(t)
	c := createTestCase(t, env, "Single Verdict Rule Case")
	_ = doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/assign", c.ID), env.editorToken, AssignReporterRequest{UserID: env.reporterUserID}, nil)

	verdictPayload := PublishUpdatePayload{
		UpdateType:   livecase.UpdateTypeVerdict,
		SpeakerLabel: stringPtr("Full Bench"),
		Content:      "Verdict: The petition is allowed with costs. Section 66A struck down as unconstitutional.",
	}

	// First verdict: succeeds
	rec1 := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.reporterToken, verdictPayload, nil)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first verdict, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// Second verdict: rejected with 409 Conflict
	secondVerdict := PublishUpdatePayload{
		UpdateType:   livecase.UpdateTypeVerdict,
		SpeakerLabel: stringPtr("Full Bench"),
		Content:      "Another verdict attempt.",
	}
	rec2 := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.reporterToken, secondVerdict, nil)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for second verdict, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestLiveCase_AppendOnlyCorrectionsAndRetractions(t *testing.T) {
	env := setupLiveCaseTestEnv(t)
	c := createTestCase(t, env, "Append-Only Integrity Test")
	_ = doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/assign", c.ID), env.editorToken, AssignReporterRequest{UserID: env.reporterUserID}, nil)

	// Post initial update (seq 1)
	postRec := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.reporterToken, PublishUpdatePayload{
		UpdateType:   livecase.UpdateTypeArgument,
		SpeakerLabel: stringPtr("Advocate"),
		Content:      "Initial statement that needs slight clarification.",
	}, nil)
	if postRec.Code != http.StatusCreated {
		t.Fatalf("failed initial post: %d %s", postRec.Code, postRec.Body.String())
	}
	var u1 livecase.LiveUpdate
	_ = json.NewDecoder(postRec.Body).Decode(&u1)
	if u1.SequenceNo != 1 {
		t.Fatalf("expected seq 1, got %d", u1.SequenceNo)
	}

	// 1. Post Correction (seq 2)
	correctRec := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates/%s/correct", c.ID, u1.ID), env.reporterToken, CorrectUpdatePayload{
		UpdateType:   livecase.UpdateTypeArgument,
		SpeakerLabel: stringPtr("Advocate"),
		Content:      "Clarified statement: The petitioner meant Section 43A, not 43.",
	}, nil)
	if correctRec.Code != http.StatusCreated {
		t.Fatalf("failed correction: %d %s", correctRec.Code, correctRec.Body.String())
	}
	var u2 livecase.LiveUpdate
	_ = json.NewDecoder(correctRec.Body).Decode(&u2)
	if u2.SequenceNo != 2 || u2.Status != livecase.UpdateStatusCorrected || *u2.CorrectsUpdateID != u1.ID {
		t.Fatalf("invalid correction update: %+v", u2)
	}

	// Verify original update 1 in DB remains unchanged (append-only proof!)
	var u1DBStatus string
	err := env.db.QueryRow("SELECT status FROM live_updates WHERE id = $1", u1.ID).Scan(&u1DBStatus)
	if err != nil || u1DBStatus != string(livecase.UpdateStatusPublished) {
		t.Fatalf("original update row was mutated in place! status=%s, err=%v", u1DBStatus, err)
	}

	// 2. Post Retraction (seq 3)
	retractRec := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates/%s/retract", c.ID, u1.ID), env.reporterToken, RetractUpdatePayload{
		Reason: "Retracted due to misattribution of speaker statement.",
	}, nil)
	if retractRec.Code != http.StatusCreated {
		t.Fatalf("failed retraction: %d %s", retractRec.Code, retractRec.Body.String())
	}
	var u3 livecase.LiveUpdate
	_ = json.NewDecoder(retractRec.Body).Decode(&u3)
	if u3.SequenceNo != 3 || u3.Status != livecase.UpdateStatusRetracted || *u3.CorrectsUpdateID != u1.ID {
		t.Fatalf("invalid retraction update: %+v", u3)
	}

	// 3. Attempting to correct or retract again must fail
	failCorrect := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates/%s/correct", c.ID, u1.ID), env.reporterToken, CorrectUpdatePayload{
		UpdateType: livecase.UpdateTypeArgument,
		Content:    "Attempted correction after retraction",
	}, nil)
	if failCorrect.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for correction on retracted update, got %d: %s", failCorrect.Code, failCorrect.Body.String())
	}

	failRetract := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates/%s/retract", c.ID, u1.ID), env.reporterToken, RetractUpdatePayload{
		Reason: "Second retraction attempt",
	}, nil)
	if failRetract.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for second retraction, got %d: %s", failRetract.Code, failRetract.Body.String())
	}
}

func TestLiveCase_HighConcurrency_50Updates(t *testing.T) {
	env := setupLiveCaseTestEnv(t)
	c := createTestCase(t, env, "High-Concurrency 50 Updates Benchmark")
	_ = doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/assign", c.ID), env.editorToken, AssignReporterRequest{UserID: env.reporterUserID}, nil)

	concurrency := 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	type postResult struct {
		seq int64
		err error
	}
	results := make(chan postResult, concurrency)

	// Launch 50 concurrent goroutines competing to append updates to the same court case
	for i := 1; i <= concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			payload := PublishUpdatePayload{
				UpdateType:   livecase.UpdateTypeArgument,
				SpeakerLabel: stringPtr(fmt.Sprintf("Counsel %d", idx)),
				Content:      fmt.Sprintf("Concurrent argument update statement #%d from courtroom floor.", idx),
			}

			rec := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.reporterToken, payload, nil)
			if rec.Code != http.StatusCreated {
				results <- postResult{err: fmt.Errorf("goroutine %d failed with status %d: %s", idx, rec.Code, rec.Body.String())}
				return
			}

			var u livecase.LiveUpdate
			if err := json.NewDecoder(rec.Body).Decode(&u); err != nil {
				results <- postResult{err: fmt.Errorf("failed decoding goroutine %d response: %w", idx, err)}
				return
			}
			results <- postResult{seq: u.SequenceNo}
		}(i)
	}

	wg.Wait()
	close(results)

	// Verify all 50 completed and collect sequence numbers
	seqMap := make(map[int64]bool)
	for res := range results {
		if res.err != nil {
			t.Fatalf("concurrency error: %v", res.err)
		}
		if seqMap[res.seq] {
			t.Fatalf("duplicate sequence number detected: %d", res.seq)
		}
		seqMap[res.seq] = true
	}

	// Verify sequence numbers are strictly consecutive 1 through 50 with NO gaps
	if len(seqMap) != concurrency {
		t.Fatalf("expected %d distinct sequence numbers, got %d", concurrency, len(seqMap))
	}
	for expectedSeq := int64(1); expectedSeq <= int64(concurrency); expectedSeq++ {
		if !seqMap[expectedSeq] {
			t.Fatalf("gap detected: missing expected sequence number %d", expectedSeq)
		}
	}

	// Verify court_cases row in database reflects exact last_sequence_no = 50
	var dbLastSeq int64
	err := env.db.QueryRow("SELECT last_sequence_no FROM court_cases WHERE id = $1", c.ID).Scan(&dbLastSeq)
	if err != nil || dbLastSeq != int64(concurrency) {
		t.Fatalf("database last_sequence_no expected %d, got %d, err: %v", concurrency, dbLastSeq, err)
	}
}

func TestLiveCase_TransactionalOutboxWorker(t *testing.T) {
	env := setupLiveCaseTestEnv(t)
	c := createTestCase(t, env, "Transactional Outbox Test Case")
	_ = doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/assign", c.ID), env.editorToken, AssignReporterRequest{UserID: env.reporterUserID}, nil)

	// Drain any previous test outbox backlog first
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer drainCancel()
	for {
		processed, err := env.outboxWorker.ProcessBatch(drainCtx, 50)
		if err != nil || processed == 0 {
			break
		}
	}

	// Post an update
	payload := PublishUpdatePayload{
		UpdateType:   livecase.UpdateTypeOrder,
		SpeakerLabel: stringPtr("Bench"),
		Content:      "Notice issued to respondents returnable in two weeks.",
	}
	rec := doRequest(t, env.router, http.MethodPost, fmt.Sprintf("/v1/cms/cases/%s/updates", c.ID), env.reporterToken, payload, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed posting update: %s", rec.Body.String())
	}
	var u livecase.LiveUpdate
	_ = json.NewDecoder(rec.Body).Decode(&u)

	// Check that outbox row exists
	var outboxID string
	var publishedAt *time.Time
	err := env.db.QueryRow(`
		SELECT id, published_at FROM outbox_events
		WHERE aggregate_type = 'court_case' AND aggregate_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, c.ID).Scan(&outboxID, &publishedAt)
	if err != nil {
		t.Fatalf("outbox row not found for case %s: %v", c.ID, err)
	}

	// Run outbox worker batch processing
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	processed, err := env.outboxWorker.ProcessBatch(ctx, 10)
	if err != nil {
		t.Fatalf("outbox worker batch failed: %v", err)
	}
	if processed == 0 {
		t.Fatalf("expected at least 1 event processed by worker")
	}

	// Verify published_at is now stamped in DB
	err = env.db.QueryRow("SELECT published_at FROM outbox_events WHERE id = $1", outboxID).Scan(&publishedAt)
	if err != nil || publishedAt == nil {
		t.Fatalf("outbox event %s was not marked published! err=%v, publishedAt=%v", outboxID, err, publishedAt)
	}
}

func stringPtr(s string) *string {
	return &s
}
