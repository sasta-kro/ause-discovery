package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"ause-discovery.local/backend/generated"
	"ause-discovery.local/backend/internal/platform/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Event struct {
	ActorID    uuid.UUID
	EventType  string
	TargetType string
	TargetID   uuid.UUID
	Metadata   map[string]any
}

func Append(ctx context.Context, database generated.DBTX, event Event) error {
	if event.EventType == "" || event.TargetType == "" {
		return fmt.Errorf("audit event and target types are required")
	}
	if event.Metadata == nil {
		event.Metadata = map[string]any{}
	}
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	queries := generated.New(database)
	_, err = queries.CreateAuditEvent(ctx, generated.CreateAuditEventParams{
		ID:         identity.NewUUID(),
		ActorID:    identity.NullableUUID(event.ActorID),
		EventType:  event.EventType,
		TargetType: event.TargetType,
		TargetID:   identity.NullableUUID(event.TargetID),
		Metadata:   metadata,
	})
	return err
}

func AppendTx(ctx context.Context, transaction pgx.Tx, event Event) error {
	return Append(ctx, transaction, event)
}
