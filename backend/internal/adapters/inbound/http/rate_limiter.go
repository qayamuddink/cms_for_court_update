package http

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// IPRateLimiter limits requests per client IP address.
type IPRateLimiter struct {
	mu      sync.Mutex
	limits  map[string][]time.Time
	rate    int
	window  time.Duration
	cleanup time.Duration
}

// NewIPRateLimiter creates an IPRateLimiter allowing rate requests per window.
func NewIPRateLimiter(rate int, window time.Duration) *IPRateLimiter {
	limiter := &IPRateLimiter{
		limits:  make(map[string][]time.Time),
		rate:    rate,
		window:  window,
		cleanup: window * 2,
	}

	go limiter.cleanupLoop()

	return limiter
}

func (l *IPRateLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	timestamps := l.limits[ip]

	// Filter timestamps within current window
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.rate {
		l.limits[ip] = valid
		return false
	}

	l.limits[ip] = append(valid, now)
	return true
}

func (l *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cleanup)
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-l.window)
		for ip, ts := range l.limits {
			valid := ts[:0]
			for _, t := range ts {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(l.limits, ip)
			} else {
				l.limits[ip] = valid
			}
		}
		l.mu.Unlock()
	}
}

func extractIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// Limit returns a middleware that limits requests by client IP.
func (l *IPRateLimiter) Limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		now := time.Now()

		if !l.allow(ip, now) {
			reqID := GetRequestID(r.Context())
			w.Header().Set("Retry-After", "60")
			writeJSONError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests, please try again later", reqID)
			return
		}

		next(w, r)
	}
}
