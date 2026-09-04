package audit

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func encodeText(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func TestCursorRoundTripPreservesKeysetPosition(t *testing.T) {
	createdAt := time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC)
	item := PageItem{ID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), CreatedAt: createdAt}

	encoded, err := encodeCursor(item)
	if err != nil {
		t.Fatalf("encodeCursor returned an error: %v", err)
	}
	decoded, err := decodeCursor(encoded)
	if err != nil {
		t.Fatalf("decodeCursor returned an error: %v", err)
	}
	if decoded.ID != item.ID || !decoded.CreatedAt.Equal(createdAt) {
		t.Fatalf("cursor decoded to id %s time %s, expected id %s time %s", decoded.ID, decoded.CreatedAt, item.ID, createdAt)
	}
}

func TestDecodeCursorRejectsMalformedValues(t *testing.T) {
	validJSON := `{"v":1,"created_at":"2026-09-04T10:30:00Z","id":"018f0000-0000-7000-8000-000000000001"}`
	invalid := map[string]string{
		"empty":            "",
		"not base64":       "cursor!",
		"not json":         encodeText("not json"),
		"wrong version":    encodeText(`{"v":2,"created_at":"2026-09-04T10:30:00Z","id":"018f0000-0000-7000-8000-000000000001"}`),
		"missing id":       encodeText(`{"v":1,"created_at":"2026-09-04T10:30:00Z"}`),
		"nil id":           encodeText(`{"v":1,"created_at":"2026-09-04T10:30:00Z","id":"00000000-0000-0000-0000-000000000000"}`),
		"missing time":     encodeText(`{"v":1,"id":"018f0000-0000-7000-8000-000000000001"}`),
		"unknown field":    encodeText(validJSON[:len(validJSON)-1] + `,"extra":true}`),
		"trailing data":    encodeText(validJSON + " {}"),
		"oversized cursor": encodeText(`{"v":1,"created_at":"2026-09-04T10:30:00Z","id":"018f0000-0000-7000-8000-000000000001","padding":"` + padCursor(900) + `"}`),
	}
	for name, cursor := range invalid {
		if _, err := decodeCursor(cursor); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("%s cursor was accepted with error %v", name, err)
		}
	}
}

func padCursor(length int) string {
	value := make([]byte, length)
	for index := range value {
		value[index] = 'x'
	}
	return string(value)
}

func TestListRejectsMalformedCursorBeforeQuerying(t *testing.T) {
	service := Service{}
	_, err := service.List(context.Background(), PageQuery{Cursor: "not-base64!"})
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("malformed cursor returned error %v, expected ErrInvalidCursor", err)
	}
}
