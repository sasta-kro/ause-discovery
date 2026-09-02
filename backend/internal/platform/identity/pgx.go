package identity

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func NewUUID() pgtype.UUID {
	return UUID(uuid.Must(uuid.NewV7()))
}

func UUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}

func NullableUUID(value uuid.UUID) pgtype.UUID {
	if value == uuid.Nil {
		return pgtype.UUID{}
	}
	return UUID(value)
}

func UUIDValue(value pgtype.UUID) uuid.UUID {
	if !value.Valid {
		return uuid.Nil
	}
	return uuid.UUID(value.Bytes)
}
