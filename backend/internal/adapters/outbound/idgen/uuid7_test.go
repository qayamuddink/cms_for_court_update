package idgen

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUUIDv7Generator(t *testing.T) {
	gen := NewUUIDv7Generator()
	id1 := gen.NewID()
	time.Sleep(2 * time.Millisecond)
	id2 := gen.NewID()

	if id1 == "" || id2 == "" {
		t.Fatalf("expected non-empty IDs, got id1=%q, id2=%q", id1, id2)
	}

	u1, err := uuid.Parse(id1)
	if err != nil {
		t.Fatalf("id1 is not a valid UUID: %v", err)
	}
	if u1.Version() != 7 {
		t.Errorf("expected UUID version 7, got %d", u1.Version())
	}

	u2, err := uuid.Parse(id2)
	if err != nil {
		t.Fatalf("id2 is not a valid UUID: %v", err)
	}
	if u2.Version() != 7 {
		t.Errorf("expected UUID version 7, got %d", u2.Version())
	}

	// Verify time ordering
	if id1 >= id2 {
		t.Errorf("expected id1 < id2 for time-sortable UUIDv7, got id1=%s, id2=%s", id1, id2)
	}
}
