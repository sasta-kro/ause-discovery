package audit

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"ause-discovery.local/backend/internal/platform/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestListOrdersFiltersAndPagesAuditEvents(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createAuditTestDatabase(t, ctx, databaseURL)
	service := Service{Pool: pool}

	actorOne := uuid.MustParse("018f0000-0000-7000-8000-0000000000a1")
	actorTwo := uuid.MustParse("018f0000-0000-7000-8000-0000000000a2")
	projectID := uuid.MustParse("018f0000-0000-7000-8000-0000000000b1")
	for index, actor := range []uuid.UUID{actorOne, actorTwo} {
		if _, err := pool.Exec(ctx, `INSERT INTO application_users (id, username, status) VALUES ($1, $2, 'active')`, identity.UUID(actor), fmt.Sprintf("audit-actor-%d", index)); err != nil {
			t.Fatalf("insert actor: %v", err)
		}
	}
	sharedTime := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	unauthenticated := insertAuditEvent(t, ctx, pool, 0x01, nil, "login.failure", "application_user", nil, `{}`, sharedTime)
	created := insertAuditEvent(t, ctx, pool, 0x02, &actorOne, "project.created", "project", &projectID, `{"count": 2, "source": "api"}`, sharedTime)
	updated := insertAuditEvent(t, ctx, pool, 0x03, &actorOne, "project.updated", "project", &projectID, `{"project_revision": 3}`, sharedTime.Add(-time.Second))
	person := insertAuditEvent(t, ctx, pool, 0x04, &actorTwo, "person.created", "person", nil, `{}`, sharedTime.Add(-2*time.Second))

	page, err := service.List(ctx, PageQuery{})
	if err != nil {
		t.Fatalf("List returned an error: %v", err)
	}
	if page.Limit != defaultPageLimit {
		t.Fatalf("default limit was %d, expected %d", page.Limit, defaultPageLimit)
	}
	assertPageOrder(t, page, created, unauthenticated, updated, person)
	if page.Items[0].ActorID == nil || *page.Items[0].ActorID != actorOne || page.Items[0].TargetID == nil || *page.Items[0].TargetID != projectID {
		t.Fatalf("actor event decoded to actor %v target %v", page.Items[0].ActorID, page.Items[0].TargetID)
	}
	if page.Items[0].Metadata["count"] != float64(2) || page.Items[0].Metadata["source"] != "api" {
		t.Fatalf("metadata decoded to %v", page.Items[0].Metadata)
	}
	if page.Items[1].ActorID != nil || page.Items[1].TargetID != nil {
		t.Fatalf("unauthenticated event kept actor %v or target %v", page.Items[1].ActorID, page.Items[1].TargetID)
	}
	if page.Items[1].Metadata == nil || len(page.Items[1].Metadata) != 0 {
		t.Fatalf("empty metadata decoded to %v, expected an empty object", page.Items[1].Metadata)
	}
	if page.Items[2].Metadata["project_revision"] != float64(3) {
		t.Fatalf("metadata decoded to %v", page.Items[2].Metadata)
	}

	actionPage, err := service.List(ctx, PageQuery{Action: "project.created"})
	if err != nil {
		t.Fatalf("List with action filter returned an error: %v", err)
	}
	assertPageOrder(t, actionPage, created)
	substringPage, err := service.List(ctx, PageQuery{Action: "project"})
	if err != nil {
		t.Fatalf("List with substring action returned an error: %v", err)
	}
	if len(substringPage.Items) != 0 || substringPage.NextCursor != nil {
		t.Fatalf("substring action matched %d events, expected exact matching only", len(substringPage.Items))
	}

	actorPage, err := service.List(ctx, PageQuery{ActorID: &actorOne})
	if err != nil {
		t.Fatalf("List with actor filter returned an error: %v", err)
	}
	assertPageOrder(t, actorPage, created, updated)

	firstPage, err := service.List(ctx, PageQuery{Limit: 2})
	if err != nil {
		t.Fatalf("first page returned an error: %v", err)
	}
	if firstPage.Limit != 2 || firstPage.NextCursor == nil {
		t.Fatalf("first page had limit %d and cursor %v", firstPage.Limit, firstPage.NextCursor)
	}
	assertPageOrder(t, firstPage, created, unauthenticated)
	secondPage, err := service.List(ctx, PageQuery{Limit: 2, Cursor: *firstPage.NextCursor})
	if err != nil {
		t.Fatalf("second page returned an error: %v", err)
	}
	if secondPage.NextCursor != nil {
		t.Fatalf("final page exposed cursor %v", *secondPage.NextCursor)
	}
	assertPageOrder(t, secondPage, updated, person)
	seen := map[uuid.UUID]bool{}
	for _, item := range append(append([]PageItem{}, firstPage.Items...), secondPage.Items...) {
		if seen[item.ID] {
			t.Fatalf("event %s appeared on both pages", item.ID)
		}
		seen[item.ID] = true
	}

	bounded, err := service.List(ctx, PageQuery{Limit: maximumPageLimit + 50})
	if err != nil {
		t.Fatalf("List with oversized limit returned an error: %v", err)
	}
	if bounded.Limit != maximumPageLimit {
		t.Fatalf("oversized limit normalized to %d, expected %d", bounded.Limit, maximumPageLimit)
	}

	afterFinal, err := service.List(ctx, PageQuery{Cursor: mustEncodeCursor(t, PageItem{ID: person, CreatedAt: sharedTime.Add(-2 * time.Second)})})
	if err != nil {
		t.Fatalf("List past the final event returned an error: %v", err)
	}
	if len(afterFinal.Items) != 0 || afterFinal.NextCursor != nil {
		t.Fatalf("cursor past the final event returned %d items with cursor %v", len(afterFinal.Items), afterFinal.NextCursor)
	}
}

func assertPageOrder(t *testing.T, page Page, expected ...uuid.UUID) {
	t.Helper()
	if len(page.Items) != len(expected) {
		t.Fatalf("page held %d items, expected %d", len(page.Items), len(expected))
	}
	for index, id := range expected {
		if page.Items[index].ID != id {
			t.Fatalf("item %d was %s, expected %s", index, page.Items[index].ID, id)
		}
	}
}

func mustEncodeCursor(t *testing.T, item PageItem) string {
	t.Helper()
	encoded, err := encodeCursor(item)
	if err != nil {
		t.Fatalf("encodeCursor returned an error: %v", err)
	}
	return encoded
}

func insertAuditEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, lastByte byte, actorID *uuid.UUID, eventType, targetType string, targetID *uuid.UUID, metadata string, createdAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.MustParse("018f0000-0000-7000-8000-000000000000")
	id[15] = lastByte
	var actorParam pgtype.UUID
	if actorID != nil {
		actorParam = identity.UUID(*actorID)
	}
	var targetParam pgtype.UUID
	if targetID != nil {
		targetParam = identity.UUID(*targetID)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO audit_events (id, actor_id, event_type, target_type, target_id, metadata, created_at) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)`,
		identity.UUID(id), actorParam, eventType, targetType, targetParam, metadata, pgtype.Timestamptz{Time: createdAt, Valid: true}); err != nil {
		t.Fatalf("insert audit event: %v", err)
	}
	return id
}

func createAuditTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	adminConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	adminConfig.Database = "postgres"
	adminConnection, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	databaseName := fmt.Sprintf("ause_audit_%d", time.Now().UnixNano())
	if _, err := adminConnection.Exec(ctx, "CREATE DATABASE "+databaseName); err != nil {
		adminConnection.Close(ctx)
		t.Fatalf("create temporary database: %v", err)
	}
	adminConnection.Close(ctx)

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse pool configuration: %v", err)
	}
	poolConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open temporary database: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		t.Fatalf("set migration dialect: %v", err)
	}
	sqlDatabase := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, sqlDatabase, "../../migrations"); err != nil {
		sqlDatabase.Close()
		pool.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	sqlDatabase.Close()

	t.Cleanup(func() {
		pool.Close()
		adminConfig.Database = "postgres"
		connection, connectErr := pgx.ConnectConfig(ctx, adminConfig)
		if connectErr != nil {
			return
		}
		defer connection.Close(ctx)
		_, _ = connection.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseName+" WITH (FORCE)")
	})
	return pool
}
