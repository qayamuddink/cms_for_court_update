package idgen

import "github.com/google/uuid"

// UUIDv7Generator implements ports.IDGenerator using RFC 9562 UUIDv7.
type UUIDv7Generator struct{}

// NewUUIDv7Generator creates a new UUIDv7Generator.
func NewUUIDv7Generator() *UUIDv7Generator {
	return &UUIDv7Generator{}
}

// NewID returns a new time-sortable UUIDv7 string.
func (g *UUIDv7Generator) NewID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// Fallback to NewString if NewV7 returns error
		return uuid.NewString()
	}
	return id.String()
}
