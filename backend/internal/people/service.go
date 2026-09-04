package people

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ause-discovery.local/backend/generated"
	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/platform/database"
	"ause-discovery.local/backend/internal/platform/identity"
	"ause-discovery.local/backend/internal/platform/pagecursor"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("person not found")
	ErrConflict = errors.New("revision conflict")
)

type Service struct{ Pool *pgxpool.Pool }
type Input struct {
	DisplayName string
	StudentID   *string
	StaffID     *string
}
type Person struct {
	ID             uuid.UUID
	DisplayName    string
	NormalizedName string
	StudentID      *string
	StaffID        *string
	Revision       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (service Service) Create(ctx context.Context, actorID uuid.UUID, input Input) (Person, error) {
	var result Person
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		var createErr error
		result, createErr = service.CreateInTransaction(ctx, transaction, actorID, input)
		return createErr
	})
	return result, err
}

func (service Service) CreateInTransaction(ctx context.Context, transaction pgx.Tx, actorID uuid.UUID, input Input) (Person, error) {
	normalized, studentID, staffID, err := validate(input)
	if err != nil {
		return Person{}, err
	}
	personID := uuid.Must(uuid.NewV7())
	record, err := generated.New(transaction).CreatePerson(ctx, generated.CreatePersonParams{ID: identity.UUID(personID), DisplayName: strings.TrimSpace(input.DisplayName), NormalizedName: normalized, StudentID: optionalText(studentID), StaffID: optionalText(staffID)})
	if err != nil {
		return Person{}, err
	}
	if err := audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "person.created", TargetType: "person", TargetID: personID}); err != nil {
		return Person{}, err
	}
	return fromRecord(record), nil
}

func (service Service) Update(ctx context.Context, actorID, personID uuid.UUID, expectedRevision int64, input Input) (Person, error) {
	normalized, studentID, staffID, err := validate(input)
	if err != nil {
		return Person{}, err
	}
	var result generated.Person
	err = database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		var updateErr error
		result, updateErr = generated.New(transaction).UpdatePerson(ctx, generated.UpdatePersonParams{ID: identity.UUID(personID), DisplayName: strings.TrimSpace(input.DisplayName), NormalizedName: normalized, StudentID: optionalText(studentID), StaffID: optionalText(staffID), Revision: expectedRevision})
		if errors.Is(updateErr, pgx.ErrNoRows) {
			return ErrConflict
		}
		if updateErr != nil {
			return updateErr
		}
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "person.updated", TargetType: "person", TargetID: personID})
	})
	return fromRecord(result), err
}

func (service Service) Get(ctx context.Context, personID uuid.UUID) (Person, error) {
	var record generated.Person
	err := service.Pool.QueryRow(ctx, `SELECT id, display_name, normalized_name, student_id, staff_id, revision, created_at, updated_at FROM people WHERE id = $1`, identity.UUID(personID)).Scan(&record.ID, &record.DisplayName, &record.NormalizedName, &record.StudentID, &record.StaffID, &record.Revision, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Person{}, ErrNotFound
	}
	return fromRecord(record), err
}

type ListPage struct {
	Items      []Person
	Limit      int
	NextCursor *string
}

// List returns one keyset-ordered People page. Ordering is display_name then id
// ascending, and the opaque cursor is the last returned row position.
func (service Service) List(ctx context.Context, query string, limit int, cursor string) (ListPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var position pagecursor.Cursor
	if cursor != "" {
		decoded, err := pagecursor.Decode(cursor)
		if err != nil {
			return ListPage{}, err
		}
		position = decoded
	}
	rows, err := service.Pool.Query(ctx, `SELECT id, display_name, normalized_name, student_id, staff_id, revision, created_at, updated_at FROM people
WHERE ($1 = '' OR normalized_name LIKE '%' || $1 || '%' OR student_id = $1 OR staff_id = $1)
  AND ($3::text IS NULL OR (display_name, id) > ($3::text, $4::uuid))
ORDER BY display_name, id LIMIT $2`, normalize(query), limit+1, position.SortValue, identity.NullableUUID(position.ID))
	if err != nil {
		return ListPage{}, err
	}
	defer rows.Close()
	items := []Person{}
	for rows.Next() {
		var record generated.Person
		if err := rows.Scan(&record.ID, &record.DisplayName, &record.NormalizedName, &record.StudentID, &record.StaffID, &record.Revision, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return ListPage{}, err
		}
		items = append(items, fromRecord(record))
	}
	if err := rows.Err(); err != nil {
		return ListPage{}, err
	}
	page := ListPage{Items: items, Limit: limit}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		next, err := pagecursor.Encode(last.DisplayName, last.ID)
		if err != nil {
			return ListPage{}, err
		}
		page.NextCursor = &next
	}
	return page, nil
}

func validate(input Input) (string, *string, *string, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		return "", nil, nil, errors.New("display name is required")
	}
	studentID := trimOptional(input.StudentID)
	staffID := trimOptional(input.StaffID)
	if studentID != nil && (len(*studentID) != 7 || !allDigits(*studentID)) {
		return "", nil, nil, errors.New("student id must contain exactly seven digits")
	}
	if staffID != nil {
		normalized := strings.ToLower(*staffID)
		staffID = &normalized
	}
	return normalize(displayName), studentID, staffID, nil
}
func normalize(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}
func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
func allDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
func optionalText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}
func fromRecord(record generated.Person) Person {
	result := Person{ID: identity.UUIDValue(record.ID), DisplayName: record.DisplayName, NormalizedName: record.NormalizedName, Revision: record.Revision, CreatedAt: record.CreatedAt.Time, UpdatedAt: record.UpdatedAt.Time}
	if record.StudentID.Valid {
		value := record.StudentID.String
		result.StudentID = &value
	}
	if record.StaffID.Valid {
		value := record.StaffID.String
		result.StaffID = &value
	}
	return result
}
func CurrentRevision(ctx context.Context, pool *pgxpool.Pool, personID uuid.UUID) (int64, error) {
	var revision int64
	err := pool.QueryRow(ctx, "SELECT revision FROM people WHERE id = $1", identity.UUID(personID)).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return revision, err
}
func IsUniqueViolation(err error) bool { return strings.Contains(fmt.Sprint(err), "duplicate key") }
