package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/identity"
)

// JWTService implements ports.TokenService using HMAC-SHA256.
type JWTService struct {
	secretKey          []byte
	issuer             string
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
	clock              ports.Clock
}

// NewJWTService creates a new JWTService.
func NewJWTService(secret string, issuer string, accessTTL, refreshTTL time.Duration, clock ports.Clock) *JWTService {
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	if refreshTTL <= 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	if clock == nil {
		clock = ports.RealClock{}
	}
	return &JWTService{
		secretKey:          []byte(secret),
		issuer:             issuer,
		accessTokenExpiry:  accessTTL,
		refreshTokenExpiry: refreshTTL,
		clock:              clock,
	}
}

type jwtCustomClaims struct {
	UserID      string                 `json:"user_id"`
	Email       string                 `json:"email"`
	SystemRole  string                 `json:"system_role"`
	Entitlement ports.EntitlementClaim `json:"entitlement"`
	jwt.RegisteredClaims
}

// GenerateAccessToken generates a short-lived JWT access token carrying user identity, system role, and entitlement claim.
func (j *JWTService) GenerateAccessToken(user *identity.User, entitlement ports.EntitlementClaim) (string, time.Time, error) {
	now := j.clock.Now()
	expiresAt := now.Add(j.accessTokenExpiry)

	claims := jwtCustomClaims{
		UserID:      user.ID,
		Email:       user.Email,
		SystemRole:  string(user.SystemRole),
		Entitlement: entitlement,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.secretKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign access token: %w", err)
	}

	return signed, expiresAt, nil
}

// GenerateRefreshToken generates a cryptographically secure random token, its SHA-256 hash, and expiration timestamp.
func (j *JWTService) GenerateRefreshToken() (string, string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to read random bytes for refresh token: %w", err)
	}

	rawToken := hex.EncodeToString(b)
	hash := j.HashRefreshToken(rawToken)
	expiresAt := j.clock.Now().Add(j.refreshTokenExpiry)

	return rawToken, hash, expiresAt, nil
}

// HashRefreshToken calculates the SHA-256 hash of a raw refresh token.
func (j *JWTService) HashRefreshToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// ValidateAccessToken parses and verifies a JWT access token.
func (j *JWTService) ValidateAccessToken(tokenString string) (*ports.AuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwtCustomClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*jwtCustomClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return &ports.AuthClaims{
		UserID:      claims.UserID,
		Email:       claims.Email,
		SystemRole:  claims.SystemRole,
		Entitlement: claims.Entitlement,
	}, nil
}
