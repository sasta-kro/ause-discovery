package people

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"ause-discovery.local/backend/internal/platform/pagecursor"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestPersonListFiltersAndPagesWithCursor(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createPeopleTestDatabase(t, ctx, databaseURL)
	service := Service{Pool: pool}
	insertPerson(t, ctx, pool, "018f0000-0000-7000-8000-000000000401", "Alice Example", "alice example", "7000001", "")
	insertPerson(t, ctx, pool, "018f0000-0000-7000-8000-000000000402", "Alice Example", "alice example", "", "")
	insertPerson(t, ctx, pool, "018f0000-0000-7000-8000-000000000403", "Bob Example", "bob example", "", "advisor-1")
	insertPerson(t, ctx, pool, "018f0000-0000-7000-8000-000000000404", "Carol Standalone", "carol standalone", "", "")

	first, err := service.List(ctx, "", 2, "")
	if err != nil {
		t.Fatalf("first page returned an error: %v", err)
	}
	if len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("first page held %d items with cursor %v", len(first.Items), first.NextCursor)
	}
	assertPeopleOrder(t, first.Items, "018f0000-0000-7000-8000-000000000401", "018f0000-0000-7000-8000-000000000402")
	second, err := service.List(ctx, "", 2, *first.NextCursor)
	if err != nil {
		t.Fatalf("second page returned an error: %v", err)
	}
	if second.NextCursor != nil {
		t.Fatalf("final page exposed cursor %v", *second.NextCursor)
	}
	assertPeopleOrder(t, second.Items, "018f0000-0000-7000-8000-000000000403", "018f0000-0000-7000-8000-000000000404")
	seen := map[uuid.UUID]bool{}
	for _, item := range append(append([]Person{}, first.Items...), second.Items...) {
		if seen[item.ID] {
			t.Fatalf("Person %s appeared on both pages", item.ID)
		}
		seen[item.ID] = true
	}

	nameQuery, err := service.List(ctx, "example", 20, "")
	if err != nil {
		t.Fatalf("name query returned an error: %v", err)
	}
	assertPeopleOrder(t, nameQuery.Items, "018f0000-0000-7000-8000-000000000401", "018f0000-0000-7000-8000-000000000402", "018f0000-0000-7000-8000-000000000403")
	studentQuery, err := service.List(ctx, "7000001", 20, "")
	if err != nil {
		t.Fatalf("student query returned an error: %v", err)
	}
	assertPeopleOrder(t, studentQuery.Items, "018f0000-0000-7000-8000-000000000401")
	staffQuery, err := service.List(ctx, "advisor-1", 20, "")
	if err != nil {
		t.Fatalf("staff query returned an error: %v", err)
	}
	assertPeopleOrder(t, staffQuery.Items, "018f0000-0000-7000-8000-000000000403")

	if _, err := service.List(ctx, "", 2, "not-a-cursor"); !errors.Is(err, pagecursor.ErrInvalid) {
		t.Fatalf("malformed cursor returned error %v", err)
	}
}

func insertPerson(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, displayName, normalizedName, studentID, staffID string) {
	t.Helper()
	var studentArg, staffArg *string
	if studentID != "" {
		studentArg = &studentID
	}
	if staffID != "" {
		staffArg = &staffID
	}
	if _, err := pool.Exec(ctx, `INSERT INTO people (id, display_name, normalized_name, student_id, staff_id) VALUES ($1, $2, $3, $4, $5)`, id, displayName, normalizedName, studentArg, staffArg); err != nil {
		t.Fatalf("insert Person: %v", err)
	}
}

func assertPeopleOrder(t *testing.T, items []Person, expected ...string) {
	t.Helper()
	if len(items) != len(expected) {
		t.Fatalf("list held %d items, expected %d", len(items), len(expected))
	}
	for index, id := range expected {
		if items[index].ID.String() != id {
			t.Fatalf("item %d was %s, expected %s", index, items[index].ID, id)
		}
	}
}

func createPeopleTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
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
	databaseName := fmt.Sprintf("ause_people_%d", time.Now().UnixNano())
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

func TestPersonListCursorHandlesMaximumSortKeys(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	cases := []struct {
		name                string
		firstName           string
		followingName       string
		firstID             string
		followingID         string
		minimumCursorLength int
	}{
		{name: "multibyte name", firstName: strings.Repeat("ก", 300), followingName: strings.Repeat("ก", 299) + "ข", firstID: "018f0000-0000-7000-8000-000000000501", followingID: "018f0000-0000-7000-8000-000000000502", minimumCursorLength: 1024},
		{name: "escaped name", firstName: strings.Repeat("&", 300), followingName: strings.Repeat("&", 299) + "0", firstID: "018f0000-0000-7000-8000-000000000511", followingID: "018f0000-0000-7000-8000-000000000512", minimumCursorLength: 2048},
	}
	for _, testCase := range cases {
		pool := createPeopleTestDatabase(t, ctx, databaseURL)
		service := Service{Pool: pool}
		insertPerson(t, ctx, pool, testCase.firstID, testCase.firstName, testCase.firstName, "", "")
		insertPerson(t, ctx, pool, testCase.followingID, testCase.followingName, testCase.followingName, "", "")

		first, err := service.List(ctx, "", 1, "")
		if err != nil {
			t.Fatalf("%s: first page returned an error: %v", testCase.name, err)
		}
		if len(first.Items) != 1 || first.NextCursor == nil {
			t.Fatalf("%s: first page held %d items with cursor %v", testCase.name, len(first.Items), first.NextCursor)
		}
		assertPeopleOrder(t, first.Items, testCase.firstID)
		cursorLength := len(*first.NextCursor)
		if cursorLength <= testCase.minimumCursorLength || cursorLength > 4096 {
			t.Fatalf("%s: cursor length was %d, expected it to exceed %d and stay within the 4096 contract bound", testCase.name, cursorLength, testCase.minimumCursorLength)
		}

		second, err := service.List(ctx, "", 1, *first.NextCursor)
		if err != nil {
			t.Fatalf("%s: second page returned an error: %v", testCase.name, err)
		}
		if second.NextCursor != nil {
			t.Fatalf("%s: final page exposed cursor %v", testCase.name, *second.NextCursor)
		}
		assertPeopleOrder(t, second.Items, testCase.followingID)
	}
}
