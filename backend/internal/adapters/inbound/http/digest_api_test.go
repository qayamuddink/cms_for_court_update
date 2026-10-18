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

	"github.com/livelaw/backend/internal/adapters/outbound/hasher"
	"github.com/livelaw/backend/internal/adapters/outbound/idgen"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	"github.com/livelaw/backend/internal/adapters/outbound/token"
	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/digest"
	"github.com/livelaw/backend/internal/domain/judgment"
	"github.com/livelaw/backend/migrations"
)

type digestTestEnv struct {
	router         http.Handler
	db             *sql.DB
	digestRepo     ports.DigestRepository
	judgmentRepo   ports.JudgmentRepository
	articleRepo    ports.ArticleRepository
	outboxRepo     ports.OutboxRepository
	digestCommands *commands.DigestCommands
	digestQueries  *queries.DigestQueries
	editorToken    string
	readerToken    string
	editorUserID   string
}

func setupDigestTestEnv(t *testing.T) *digestTestEnv {
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
		DELETE FROM digest_items;
		DELETE FROM digests;
		DELETE FROM judgment_downloads;
		DELETE FROM judgment_pages;
		DELETE FROM judgment_statutes;
		DELETE FROM judgments;
		DELETE FROM article_revisions;
		DELETE FROM article_slug_redirects;
		DELETE FROM articles;
		DELETE FROM outbox_events;
		DELETE FROM users WHERE email LIKE '%@testdigest.com';
	`)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	clock := ports.RealClock{}
	idGen := idgen.NewUUIDv7Generator()
	pwHasher := hasher.NewBcryptHasher(4)
	tokenService := token.NewJWTService("digest-test-secret-super-long-key-for-jwt", "test", 15*time.Minute, 24*time.Hour, clock)

	userRepo := postgres.NewSQLUserRepository(db)
	tokenRepo := postgres.NewSQLRefreshTokenRepository(db)
	subRepo := postgres.NewSQLSubscriptionRepository(db)
	articleRepo := postgres.NewSQLArticleRepository(db)
	judgmentRepo := postgres.NewSQLJudgmentRepository(db)
	outboxRepo := postgres.NewSQLOutboxRepository(db)
	digestRepo := postgres.NewSQLDigestRepository(db)

	digestCmds := commands.NewDigestCommands(digestRepo, judgmentRepo, articleRepo, outboxRepo, idGen, clock)
	digestQ := queries.NewDigestQueries(digestRepo)

	authUseCases := commands.NewAuthUseCases(userRepo, tokenRepo, pwHasher, tokenService, idGen, clock, subRepo)
	userQueries := queries.NewUserQueries(userRepo)

	router := NewRouter(RouterConfig{
		Logger:         logger,
		Clock:          clock,
		TokenService:   tokenService,
		AuthUseCases:   authUseCases,
		UserQueries:    userQueries,
		DigestCommands: digestCmds,
		DigestQueries:  digestQ,
	})

	seeder := postgres.NewSeeder(userRepo, pwHasher, idGen, clock, logger)
	_ = seeder.SeedUsers(context.Background())

	editorLogin := loginHelper(t, router, "editor@livelaw.in", "EditorPass123!")
	readerLogin := loginHelper(t, router, "reader@livelaw.in", "ReaderPass123!")

	return &digestTestEnv{
		router:         router,
		db:             db,
		digestRepo:     digestRepo,
		judgmentRepo:   judgmentRepo,
		articleRepo:    articleRepo,
		outboxRepo:     outboxRepo,
		digestCommands: digestCmds,
		digestQueries:  digestQ,
		editorToken:    editorLogin.AccessToken,
		readerToken:    readerLogin.AccessToken,
		editorUserID:   editorLogin.User.ID,
	}
}

func seedTestJudgmentsForDigest(t *testing.T, env *digestTestEnv) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	cases := []struct {
		id       string
		title    string
		citNum   int
		cit      string
		subject  string
		statute  string
		section  string
		headnote string
		held     string
	}{
		{
			id:       "01925b6a-7422-7744-88aa-333333333301",
			title:    "Zeta Motors Ltd v. Union of India",
			citNum:   1,
			cit:      "2026 LiveLaw (SC) 1",
			subject:  "Arbitration",
			statute:  "Arbitration and Conciliation Act 1996",
			section:  "Section 11",
			headnote: "Appointment of arbitrator in multi-party contracts.",
			held:     "Non-signatory parties can be joined under Group of Companies doctrine.",
		},
		{
			id:       "01925b6a-7422-7744-88aa-333333333302",
			title:    "Alpha Finance Corp v. Commercial Tax Officer",
			citNum:   2,
			cit:      "2026 LiveLaw (SC) 2",
			subject:  "Insolvency and Bankruptcy",
			statute:  "Insolvency and Bankruptcy Code 2016",
			section:  "Section 53",
			headnote: "Priority of dues in liquidation waterfall.",
			held:     "Statutory tax dues are subordinate to secured creditors.",
		},
		{
			id:       "01925b6a-7422-7744-88aa-333333333303",
			title:    "Beta Chemicals Ltd v. State of Gujarat",
			citNum:   3,
			cit:      "2026 LiveLaw (SC) 3",
			subject:  "Arbitration",
			statute:  "Arbitration and Conciliation Act 1996",
			section:  "Section 34",
			headnote: "Setting aside arbitral awards on grounds of patent illegality.",
			held:     "Plausible interpretation by arbitrator cannot be interfered with under Section 34.",
		},
	}

	for _, c := range cases {
		citNum := c.citNum
		citStr := c.cit
		citYear := 2026
		courtCode := "SC"
		j := &judgment.Judgment{
			ID:                c.id,
			Citation:          &citStr,
			CitationYear:      &citYear,
			CitationCourtCode: &courtCode,
			CitationNumber:    &citNum,
			CaseTitle:         c.title,
			CaseNumber:        fmt.Sprintf("C.A. No. %d/2026", c.citNum),
			Court:             "Supreme Court of India",
			CourtCode:         "SC",
			Bench:             []string{"D.Y. Chandrachud, CJI", "Sanjiv Khanna, J."},
			JudgmentDate:      now,
			Appellant:         "Appellant",
			Respondent:        "Respondent",
			Advocates:         []string{"Senior Counsel"},
			SubjectTags:       []string{c.subject},
			Headnote:          c.headnote,
			Summary:           c.held,
			Held:              c.held,
			AccessLevel:       judgment.AccessLevelFree,
			Status:            judgment.StatusPublished,
			ExtractionStatus:  judgment.ExtractionCompleted,
			CreatedBy:         env.editorUserID,
			PublishedAt:       &now,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := env.judgmentRepo.Create(ctx, j); err != nil {
			t.Fatalf("failed creating test judgment: %v", err)
		}

		if err := env.judgmentRepo.AddStatutes(ctx, c.id, []judgment.StatuteCited{
			{
				ID:            fmt.Sprintf("01925b6a-7422-7744-88aa-44444444440%d", c.citNum),
				JudgmentID:    c.id,
				StatuteName:   c.statute,
				SectionsCited: []string{c.section},
			},
		}); err != nil {
			t.Fatalf("failed adding statutes: %v", err)
		}
	}
}

func TestDigest_CreateAndAutoAssembleFromJudgments(t *testing.T) {
	env := setupDigestTestEnv(t)
	defer env.db.Close()

	seedTestJudgmentsForDigest(t, env)

	// 1. Create a Digest draft via CMS endpoint
	createPayload := map[string]interface{}{
		"title":          "Supreme Court Monthly Digest: January 2026",
		"court":          "Supreme Court of India",
		"court_code":     "SC",
		"digest_type":    "digest",
		"period":         "monthly",
		"period_start":   "2026-01-01",
		"period_end":     "2026-12-31",
		"citation_prefix": "2026 LiveLaw (SC)",
		"intro":          "A monthly roundup of landmark constitutional and commercial rulings.",
		"access_level":   "premium",
	}
	body, _ := json.Marshal(createPayload)

	req := httptest.NewRequest(http.MethodPost, "/v1/cms/digests", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var created digest.Digest
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == "" {
		t.Fatalf("expected digest ID to be populated")
	}

	// 2. Trigger Auto-Assemble
	assembleReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/digests/%s/auto-assemble", created.ID), nil)
	assembleReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	assembleRec := httptest.NewRecorder()
	env.router.ServeHTTP(assembleRec, assembleReq)

	if assembleRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for auto-assemble, got %d: %s", assembleRec.Code, assembleRec.Body.String())
	}

	var detail queries.DigestDetailView
	_ = json.Unmarshal(assembleRec.Body.Bytes(), &detail)

	if len(detail.Digest.Items) != 3 {
		t.Fatalf("expected 3 items auto-assembled, got %d", len(detail.Digest.Items))
	}

	// 3. Verify Nominal Index Alphabetical Ordering:
	// Alpha Finance Corp -> Beta Chemicals Ltd -> Zeta Motors Ltd
	nominal := detail.NominalIndex
	if len(nominal) != 3 {
		t.Fatalf("expected 3 nominal index entries, got %d", len(nominal))
	}
	if !strings.HasPrefix(nominal[0].CaseTitle, "Alpha") {
		t.Errorf("expected Alpha first in nominal index, got: %s", nominal[0].CaseTitle)
	}
	if !strings.HasPrefix(nominal[1].CaseTitle, "Beta") {
		t.Errorf("expected Beta second in nominal index, got: %s", nominal[1].CaseTitle)
	}
	if !strings.HasPrefix(nominal[2].CaseTitle, "Zeta") {
		t.Errorf("expected Zeta third in nominal index, got: %s", nominal[2].CaseTitle)
	}

	// 4. Verify Subject Index Grouping:
	// "Arbitration" (2 judgments) and "Insolvency and Bankruptcy" (1 judgment)
	subjects := detail.SubjectIndex
	if len(subjects) != 2 {
		t.Fatalf("expected 2 subject groups, got %d", len(subjects))
	}
	if subjects[0].Subject != "Arbitration" || len(subjects[0].Judgments) != 2 {
		t.Errorf("expected Arbitration group with 2 cases, got: %+v", subjects[0])
	}
	if subjects[1].Subject != "Insolvency and Bankruptcy" || len(subjects[1].Judgments) != 1 {
		t.Errorf("expected Insolvency group with 1 case, got: %+v", subjects[1])
	}

	// 5. Verify Statute Index:
	// Arbitration Act Sec 11, Sec 34; IBC Sec 53
	statutes := detail.StatuteIndex
	if len(statutes) != 3 {
		t.Fatalf("expected 3 statute sections in index, got %d", len(statutes))
	}
}

func TestDigest_CitationRangeBoundaryFiltering(t *testing.T) {
	env := setupDigestTestEnv(t)
	defer env.db.Close()

	seedTestJudgmentsForDigest(t, env)

	// Create digest strictly bounded to Citation 2 only (citation_range_start: 2, citation_range_end: 2)
	start := 2
	end := 2
	createPayload := map[string]interface{}{
		"title":                "Supreme Court Single Citation Focus",
		"court":                "Supreme Court of India",
		"court_code":           "SC",
		"period":               "weekly",
		"period_start":         "2026-01-01",
		"period_end":           "2026-12-31",
		"citation_range_start": start,
		"citation_range_end":   end,
	}
	body, _ := json.Marshal(createPayload)

	req := httptest.NewRequest(http.MethodPost, "/v1/cms/digests", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+env.editorToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	var created digest.Digest
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Auto-assemble
	assembleReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/digests/%s/auto-assemble", created.ID), nil)
	assembleReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	assembleRec := httptest.NewRecorder()
	env.router.ServeHTTP(assembleRec, assembleReq)

	var detail queries.DigestDetailView
	_ = json.Unmarshal(assembleRec.Body.Bytes(), &detail)

	if len(detail.Digest.Items) != 1 {
		t.Fatalf("expected exactly 1 item matching citation range [2, 2], got %d", len(detail.Digest.Items))
	}
	if detail.Digest.Items[0].Citation != "2026 LiveLaw (SC) 2" {
		t.Errorf("unexpected citation: %s", detail.Digest.Items[0].Citation)
	}
}

func TestDigest_SplitIntoParts(t *testing.T) {
	env := setupDigestTestEnv(t)
	defer env.db.Close()

	seedTestJudgmentsForDigest(t, env)

	// Create and auto-assemble 3 items
	d, err := env.digestCommands.Create(context.Background(), commands.CreateDigestInput{
		Title:       "Supreme Court Large Digest",
		Court:       "Supreme Court of India",
		CourtCode:   "SC",
		DigestType:  digest.DigestTypeComprehensive,
		Period:      digest.PeriodMonthly,
		PeriodStart: time.Now().AddDate(0, 0, -5),
		PeriodEnd:   time.Now().AddDate(0, 0, 5),
		CreatedBy:   env.editorUserID,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	assembled, err := env.digestCommands.AutoAssemble(context.Background(), d.ID)
	if err != nil || len(assembled.Items) != 3 {
		t.Fatalf("auto-assemble failed or items count != 3: %v", err)
	}

	// Split into 2 parts
	splitBody := map[string]int{"parts": 2}
	sBytes, _ := json.Marshal(splitBody)
	splitReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/digests/%s/split-parts", d.ID), bytes.NewReader(sBytes))
	splitReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	splitReq.Header.Set("Content-Type", "application/json")
	splitRec := httptest.NewRecorder()
	env.router.ServeHTTP(splitRec, splitReq)

	if splitRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for split-parts, got %d: %s", splitRec.Code, splitRec.Body.String())
	}

	var splitRes map[string]interface{}
	_ = json.Unmarshal(splitRec.Body.Bytes(), &splitRes)
	parts := splitRes["parts"].([]interface{})
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts returned, got %d", len(parts))
	}

	part1 := parts[0].(map[string]interface{})
	part2 := parts[1].(map[string]interface{})

	if !strings.Contains(part1["title"].(string), "Part 1 of 2") {
		t.Errorf("expected Part 1 of 2 in title, got: %v", part1["title"])
	}
	if !strings.Contains(part2["title"].(string), "Part 2 of 2") {
		t.Errorf("expected Part 2 of 2 in title, got: %v", part2["title"])
	}

	// Verify original digest is archived
	orig, _ := env.digestRepo.GetByID(context.Background(), d.ID)
	if orig.Status != digest.StatusArchived {
		t.Errorf("expected parent digest to be archived, got: %s", orig.Status)
	}
}

func TestDigest_PublishCreatesPremiumDigestArticle(t *testing.T) {
	env := setupDigestTestEnv(t)
	defer env.db.Close()

	seedTestJudgmentsForDigest(t, env)

	// Create and auto-assemble
	d, err := env.digestCommands.Create(context.Background(), commands.CreateDigestInput{
		Title:       "Supreme Court Landmark Digest 2026",
		Court:       "Supreme Court of India",
		CourtCode:   "SC",
		DigestType:  digest.DigestTypeComprehensive,
		Period:      digest.PeriodQuarterly,
		PeriodStart: time.Now().AddDate(0, 0, -10),
		PeriodEnd:   time.Now().AddDate(0, 0, 10),
		AccessLevel: digest.AccessLevelPremium,
		CreatedBy:   env.editorUserID,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	_, err = env.digestCommands.AutoAssemble(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("auto-assemble failed: %v", err)
	}

	// One-click Publish
	pubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/cms/digests/%s/publish", d.ID), nil)
	pubReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	pubRec := httptest.NewRecorder()
	env.router.ServeHTTP(pubRec, pubReq)

	if pubRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for publish, got %d: %s", pubRec.Code, pubRec.Body.String())
	}

	var pubRes map[string]interface{}
	_ = json.Unmarshal(pubRec.Body.Bytes(), &pubRes)

	artMap := pubRes["article"].(map[string]interface{})
	if artMap["article_type"] != "digest" {
		t.Errorf("expected article_type 'digest', got: %v", artMap["article_type"])
	}
	if artMap["access_level"] != "premium" {
		t.Errorf("expected premium access level, got: %v", artMap["access_level"])
	}

	// Verify public reader endpoint retrieves the published digest
	readReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/digests/%s", d.Slug), nil)
	readRec := httptest.NewRecorder()
	env.router.ServeHTTP(readRec, readReq)

	if readRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on public endpoint, got %d: %s", readRec.Code, readRec.Body.String())
	}
}

func TestDigest_GenerateMonthlyDrafts(t *testing.T) {
	env := setupDigestTestEnv(t)
	defer env.db.Close()

	// Trigger generate monthly drafts
	genReq := httptest.NewRequest(http.MethodPost, "/v1/cms/digests/generate-monthly", nil)
	genReq.Header.Set("Authorization", "Bearer "+env.editorToken)
	genRec := httptest.NewRecorder()
	env.router.ServeHTTP(genRec, genReq)

	if genRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for generate-monthly, got %d: %s", genRec.Code, genRec.Body.String())
	}

	var res map[string]interface{}
	_ = json.Unmarshal(genRec.Body.Bytes(), &res)
	count := int(res["count"].(float64))
	if count < 1 {
		t.Errorf("expected at least 1 monthly draft generated, got %d", count)
	}
}
