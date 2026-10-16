package http

import (
	"log/slog"
	"net/http"

	"github.com/livelaw/backend/internal/application/ports"
)

// RouterConfig holds dependencies required to construct the HTTP router.
type RouterConfig struct {
	Logger          *slog.Logger
	DBHealthChecker ports.DBHealthChecker
	Clock           ports.Clock
}

// NewRouter builds the HTTP handler with all middlewares applied.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	healthHandler := NewHealthHandler(cfg.DBHealthChecker, cfg.Clock)
	mux.Handle("GET /healthz", healthHandler)

	// Wrap mux with middlewares
	handler := RecoveryMiddleware(cfg.Logger)(mux)
	handler = LoggingMiddleware(cfg.Logger)(handler)
	handler = RequestIDMiddleware(handler)
	handler = CORSMiddleware(handler)

	return handler
}
