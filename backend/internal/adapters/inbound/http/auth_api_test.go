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
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/livelaw/backend/internal/adapters/outbound/hasher"
	"github.com/livelaw/backend/internal/adapters/outbound/idgen"
	"github.com/livelaw/backend/internal/adapters/outbound/postgres"
	"github.com/livelaw/backend/internal/adapters/outbound/token"
	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/identity"
	"github.com/livelaw/backend/migrations"
)

func setupTestRouter(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://livelaw:livelaw_secret@localhost:5432/livelaw?sslmode=disable"
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Skipf("skipping API test: failed to connect to database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping API test: database not reachable: %v", err)
	}

	// Ensure migrations are up
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
	pwHasher := hasher.NewBcryptHasher(4) // Low cost for fast tests
	tokenService := token.NewJWTService("test-secret-key-32-bytes-long-super-secret", "test", 15*time.Minute, 7*24*time.Hour, clock)

	// Seed the canonical users
	seeder := postgres.NewSeeder(userRepo, pwHasher, idGenerator, clock, logger)
	if err := seeder.SeedUsers(ctx); err != nil {
		t.Fatalf("failed seeding users: %v", err)
	}

	authUseCases := commands.NewAuthUseCases(userRepo, tokenRepo, pwHasher, tokenService, idGenerator, clock)
	userQueries := queries.NewUserQueries(userRepo)
	rateLimiter := NewIPRateLimiter(100, 1*time.Minute)

	router := NewRouter(RouterConfig{
		Logger:          logger,
		DBHealthChecker: dbHealthChecker,
		Clock:           clock,
		TokenService:    tokenService,
		AuthUseCases:    authUseCases,
		UserQueries:     userQueries,
		RateLimiter:     rateLimiter,
	})

	return router, db
}

func loginHelper(t *testing.T, router http.Handler, email, password string) AuthResponse {
	t.Helper()
	body, _ := json.Marshal(LoginRequest{Email: email, Password: password})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed for %s with status %d: %s", email, rec.Code, rec.Body.String())
	}

	var resp AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding login response: %v", err)
	}
	return resp
}

func TestAuth_SeededUsersLogin(t *testing.T) {
	router, _ := setupTestRouter(t)

	users := []struct {
		email    string
		password string
		role     identity.SystemRole
	}{
		{"reader@livelaw.in", "ReaderPass123!", identity.RoleReader},
		{"reporter@livelaw.in", "ReporterPass123!", identity.RoleReporter},
		{"editor@livelaw.in", "EditorPass123!", identity.RoleEditor},
		{"admin@livelaw.in", "AdminPass123!", identity.RoleAdmin},
	}

	for _, u := range users {
		t.Run(string(u.role), func(t *testing.T) {
			resp := loginHelper(t, router, u.email, u.password)
			if resp.User.Email != u.email {
				t.Errorf("expected email %s, got %s", u.email, resp.User.Email)
			}
			if resp.User.SystemRole != u.role {
				t.Errorf("expected role %s, got %s", u.role, resp.User.SystemRole)
			}
			if resp.AccessToken == "" || resp.RefreshToken == "" {
				t.Errorf("expected access and refresh tokens")
			}
		})
	}
}

func TestAuth_RegisterAndDuplicate(t *testing.T) {
	router, _ := setupTestRouter(t)

	uniqueEmail := fmt.Sprintf("newreader_%d@livelaw.in", time.Now().UnixNano())
	regBody, _ := json.Marshal(RegisterRequest{
		Email:       uniqueEmail,
		Password:    "ValidPassword123!",
		FullName:    "New Registered Reader",
		DisplayRole: "student",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(regBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var authResp AuthResponse
	_ = json.NewDecoder(rec.Body).Decode(&authResp)
	if authResp.User.Email != uniqueEmail {
		t.Errorf("expected email %s, got %s", uniqueEmail, authResp.User.Email)
	}
	if authResp.User.SystemRole != identity.RoleReader {
		t.Errorf("expected registered user to have reader role, got %s", authResp.User.SystemRole)
	}

	// Attempt duplicate registration
	dupReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(regBody))
	dupReq.Header.Set("Content-Type", "application/json")
	dupRec := httptest.NewRecorder()
	router.ServeHTTP(dupRec, dupReq)

	if dupRec.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for duplicate email, got %d", dupRec.Code)
	}
}

func TestAuth_RefreshTokenRotationAndRevocation(t *testing.T) {
	router, _ := setupTestRouter(t)

	// 1. Initial Login
	loginResp := loginHelper(t, router, "reader@livelaw.in", "ReaderPass123!")
	r1 := loginResp.RefreshToken

	// 2. Refresh with R1 -> returns R2
	refBody1, _ := json.Marshal(RefreshRequest{RefreshToken: r1})
	req1 := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewReader(refBody1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("first refresh failed with %d: %s", rec1.Code, rec1.Body.String())
	}

	var refResp1 AuthResponse
	_ = json.NewDecoder(rec1.Body).Decode(&refResp1)
	r2 := refResp1.RefreshToken

	if r2 == "" || r2 == r1 {
		t.Errorf("expected new rotated refresh token, got r2=%s", r2)
	}

	// 3. Attempting to use R1 again (must fail due to rotation)
	reqOld := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewReader(refBody1))
	reqOld.Header.Set("Content-Type", "application/json")
	recOld := httptest.NewRecorder()
	router.ServeHTTP(recOld, reqOld)

	if recOld.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for reused refresh token R1, got %d", recOld.Code)
	}

	// 4. Refresh with R2 -> returns R3
	refBody2, _ := json.Marshal(RefreshRequest{RefreshToken: r2})
	req2 := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewReader(refBody2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("second refresh failed with %d: %s", rec2.Code, rec2.Body.String())
	}
	var refResp2 AuthResponse
	_ = json.NewDecoder(rec2.Body).Decode(&refResp2)
	r3 := refResp2.RefreshToken

	// 5. Logout with R3
	logoutBody, _ := json.Marshal(LogoutRequest{RefreshToken: r3})
	reqLogout := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", bytes.NewReader(logoutBody))
	reqLogout.Header.Set("Content-Type", "application/json")
	recLogout := httptest.NewRecorder()
	router.ServeHTTP(recLogout, reqLogout)

	if recLogout.Code != http.StatusOK {
		t.Errorf("logout failed with status %d", recLogout.Code)
	}

	// 6. Attempting to refresh with revoked R3 (must fail)
	refBody3, _ := json.Marshal(RefreshRequest{RefreshToken: r3})
	reqRevoked := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewReader(refBody3))
	reqRevoked.Header.Set("Content-Type", "application/json")
	recRevoked := httptest.NewRecorder()
	router.ServeHTTP(recRevoked, reqRevoked)

	if recRevoked.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for revoked refresh token R3, got %d", recRevoked.Code)
	}
}

func TestRBAC_EndpointsAccessMatrix(t *testing.T) {
	router, _ := setupTestRouter(t)

	reader := loginHelper(t, router, "reader@livelaw.in", "ReaderPass123!")
	reporter := loginHelper(t, router, "reporter@livelaw.in", "ReporterPass123!")
	editor := loginHelper(t, router, "editor@livelaw.in", "EditorPass123!")
	admin := loginHelper(t, router, "admin@livelaw.in", "AdminPass123!")

	// 1. GET /v1/me endpoint
	t.Run("GET /v1/me", func(t *testing.T) {
		// Unauthenticated
		req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated /v1/me expected 401, got %d", rec.Code)
		}

		// Reader authenticated
		reqAuth := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
		reqAuth.Header.Set("Authorization", "Bearer "+reader.AccessToken)
		recAuth := httptest.NewRecorder()
		router.ServeHTTP(recAuth, reqAuth)
		if recAuth.Code != http.StatusOK {
			t.Errorf("reader /v1/me expected 200, got %d", recAuth.Code)
		}
	})

	// 2. POST /v1/cms/users (Create Staff)
	t.Run("POST /v1/cms/users role matrix", func(t *testing.T) {
		newStaffBody, _ := json.Marshal(CreateStaffRequest{
			Email:       fmt.Sprintf("court_reporter_%d@livelaw.in", time.Now().UnixNano()),
			Password:    "StaffReporterPass123!",
			FullName:    "Court Reporter Two",
			SystemRole:  "reporter",
			DisplayRole: "lawyer",
		})

		// Reader forbidden
		reqReader := httptest.NewRequest(http.MethodPost, "/v1/cms/users", bytes.NewReader(newStaffBody))
		reqReader.Header.Set("Authorization", "Bearer "+reader.AccessToken)
		recReader := httptest.NewRecorder()
		router.ServeHTTP(recReader, reqReader)
		if recReader.Code != http.StatusForbidden {
			t.Errorf("reader creating staff expected 403, got %d", recReader.Code)
		}

		// Reporter forbidden
		reqReporter := httptest.NewRequest(http.MethodPost, "/v1/cms/users", bytes.NewReader(newStaffBody))
		reqReporter.Header.Set("Authorization", "Bearer "+reporter.AccessToken)
		recReporter := httptest.NewRecorder()
		router.ServeHTTP(recReporter, reqReporter)
		if recReporter.Code != http.StatusForbidden {
			t.Errorf("reporter creating staff expected 403, got %d", recReporter.Code)
		}

		// Editor forbidden
		reqEditor := httptest.NewRequest(http.MethodPost, "/v1/cms/users", bytes.NewReader(newStaffBody))
		reqEditor.Header.Set("Authorization", "Bearer "+editor.AccessToken)
		recEditor := httptest.NewRecorder()
		router.ServeHTTP(recEditor, reqEditor)
		if recEditor.Code != http.StatusForbidden {
			t.Errorf("editor creating staff expected 403, got %d", recEditor.Code)
		}

		// Admin allowed
		reqAdmin := httptest.NewRequest(http.MethodPost, "/v1/cms/users", bytes.NewReader(newStaffBody))
		reqAdmin.Header.Set("Authorization", "Bearer "+admin.AccessToken)
		recAdmin := httptest.NewRecorder()
		router.ServeHTTP(recAdmin, reqAdmin)
		if recAdmin.Code != http.StatusCreated {
			t.Errorf("admin creating staff expected 201, got %d: %s", recAdmin.Code, recAdmin.Body.String())
		}
	})

	// 3. GET /v1/cms/users (List Users)
	t.Run("GET /v1/cms/users role matrix", func(t *testing.T) {
		tokens := []struct {
			role  string
			token string
			code  int
		}{
			{"reader", reader.AccessToken, http.StatusForbidden},
			{"reporter", reporter.AccessToken, http.StatusForbidden},
			{"editor", editor.AccessToken, http.StatusForbidden},
			{"admin", admin.AccessToken, http.StatusOK},
		}

		for _, tc := range tokens {
			req := httptest.NewRequest(http.MethodGet, "/v1/cms/users?limit=10", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tc.code {
				t.Errorf("%s listing users expected status %d, got %d", tc.role, tc.code, rec.Code)
			}
		}
	})
}

func TestAuth_SuspendedUserCannotLoginOrRefresh(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create a user
	uniqueEmail := fmt.Sprintf("suspend_me_%d@livelaw.in", time.Now().UnixNano())
	regBody, _ := json.Marshal(RegisterRequest{
		Email:    uniqueEmail,
		Password: "ValidPassword123!",
		FullName: "Soon Suspended",
	})
	reqReg := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(regBody))
	reqReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	router.ServeHTTP(recReg, reqReg)

	var authResp AuthResponse
	_ = json.NewDecoder(recReg.Body).Decode(&authResp)
	targetID := authResp.User.ID
	refreshTkn := authResp.RefreshToken

	// Admin suspends user
	admin := loginHelper(t, router, "admin@livelaw.in", "AdminPass123!")
	suspendStatus := "suspended"
	patchBody, _ := json.Marshal(UpdateUserRequest{Status: &suspendStatus})
	reqPatch := httptest.NewRequest(http.MethodPatch, "/v1/cms/users/"+targetID, bytes.NewReader(patchBody))
	reqPatch.Header.Set("Authorization", "Bearer "+admin.AccessToken)
	reqPatch.Header.Set("Content-Type", "application/json")
	recPatch := httptest.NewRecorder()
	router.ServeHTTP(recPatch, reqPatch)

	if recPatch.Code != http.StatusOK {
		t.Fatalf("suspending user failed with %d: %s", recPatch.Code, recPatch.Body.String())
	}

	// 1. Attempt login as suspended user
	loginBody, _ := json.Marshal(LoginRequest{Email: uniqueEmail, Password: "ValidPassword123!"})
	reqLogin := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	router.ServeHTTP(recLogin, reqLogin)

	if recLogin.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for suspended user login, got %d", recLogin.Code)
	}

	// 2. Attempt token refresh as suspended user
	refBody, _ := json.Marshal(RefreshRequest{RefreshToken: refreshTkn})
	reqRef := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewReader(refBody))
	reqRef.Header.Set("Content-Type", "application/json")
	recRef := httptest.NewRecorder()
	router.ServeHTTP(recRef, reqRef)

	if recRef.Code != http.StatusUnauthorized && recRef.Code != http.StatusForbidden {
		t.Errorf("expected 401 or 403 for suspended user refresh, got %d", recRef.Code)
	}
}
