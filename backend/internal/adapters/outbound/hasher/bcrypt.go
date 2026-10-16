package hasher

import (
	"golang.org/x/crypto/bcrypt"
)

// BcryptHasher implements ports.PasswordHasher using bcrypt.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher creates a BcryptHasher with given cost (default 12 if <= 0).
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost <= 0 {
		cost = 12
	}
	return &BcryptHasher{cost: cost}
}

// Hash securely hashes a plain-text password.
func (b *BcryptHasher) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Compare checks whether a plain password matches a hashed password.
func (b *BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
