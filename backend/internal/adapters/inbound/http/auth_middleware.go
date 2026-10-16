package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/identity"
)

const AuthClaimsKey contextKey = "auth_claims"

// GetAuthClaims retrieves the verified auth claims from context.
func GetAuthClaims(ctx context.Context) *ports.AuthClaims {
	if val, ok := ctx.Value(AuthClaimsKey).(*ports.AuthClaims); ok {
		return val
	}
	return nil
}

// AuthenticateMiddleware extracts and validates Bearer JWT tokens from the Authorization header.
func AuthenticateMiddleware(tokenService ports.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				reqID := GetRequestID(r.Context())
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid authorization header format", reqID)
				return
			}

			claims, err := tokenService.ValidateAccessToken(parts[1])
			if err != nil {
				reqID := GetRequestID(r.Context())
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired access token", reqID)
				return
			}

			ctx := context.WithValue(r.Context(), AuthClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth enforces that the request must have a verified user claim.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := GetAuthClaims(r.Context())
		if claims == nil {
			reqID := GetRequestID(r.Context())
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
			return
		}
		next(w, r)
	}
}

// RequireRoles enforces that the authenticated user possesses one of the allowed system roles.
func RequireRoles(roles ...identity.SystemRole) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims := GetAuthClaims(r.Context())
			if claims == nil {
				reqID := GetRequestID(r.Context())
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
				return
			}

			userRole := identity.SystemRole(claims.SystemRole)
			authorized := false
			for _, allowed := range roles {
				if userRole == allowed {
					authorized = true
					break
				}
			}

			if !authorized {
				reqID := GetRequestID(r.Context())
				writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient privileges for this resource", reqID)
				return
			}

			next(w, r)
		}
	}
}
