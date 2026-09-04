package pagecursor

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	id := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	encoded, err := EncodeTime(time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC), id)
	if err != nil {
		t.Fatalf("EncodeTime returned an error: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode returned an error: %v", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, decoded.SortValue)
	if err != nil || !parsed.Equal(time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)) || decoded.ID != id {
		t.Fatalf("cursor decoded to %q %s", decoded.SortValue, decoded.ID)
	}

	textEncoded, err := Encode("display name", id)
	if err != nil {
		t.Fatalf("Encode returned an error: %v", err)
	}
	textDecoded, err := Decode(textEncoded)
	if err != nil || textDecoded.SortValue != "display name" || textDecoded.ID != id {
		t.Fatalf("text cursor decoded to %+v with error %v", textDecoded, err)
	}
}

func TestDecodeRejectsMalformedValues(t *testing.T) {
	valid := `{"v":1,"s":"2026-09-04T10:00:00Z","id":"018f0000-0000-7000-8000-000000000001"}`
	invalid := map[string]string{
		"empty":          "",
		"not base64":     "cursor!",
		"not json":       encodeText("not json"),
		"wrong version":  encodeText(`{"v":2,"s":"x","id":"018f0000-0000-7000-8000-000000000001"}`),
		"missing sort":   encodeText(`{"v":1,"id":"018f0000-0000-7000-8000-000000000001"}`),
		"nil id":         encodeText(`{"v":1,"s":"x","id":"00000000-0000-0000-0000-000000000000"}`),
		"unknown field":  encodeText(valid[:len(valid)-1] + `,"extra":1}`),
		"trailing data":  encodeText(valid + " {}"),
		"oversized":      encodeText(`{"v":1,"s":"` + pad(2000) + `","id":"018f0000-0000-7000-8000-000000000001"}`),
		"invalid id hex": encodeText(`{"v":1,"s":"x","id":"018f0000-0000-7000-8000-0000000000zz"}`),
	}
	for name, value := range invalid {
		if _, err := Decode(value); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s cursor was accepted with error %v", name, err)
		}
	}
}

func encodeText(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func pad(length int) string {
	value := make([]byte, length)
	for index := range value {
		value[index] = 'x'
	}
	return string(value)
}
