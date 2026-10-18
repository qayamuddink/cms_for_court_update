package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/livelaw/backend/internal/adapters/outbound/hasher"
	"github.com/livelaw/backend/internal/adapters/outbound/idgen"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	searchAdapter "github.com/livelaw/backend/internal/adapters/outbound/search"
	"github.com/livelaw/backend/internal/adapters/outbound/token"
	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/content"
	"github.com/livelaw/backend/internal/domain/identity"
	"github.com/livelaw/backend/internal/domain/judgment"
	"github.com/livelaw/backend/internal/domain/livecase"
	"github.com/livelaw/backend/migrations"
)

type searchTestEnv struct {
	router          http.Handler
	db              *sql.DB
	searchIndex     ports.SearchIndex
	searchIndexer   *commands.SearchIndexer
	searchQueries   *queries.SearchQueries
	articleRepo     ports.ArticleRepository
	judgmentRepo    ports.JudgmentRepository
	caseRepo        ports.CourtCaseRepository
	sectionRepo     ports.SectionRepository
	outboxRepo      ports.OutboxRepository
	editorToken     string
	readerToken     string
	subscriberToken string
	editorUserID    string
}

func setupSearchTestEnv(t *testing.T) *searchTestEnv {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://livelaw:livelaw_secret@localhost:5432/livelaw?sslmode=disable"
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatalf("failed connecting to test db: %v", err)
	}

	migrator := postgres.NewMigrator(db, migrations.FS)
	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("failed applying migrations: %v", err)
	}

	// Clean test tables
	_, _ = db.Exec(`
		DELETE FROM search_analytics;
		DELETE FROM judgment_downloads;
		DELETE FROM judgment_pages;
		DELETE FROM judgment_statutes;
		DELETE FROM judgments;
		DELETE FROM article_revisions;
		DELETE FROM article_slug_redirects;
		DELETE FROM articles;
		DELETE FROM live_updates;
		DELETE FROM case_assignments;
		DELETE FROM court_cases;
		DELETE FROM outbox_events;
		DELETE FROM users WHERE email LIKE '%@testsearch.com';
	`)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	clock := ports.RealClock{}
	idGen := idgen.NewUUIDv7Generator()
	pwHasher := hasher.NewBcryptHasher(4)
	tokenService := token.NewJWTService("search-test-secret-super-long-key-for-jwt", "test", 15*time.Minute, 24*time.Hour, clock)

	userRepo := postgres.NewSQLUserRepository(db)
	tokenRepo := postgres.NewSQLRefreshTokenRepository(db)
	subRepo := postgres.NewSQLSubscriptionRepository(db)
	articleRepo := postgres.NewSQLArticleRepository(db)
	judgmentRepo := postgres.NewSQLJudgmentRepository(db)
	caseRepo := postgres.NewSQLCourtCaseRepository(db)
	sectionRepo := postgres.NewSQLSectionRepository(db)
	outboxRepo := postgres.NewSQLOutboxRepository(db)
	analyticsRepo := postgres.NewSQLSearchAnalyticsRepository(db)

	searchIndex := searchAdapter.NewMemorySearchIndex()
	searchIndexer := commands.NewSearchIndexer(searchIndex, articleRepo, judgmentRepo, caseRepo, sectionRepo, logger)
	searchQueries := queries.NewSearchQueries(searchIndex, analyticsRepo)

	authUseCases := commands.NewAuthUseCases(userRepo, tokenRepo, pwHasher, tokenService, idGen, clock, subRepo)
	userQueries := queries.NewUserQueries(userRepo)

	seeder := postgres.NewSeeder(userRepo, pwHasher, idGen, clock, logger)
	_ = seeder.SeedUsers(context.Background())

	router := NewRouter(RouterConfig{
		Logger:        logger,
		Clock:         clock,
		TokenService:  tokenService,
		AuthUseCases:  authUseCases,
		UserQueries:   userQueries,
		SearchQueries: searchQueries,
		SearchIndexer: searchIndexer,
	})

	editorLogin := loginHelper(t, router, "editor@livelaw.in", "EditorPass123!")
	readerLogin := loginHelper(t, router, "reader@livelaw.in", "ReaderPass123!")

	// Create subscriber token with premium tier
	subTok, _, _ := tokenService.GenerateAccessToken(&identity.User{
		ID:         readerLogin.User.ID,
		Email:      readerLogin.User.Email,
		SystemRole: identity.RoleReader,
		Status:     identity.StatusActive,
	}, ports.EntitlementClaim{Tier: "premium", ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix()})

	return &searchTestEnv{
		router:          router,
		db:              db,
		searchIndex:     searchIndex,
		searchIndexer:   searchIndexer,
		searchQueries:   searchQueries,
		articleRepo:     articleRepo,
		judgmentRepo:    judgmentRepo,
		caseRepo:        caseRepo,
		sectionRepo:     sectionRepo,
		outboxRepo:      outboxRepo,
		editorToken:     editorLogin.AccessToken,
		readerToken:     readerLogin.AccessToken,
		subscriberToken: subTok,
		editorUserID:    editorLogin.User.ID,
	}
}

func TestSearch_IndexSyncAfterPublishAndUnpublish(t *testing.T) {
	env := setupSearchTestEnv(t)
	defer env.db.Close()
	ctx := context.Background()

	// 1. Create and Publish an Article directly
	artID := "01925b6a-7422-7744-88aa-112233445566"
	now := time.Now().UTC()
	art := &content.Article{
		ID:                 artID,
		Slug:               "supreme-court-bail-principles-verdict",
		Title:              "Supreme Court Lays Down Comprehensive Bail Principles",
		Summary:            "A landmark ruling governing personal liberty and timely bail adjudication.",
		BodyBlocks:         []byte(`[{"type":"paragraph","content":"The bench observed that jail is the exception and bail is the rule."}]`),
		ArticleType:        content.ArticleTypeNews,
		AccessLevel:        content.AccessLevelFree,
		Status:             content.StatusPublished,
		AuthorID:           env.editorUserID,
		Category:           "Supreme Court",
		Tags:               []string{"bail", "liberty"},
		WordCount:          350,
		ReadingTimeMinutes: 2,
		PublishedAt:        &now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := env.articleRepo.Create(ctx, art); err != nil {
		t.Fatalf("failed creating article: %v", err)
	}

	// 2. Publish outbox event to indexer
	event := ports.OutboxEvent{
		ID:            "01925b6a-7422-7744-88aa-998877665544",
		AggregateType: "article",
		AggregateID:   artID,
		EventType:     "article.published",
		SchemaVersion: 1,
		CreatedAt:     now.Format(time.RFC3339),
	}
	if err := env.searchIndexer.Publish(ctx, event); err != nil {
		t.Fatalf("failed syncing indexer: %v", err)
	}

	// 3. Search for published article
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=Bail+Principles", nil)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	totalHits := int(res["total_hits"].(float64))
	if totalHits != 1 {
		t.Fatalf("expected 1 hit, got %d", totalHits)
	}

	hits := res["hits"].([]interface{})
	hit := hits[0].(map[string]interface{})
	if hit["title"] != "Supreme Court Lays Down Comprehensive Bail Principles" {
		t.Errorf("unexpected hit title: %v", hit["title"])
	}
	if !strings.Contains(hit["snippet"].(string), "<mark>Bail") {
		t.Errorf("expected highlighted snippet with mark tag, got: %v", hit["snippet"])
	}

	// 4. Archive article and sync unpublish event
	unpublishEvent := ports.OutboxEvent{
		ID:            "01925b6a-7422-7744-88aa-998877665545",
		AggregateType: "article",
		AggregateID:   artID,
		EventType:     "article.archived",
		SchemaVersion: 1,
		CreatedAt:     now.Format(time.RFC3339),
	}
	if err := env.searchIndexer.Publish(ctx, unpublishEvent); err != nil {
		t.Fatalf("failed syncing unpublish event: %v", err)
	}

	// 5. Search again -> should return 0 hits
	req2 := httptest.NewRequest(http.MethodGet, "/v1/search?q=Bail+Principles", nil)
	rec2 := httptest.NewRecorder()
	env.router.ServeHTTP(rec2, req2)

	var res2 map[string]interface{}
	_ = json.Unmarshal(rec2.Body.Bytes(), &res2)
	if int(res2["total_hits"].(float64)) != 0 {
		t.Errorf("expected 0 hits after unpublish, got %v", res2["total_hits"])
	}
}

func TestSearch_StatuteSectionAndPDFTextWithSnippetAndPageNumber(t *testing.T) {
	env := setupSearchTestEnv(t)
	defer env.db.Close()
	ctx := context.Background()

	// Create and publish an official judgment
	jID := "01925b6a-7422-7744-88aa-333333333333"
	citation := "2026 LiveLaw (SC) 42"
	citationYear := 2026
	citationCourt := "SC"
	citationNum := 42
	now := time.Now().UTC()

	j := &judgment.Judgment{
		ID:                jID,
		Citation:          &citation,
		CitationYear:      &citationYear,
		CitationCourtCode: &citationCourt,
		CitationNumber:    &citationNum,
		CaseTitle:         "M/s ABC Enterprises v. State of Maharashtra",
		CaseNumber:        "Crl.A. No. 1024/2026",
		Court:             "Supreme Court of India",
		CourtCode:         "SC",
		Bench:             []string{"D.Y. Chandrachud, CJI", "P.S. Narasimha, J."},
		JudgmentDate:      now,
		Appellant:         "M/s ABC Enterprises",
		Respondent:        "State of Maharashtra",
		Advocates:         []string{"Kapil Sibal, Sr. Adv."},
		Issues:            "Dishonour of cheque under NI Act",
		Held:              "Statutory notice under Section 138 is mandatory before filing complaint.",
		Headnote:          "Negotiable Instruments Act 1881 - Section 138 - Notice period compliance.",
		Summary:           "Appeal allowed clarifying the calculation of 15 days period under Section 138 proviso.",
		AccessLevel:       judgment.AccessLevelFree,
		Status:            judgment.StatusPublished,
		ExtractionStatus:  judgment.ExtractionCompleted,
		CreatedBy:         env.editorUserID,
		PublishedAt:       &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := env.judgmentRepo.Create(ctx, j); err != nil {
		t.Fatalf("failed creating judgment: %v", err)
	}

	// Add statutes
	_ = env.judgmentRepo.AddStatutes(ctx, jID, []judgment.StatuteCited{
		{
			ID:            "01925b6a-7422-7744-88aa-444444444441",
			JudgmentID:    jID,
			StatuteName:   "Negotiable Instruments Act",
			SectionsCited: []string{"Section 138", "Section 141"},
		},
	})

	// Add extracted PDF text page with specific text on Page 7
	_ = env.judgmentRepo.SavePages(ctx, jID, []judgment.JudgmentPage{
		{
			ID:          "01925b6a-7422-7744-88aa-555555555551",
			JudgmentID:  jID,
			PageNumber:  1,
			TextContent: "IN THE SUPREME COURT OF INDIA. Criminal Appellate Jurisdiction.",
			CreatedAt:   now,
		},
		{
			ID:          "01925b6a-7422-7744-88aa-555555555552",
			JudgmentID:  jID,
			PageNumber:  7,
			TextContent: "Upon careful examination of the record, the bench holds that Section 138 of the Negotiable Instruments Act requires strict compliance with notice delivery.",
			CreatedAt:   now,
		},
	})

	// Sync to search index
	event := ports.OutboxEvent{
		ID:            "01925b6a-7422-7744-88aa-998877665546",
		AggregateType: "judgment",
		AggregateID:   jID,
		EventType:     "judgment.published",
		SchemaVersion: 1,
		CreatedAt:     now.Format(time.RFC3339),
	}
	if err := env.searchIndexer.Publish(ctx, event); err != nil {
		t.Fatalf("failed syncing judgment: %v", err)
	}

	// Search specifically for "Section 138"
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=Section+138", nil)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if int(res["total_hits"].(float64)) != 1 {
		t.Fatalf("expected 1 hit, got %v", res["total_hits"])
	}

	hit := res["hits"].([]interface{})[0].(map[string]interface{})
	if hit["title"] != "M/s ABC Enterprises v. State of Maharashtra" {
		t.Errorf("unexpected judgment title: %v", hit["title"])
	}
	if hit["citation"] != "2026 LiveLaw (SC) 42" {
		t.Errorf("unexpected citation: %v", hit["citation"])
	}

	// VERIFY DEFINITION OF DONE:
	// "searching for a statute section returns judgments where it appears, with a snippet and page number"
	if hit["matched_page_number"] == nil {
		t.Fatalf("expected matched_page_number to be populated from PDF page extraction")
	}
	matchedPage := int(hit["matched_page_number"].(float64))
	if matchedPage != 7 {
		t.Errorf("expected matched page 7, got %d", matchedPage)
	}

	snippet := hit["snippet"].(string)
	if !strings.Contains(snippet, "<mark>Section 138</mark>") {
		t.Errorf("expected snippet to contain highlighted <mark>Section 138</mark>, got: %s", snippet)
	}
	if !strings.Contains(snippet, "(Page 7)") {
		t.Errorf("expected snippet to contain page attribution '(Page 7)', got: %s", snippet)
	}
}

func TestSearch_FilterCombinationsAndExactCitation(t *testing.T) {
	env := setupSearchTestEnv(t)
	defer env.db.Close()
	ctx := context.Background()

	now := time.Now().UTC()
	cit := "2026 LiveLaw (SC) 100"
	citYear := 2026
	citCourt := "SC"
	citNum := 100

	j := &judgment.Judgment{
		ID:                "01925b6a-7422-7744-88aa-777777777771",
		Citation:          &cit,
		CitationYear:      &citYear,
		CitationCourtCode: &citCourt,
		CitationNumber:    &citNum,
		CaseTitle:         "Union of India v. Telecom Operators Association",
		CaseNumber:        "C.A. No. 500/2026",
		Court:             "Supreme Court of India",
		CourtCode:         "SC",
		Bench:             []string{"D.Y. Chandrachud, CJI", "Sanjiv Khanna, J."},
		JudgmentDate:      now,
		Appellant:         "Union of India",
		Respondent:        "Telecom Operators Association",
		Advocates:         []string{"Solicitor General Tushar Mehta"},
		Issues:            "Spectrum allocation and licensing dues",
		Held:              "Adjusted gross revenue calculations upheld.",
		Headnote:          "Telecom Regulatory Authority of India Act - AGR computation.",
		Summary:           "Supreme Court disposes AGR challenge.",
		AccessLevel:       judgment.AccessLevelFree,
		Status:            judgment.StatusPublished,
		ExtractionStatus:  judgment.ExtractionCompleted,
		CreatedBy:         env.editorUserID,
		PublishedAt:       &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	_ = env.judgmentRepo.Create(ctx, j)

	_ = env.searchIndexer.Publish(ctx, ports.OutboxEvent{
		ID:            "01925b6a-7422-7744-88aa-998877665547",
		AggregateType: "judgment",
		AggregateID:   j.ID,
		EventType:     "judgment.published",
		SchemaVersion: 1,
		CreatedAt:     now.Format(time.RFC3339),
	})

	// 1. Exact Citation match test: typing exact citation returns exact_citation_match
	exactReq := httptest.NewRequest(http.MethodGet, "/v1/search?q=2026+LiveLaw+(SC)+100", nil)
	exactRec := httptest.NewRecorder()
	env.router.ServeHTTP(exactRec, exactReq)

	if exactRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", exactRec.Code)
	}

	var exactRes map[string]interface{}
	_ = json.Unmarshal(exactRec.Body.Bytes(), &exactRes)
	if exactRes["exact_citation_match"] == nil {
		t.Fatalf("expected exact_citation_match to be populated for exact citation query")
	}
	exactHit := exactRes["exact_citation_match"].(map[string]interface{})
	if exactHit["citation"] != "2026 LiveLaw (SC) 100" {
		t.Errorf("unexpected exact citation: %v", exactHit["citation"])
	}

	// 2. Filter combinations: court=SC, year=2026, judge=Chandrachud
	filterURL := fmt.Sprintf("/v1/search?court=SC&year=2026&judge=%s", url.QueryEscape("Chandrachud"))
	filterReq := httptest.NewRequest(http.MethodGet, filterURL, nil)
	filterReq.Header.Set("Authorization", "Bearer "+env.subscriberToken)
	filterRec := httptest.NewRecorder()
	env.router.ServeHTTP(filterRec, filterReq)

	if filterRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", filterRec.Code)
	}
	var filterRes map[string]interface{}
	_ = json.Unmarshal(filterRec.Body.Bytes(), &filterRes)
	if int(filterRes["total_hits"].(float64)) != 1 {
		t.Errorf("expected 1 hit with filter combination, got %v", filterRes["total_hits"])
	}
}

func TestSearch_EntitlementGating(t *testing.T) {
	env := setupSearchTestEnv(t)
	defer env.db.Close()
	ctx := context.Background()

	now := time.Now().UTC()
	cit := "2026 LiveLaw (SC) 200"
	citYear := 2026
	citCourt := "SC"
	citNum := 200

	premiumJ := &judgment.Judgment{
		ID:                "01925b6a-7422-7744-88aa-888888888881",
		Citation:          &cit,
		CitationYear:      &citYear,
		CitationCourtCode: &citCourt,
		CitationNumber:    &citNum,
		CaseTitle:         "Confidential Commercial Arbitration v. Global Corp",
		CaseNumber:        "Arb. Pet. No. 99/2026",
		Court:             "Supreme Court of India",
		CourtCode:         "SC",
		Bench:             []string{"Sanjiv Khanna, J."},
		JudgmentDate:      now,
		Appellant:         "Confidential Commercial Arbitration",
		Respondent:        "Global Corp",
		Issues:            "Arbitrability of fraud claims",
		Held:              "Complex commercial fraud is arbitrable unless serious allegations.",
		Summary:           "Detailed premium analysis of foreign seat arbitration enforcement.",
		AccessLevel:       judgment.AccessLevelPremium,
		Status:            judgment.StatusPublished,
		ExtractionStatus:  judgment.ExtractionCompleted,
		CreatedBy:         env.editorUserID,
		PublishedAt:       &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	_ = env.judgmentRepo.Create(ctx, premiumJ)

	_ = env.searchIndexer.Publish(ctx, ports.OutboxEvent{
		ID:            "01925b6a-7422-7744-88aa-998877665548",
		AggregateType: "judgment",
		AggregateID:   premiumJ.ID,
		EventType:     "judgment.published",
		SchemaVersion: 1,
		CreatedAt:     now.Format(time.RFC3339),
	})

	// 1. Anonymous or Free reader using advanced filters -> gets upgrade required signal without hard error
	freeReq := httptest.NewRequest(http.MethodGet, "/v1/search?q=Arbitration&judge=Khanna", nil)
	freeRec := httptest.NewRecorder()
	env.router.ServeHTTP(freeRec, freeReq)

	if freeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for free search, got %d", freeRec.Code)
	}

	var freeRes map[string]interface{}
	_ = json.Unmarshal(freeRec.Body.Bytes(), &freeRes)
	if freeRes["is_upgrade_required"] != true {
		t.Errorf("expected is_upgrade_required to be true for advanced filter query by free reader")
	}
	if freeRes["upgrade_message"] == nil || freeRes["upgrade_message"] == "" {
		t.Errorf("expected upgrade_message explaining premium benefit")
	}

	// 2. Subscriber executing search -> full entitlement, no upgrade signal
	subReq := httptest.NewRequest(http.MethodGet, "/v1/search?q=Arbitration&judge=Khanna", nil)
	subReq.Header.Set("Authorization", "Bearer "+env.subscriberToken)
	subRec := httptest.NewRecorder()
	env.router.ServeHTTP(subRec, subReq)

	if subRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for subscriber, got %d", subRec.Code)
	}

	var subRes map[string]interface{}
	_ = json.Unmarshal(subRec.Body.Bytes(), &subRes)
	if subRes["is_upgrade_required"] == true {
		t.Errorf("subscriber should not be flagged as upgrade required")
	}
}

func TestSearch_ReindexIdempotencyAndCMSHealth(t *testing.T) {
	env := setupSearchTestEnv(t)
	defer env.db.Close()
	ctx := context.Background()

	// Seed one court case and one article
	c := &livecase.CourtCase{
		ID:                "01925b6a-7422-7744-88aa-999999999991",
		CourtName:         "Supreme Court of India",
		CaseNumber:        "W.P.(C) No. 101/2026",
		CaseType:          "Writ Petition",
		Title:             "Right to Privacy in Digital Communications",
		PresidingJudge:    "D.Y. Chandrachud, CJI",
		BenchJudges:       []string{"D.Y. Chandrachud, CJI", "Hrishikesh Roy, J."},
		PetitionerParties: []string{"Citizen Rights Foundation"},
		RespondentParties: []string{"Union of India"},
		Status:            livecase.CaseStatusListed,
		IsLive:            true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	_ = env.caseRepo.Create(ctx, c)

	now := time.Now()
	art := &content.Article{
		ID:          "01925b6a-7422-7744-88aa-999999999992",
		Slug:        "privacy-in-digital-era",
		Title:       "Privacy in Digital Era Landmark Verdict",
		Summary:     "Supreme Court elaborates on digital privacy principles.",
		ArticleType: content.ArticleTypeNews,
		Status:      content.StatusPublished,
		AuthorID:    env.editorUserID,
		Category:    "Supreme Court",
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_ = env.articleRepo.Create(ctx, art)

	// Trigger full re-index 1
	req1 := httptest.NewRequest(http.MethodPost, "/v1/cms/search/reindex", nil)
	req1.Header.Set("Authorization", "Bearer "+env.editorToken)
	rec1 := httptest.NewRecorder()
	env.router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("first reindex failed: %d: %s", rec1.Code, rec1.Body.String())
	}

	var reindexRes1 map[string]interface{}
	_ = json.Unmarshal(rec1.Body.Bytes(), &reindexRes1)
	stats1 := reindexRes1["stats"].(map[string]interface{})
	docsCount1 := int(stats1["total_documents"].(float64))

	// Trigger full re-index 2 (idempotency check)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/cms/search/reindex", nil)
	req2.Header.Set("Authorization", "Bearer "+env.editorToken)
	rec2 := httptest.NewRecorder()
	env.router.ServeHTTP(rec2, req2)

	var reindexRes2 map[string]interface{}
	_ = json.Unmarshal(rec2.Body.Bytes(), &reindexRes2)
	stats2 := reindexRes2["stats"].(map[string]interface{})
	docsCount2 := int(stats2["total_documents"].(float64))

	if docsCount1 != docsCount2 {
		t.Errorf("reindex is not idempotent: count1=%d, count2=%d", docsCount1, docsCount2)
	}

	// Check CMS Health endpoint
	healthReq := httptest.NewRequest(http.MethodGet, "/v1/cms/search/health", nil)
	healthReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	healthRec := httptest.NewRecorder()
	env.router.ServeHTTP(healthRec, healthReq)

	if healthRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from search health, got %d", healthRec.Code)
	}
}

func TestSearch_AutosuggestAndAnalytics(t *testing.T) {
	env := setupSearchTestEnv(t)
	defer env.db.Close()
	ctx := context.Background()

	// Seed data for autosuggest
	c := &livecase.CourtCase{
		ID:                "01925b6a-7422-7744-88aa-111122223333",
		CourtName:         "Supreme Court of India",
		CaseNumber:        "C.A. No. 12/2026",
		CaseType:          "Civil Appeal",
		Title:             "State of Maharashtra v. Union of India",
		PresidingJudge:    "B.R. Gavai, J.",
		PetitionerParties: []string{"State of Maharashtra"},
		RespondentParties: []string{"Union of India"},
		Status:            livecase.CaseStatusListed,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	_ = env.caseRepo.Create(ctx, c)
	_, _ = env.searchIndexer.ReindexAll(ctx)

	// 1. Test Autosuggest
	autoReq := httptest.NewRequest(http.MethodGet, "/v1/search/autosuggest?prefix=Maharashtra", nil)
	autoRec := httptest.NewRecorder()
	env.router.ServeHTTP(autoRec, autoReq)

	if autoRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from autosuggest, got %d", autoRec.Code)
	}

	var autoRes map[string]interface{}
	_ = json.Unmarshal(autoRec.Body.Bytes(), &autoRes)
	caseNames := autoRes["case_names"].([]interface{})
	if len(caseNames) == 0 {
		t.Fatalf("expected case_names suggestion for prefix 'Maharashtra'")
	}

	// 2. Search query tracking and zero-result analytics
	zeroReq := httptest.NewRequest(http.MethodGet, "/v1/search?q=NonExistentLegalTopicXYZ", nil)
	zeroRec := httptest.NewRecorder()
	env.router.ServeHTTP(zeroRec, zeroReq)

	// Fetch CMS analytics
	anaReq := httptest.NewRequest(http.MethodGet, "/v1/cms/search/analytics", nil)
	anaReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	anaRec := httptest.NewRecorder()
	env.router.ServeHTTP(anaRec, anaReq)

	if anaRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from analytics, got %d", anaRec.Code)
	}

	var anaRes map[string]interface{}
	_ = json.Unmarshal(anaRec.Body.Bytes(), &anaRes)
	totalSearches := int(anaRes["total_searches"].(float64))
	if totalSearches < 1 {
		t.Errorf("expected at least 1 recorded search in analytics, got %d", totalSearches)
	}
}
