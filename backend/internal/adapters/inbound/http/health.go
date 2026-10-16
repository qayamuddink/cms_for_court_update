package http

import (
	"context"
	"net/http"
	"time"

	"github.com/livelaw/backend/internal/application/ports"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	dbChecker ports.DBHealthChecker
	clock     ports.Clock
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(dbChecker ports.DBHealthChecker, clock ports.Clock) *HealthHandler {
	return &HealthHandler{
		dbChecker: dbChecker,
		clock:     clock,
	}
}

// ServeHTTP handles the GET /healthz request.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	status := "ok"
	dbStatus := "connected"
	httpStatus := http.StatusOK

	if h.dbChecker != nil {
		if err := h.dbChecker.Ping(ctx); err != nil {
			status = "degraded"
			dbStatus = "disconnected"
			httpStatus = http.StatusServiceUnavailable
		}
	} else {
		status = "degraded"
		dbStatus = "unconfigured"
		httpStatus = http.StatusServiceUnavailable
	}

	resp := ports.HealthStatus{
		Status:    status,
		Timestamp: h.clock.Now().Format(time.RFC3339),
		Services: map[string]string{
			"database": dbStatus,
		},
	}

	writeJSON(w, httpStatus, resp)
}
