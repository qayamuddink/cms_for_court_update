package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
)

type mockDBChecker struct {
	err error
}

func (m *mockDBChecker) Ping(ctx context.Context) error {
	return m.err
}

type fixedClock struct {
	t time.Time
}

func (f fixedClock) Now() time.Time {
	return f.t
}

func TestHealthEndpoint_Healthy(t *testing.T) {
	fixedTime := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	router := NewRouter(RouterConfig{
		Logger:          logger,
		DBHealthChecker: &mockDBChecker{err: nil},
		Clock:           fixedClock{t: fixedTime},
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", rec.Code)
	}

	var resp ports.HealthStatus
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json body: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
	if resp.Services["database"] != "connected" {
		t.Errorf("expected database 'connected', got %q", resp.Services["database"])
	}
	if resp.Timestamp != fixedTime.Format(time.RFC3339) {
		t.Errorf("expected timestamp %q, got %q", fixedTime.Format(time.RFC3339), resp.Timestamp)
	}

	// Verify Request ID header exists
	reqID := rec.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Errorf("expected X-Request-ID header to be set")
	}
}

func TestHealthEndpoint_Degraded(t *testing.T) {
	fixedTime := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	router := NewRouter(RouterConfig{
		Logger:          logger,
		DBHealthChecker: &mockDBChecker{err: errors.New("connection refused")},
		Clock:           fixedClock{t: fixedTime},
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 Service Unavailable, got %d", rec.Code)
	}

	var resp ports.HealthStatus
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json body: %v", err)
	}

	if resp.Status != "degraded" {
		t.Errorf("expected status 'degraded', got %q", resp.Status)
	}
	if resp.Services["database"] != "disconnected" {
		t.Errorf("expected database 'disconnected', got %q", resp.Services["database"])
	}
}
