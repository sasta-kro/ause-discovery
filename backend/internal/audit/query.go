package audit

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"ause-discovery.local/backend/generated"
	"ause-discovery.local/backend/internal/platform/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCursor   = errors.New("audit cursor is invalid")
	ErrInvalidMetadata = errors.New("audit metadata cannot be decoded")
)

const (
	defaultPageLimit   = 20
	maximumPageLimit   = 100
	maximumCursorBytes = 1024
	cursorVersion      = 1
)

// Service reads append-only audit events without exposing mutation paths.
type Service struct {
	Pool *pgxpool.Pool
}

// PageQuery selects one page of audit events. Zero-value Action and ActorID apply no filter.
type PageQuery struct {
	Action  string
	ActorID *uuid.UUID
	Cursor  string
	Limit   int
}

// PageItem is one decoded audit event. ActorID and TargetID stay nil when the stored value is null.
type PageItem struct {
	ID         uuid.UUID
	ActorID    *uuid.UUID
	Action     string
	TargetType string
	TargetID   *uuid.UUID
	Metadata   map[string]any
	CreatedAt  time.Time
}

// Page is one bounded result page with the effective limit and the next keyset position.
type Page struct {
	Items      []PageItem
	Limit      int
	NextCursor *string
}

func (service Service) List(ctx context.Context, query PageQuery) (Page, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = defaultPageLimit
	}
	if limit > maximumPageLimit {
		limit = maximumPageLimit
	}
	var cursorID uuid.UUID
	var cursorCreatedAt time.Time
	if query.Cursor != "" {
		cursor, err := decodeCursor(query.Cursor)
		if err != nil {
			return Page{}, err
		}
		cursorID = cursor.ID
		cursorCreatedAt = cursor.CreatedAt
	}
	var actorID pgtype.UUID
	if query.ActorID != nil {
		actorID = identity.UUID(*query.ActorID)
	}
	rows, err := generated.New(service.Pool).ListAuditEvents(ctx, generated.ListAuditEventsParams{
		Limit:           int32(limit + 1),
		EventType:       pgtype.Text{String: query.Action, Valid: query.Action != ""},
		ActorID:         actorID,
		CursorCreatedAt: pgtype.Timestamptz{Time: cursorCreatedAt, Valid: query.Cursor != ""},
		CursorID:        identity.NullableUUID(cursorID),
	})
	if err != nil {
		return Page{}, err
	}
	items := make([]PageItem, 0, len(rows))
	for _, row := range rows {
		item, err := pageItem(row)
		if err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	page := Page{Items: items, Limit: limit}
	if len(items) > limit {
		page.Items = items[:limit]
		next, err := encodeCursor(page.Items[limit-1])
		if err != nil {
			return Page{}, err
		}
		page.NextCursor = &next
	}
	return page, nil
}

func pageItem(row generated.AuditEvent) (PageItem, error) {
	item := PageItem{
		ID:         identity.UUIDValue(row.ID),
		Action:     row.EventType,
		TargetType: row.TargetType,
		Metadata:   map[string]any{},
		CreatedAt:  row.CreatedAt.Time,
	}
	if row.ActorID.Valid {
		actorID := identity.UUIDValue(row.ActorID)
		item.ActorID = &actorID
	}
	if row.TargetID.Valid {
		targetID := identity.UUIDValue(row.TargetID)
		item.TargetID = &targetID
	}
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &item.Metadata); err != nil {
			return PageItem{}, ErrInvalidMetadata
		}
	}
	return item, nil
}

type auditCursor struct {
	Version   int       `json:"v"`
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

func encodeCursor(item PageItem) (string, error) {
	payload, err := json.Marshal(auditCursor{Version: cursorVersion, CreatedAt: item.CreatedAt.UTC(), ID: item.ID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(value string) (auditCursor, error) {
	if len(value) == 0 || len(value) > maximumCursorBytes {
		return auditCursor{}, ErrInvalidCursor
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return auditCursor{}, ErrInvalidCursor
	}
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	var payload auditCursor
	if err := decoder.Decode(&payload); err != nil {
		return auditCursor{}, ErrInvalidCursor
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return auditCursor{}, ErrInvalidCursor
	}
	if payload.Version != cursorVersion || payload.ID == uuid.Nil || payload.CreatedAt.IsZero() {
		return auditCursor{}, ErrInvalidCursor
	}
	return payload, nil
}
