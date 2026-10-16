package ports

import "context"

// HealthStatus represents the health check result.
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

// DBHealthChecker defines the interface for database connectivity check.
type DBHealthChecker interface {
	Ping(ctx context.Context) error
}
