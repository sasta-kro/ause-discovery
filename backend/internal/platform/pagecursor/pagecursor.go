package pagecursor

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
)

var ErrInvalid = errors.New("page cursor is invalid")

const (
	// The bound must cover the worst legal sort key: a 300-character Person
	// display name of four-byte UTF-8 characters plus the JSON envelope stays
	// under 1700 base64url characters. 2048 leaves headroom without admitting
	// unbounded input.
	maximumEncodedLength = 2048
	version              = 1
)

// Cursor is an opaque keyset position: one sort value plus the row identity.
// The sort value carries a timestamp as RFC 3339 nano text or a plain text key.
type Cursor struct {
	SortValue string
	ID        uuid.UUID
}

// Encode renders a keyset position as an opaque base64url versioned payload.
func Encode(sortValue string, id uuid.UUID) (string, error) {
	payload, err := json.Marshal(struct {
		Version   int       `json:"v"`
		SortValue string    `json:"s"`
		ID        uuid.UUID `json:"id"`
	}{Version: version, SortValue: sortValue, ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// EncodeTime renders a timestamp keyset position.
func EncodeTime(sortTime time.Time, id uuid.UUID) (string, error) {
	return Encode(sortTime.UTC().Format(time.RFC3339Nano), id)
}

// Decode strictly validates an opaque cursor. Malformed, oversized, unknown-field,
// or trailing-data payloads return ErrInvalid without the raw contents in errors.
func Decode(value string) (Cursor, error) {
	if len(value) == 0 || len(value) > maximumEncodedLength {
		return Cursor{}, ErrInvalid
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Cursor{}, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	var payload struct {
		Version   int       `json:"v"`
		SortValue string    `json:"s"`
		ID        uuid.UUID `json:"id"`
	}
	if err := decoder.Decode(&payload); err != nil {
		return Cursor{}, ErrInvalid
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return Cursor{}, ErrInvalid
	}
	if payload.Version != version || payload.SortValue == "" || payload.ID == uuid.Nil {
		return Cursor{}, ErrInvalid
	}
	return Cursor{SortValue: payload.SortValue, ID: payload.ID}, nil
}
