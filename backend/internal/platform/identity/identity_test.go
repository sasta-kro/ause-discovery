package identity

import (
	"testing"
	"time"
)

func TestNewUUIDv7(t *testing.T) {
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	value, err := NewUUIDv7(now)
	if err != nil {
		t.Fatal(err)
	}
	if value[6]>>4 != 7 || value[8]>>6 != 2 {
		t.Fatal("invalid UUIDv7 bits")
	}
	if Milliseconds(value) != uint64(now.UnixMilli()) {
		t.Fatal("invalid UUIDv7 time")
	}
}
