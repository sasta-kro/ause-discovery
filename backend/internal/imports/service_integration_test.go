package imports

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const secondValidCSVRow = `row-002,Second Imported Project,REF-002,Second imported abstract,2026,first,computing,,capstone,[],"[{""display_name"":""Second Student"",""student_id"":""1234567""}]","[{""display_name"":""Second Advisor"",""staff_id"":""advisor-2""}]",[],[],"[""ml""]","[""web""]",[],[],[]
`

const secondSharedAdvisorCSVRow = `row-002,Second Imported Project,REF-002,Second imported abstract,2026,first,computing,,capstone,[],"[{""display_name"":""Second Student"",""student_id"":""1234567""}]","[{""display_name"":""Shared Faculty""}]",[],[],"[""ml""]","[""web""]",[],[],[]
`

func TestPreviewSelectionCommitAndRepeatedResult(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createImportTestDatabase(t, ctx, databaseURL)
	actorID := seedImportCatalog(t, ctx, pool)
	temporaryRoot := t.TempDir()
	service := Service{Pool: pool, TemporaryRoot: temporaryRoot}

	batch, err := service.CreatePreview(ctx, actorID, "projects.csv", int64(len(validCSV)), strings.NewReader(validCSV))
	if err != nil {
		t.Fatalf("CreatePreview returned an error: %v", err)
	}
	if batch.State != "ready" || batch.TotalRows != 1 || batch.ValidRows != 1 || batch.WarningRows != 0 || batch.ErrorRows != 0 {
		t.Fatalf("unexpected preview batch: %#v", batch)
	}
	entries, err := os.ReadDir(temporaryRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary source cleanup left %d entries and error %v", len(entries), err)
	}

	restartedService := Service{Pool: pool, TemporaryRoot: temporaryRoot}
	rows, err := restartedService.ListRows(ctx, batch.ID, 20, 0)
	if err != nil || len(rows) != 1 || rows[0].State != "valid" {
		t.Fatalf("persisted preview rows were %#v with error %v", rows, err)
	}
	batch, err = restartedService.UpdateRows(ctx, actorID, batch.ID, batch.Revision, []RowUpdate{{RowNumber: 1, Selected: true}})
	if err != nil {
		t.Fatalf("UpdateRows returned an error: %v", err)
	}
	result, err := restartedService.Commit(ctx, actorID, batch.ID, batch.Revision)
	if err != nil {
		t.Fatalf("Commit returned an error: %v", err)
	}
	if len(result.CreatedProjectIDs) != 1 || len(result.SkippedRows) != 0 {
		t.Fatalf("unexpected commit result: %#v", result)
	}

	repeated, err := restartedService.Commit(ctx, actorID, batch.ID, batch.Revision)
	if err != nil || repeated.BatchID != result.BatchID || repeated.CommittedAt != result.CommittedAt || repeated.CreatedProjectIDs[0] != result.CreatedProjectIDs[0] {
		t.Fatalf("repeated commit result was %#v with error %v", repeated, err)
	}
	var projectStatus string
	var syncState string
	if err := pool.QueryRow(ctx, `SELECT projects.status, project_search_sync.state FROM projects JOIN project_search_sync ON project_search_sync.project_id=projects.id WHERE projects.id=$1`, result.CreatedProjectIDs[0]).Scan(&projectStatus, &syncState); err != nil {
		t.Fatalf("read imported Project: %v", err)
	}
	if projectStatus != "published" || syncState != "pending" {
		t.Fatalf("imported Project status was %q and search state was %q", projectStatus, syncState)
	}
}

func TestCommitReusesOneNameOnlyPersonAcrossRows(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createImportTestDatabase(t, ctx, databaseURL)
	actorID := seedImportCatalog(t, ctx, pool)
	service := Service{Pool: pool, TemporaryRoot: t.TempDir()}
	firstRow := strings.Replace(validCSV, `"[{""display_name"":""Advisor Name"",""staff_id"":""advisor-1""}]"`, `"[{""display_name"":""Shared Faculty""}]"`, 1)
	source := firstRow + secondSharedAdvisorCSVRow

	batch, err := service.CreatePreview(ctx, actorID, "projects.csv", int64(len(source)), strings.NewReader(source))
	if err != nil || batch.ValidRows != 2 || batch.WarningRows != 0 || batch.ErrorRows != 0 {
		t.Fatalf("CreatePreview returned batch %#v and error %v", batch, err)
	}
	batch, err = service.UpdateRows(ctx, actorID, batch.ID, batch.Revision, []RowUpdate{{RowNumber: 1, Selected: true}, {RowNumber: 2, Selected: true}})
	if err != nil {
		t.Fatalf("UpdateRows returned an error: %v", err)
	}
	result, err := service.Commit(ctx, actorID, batch.ID, batch.Revision)
	if err != nil || len(result.CreatedProjectIDs) != 2 {
		t.Fatalf("Commit returned result %#v and error %v", result, err)
	}

	var personCount, advisorCount, distinctAdvisorCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM people WHERE normalized_name='shared faculty'),
			(SELECT count(*) FROM project_participations WHERE role='advisor'),
			(SELECT count(DISTINCT person_id) FROM project_participations WHERE role='advisor')
	`).Scan(&personCount, &advisorCount, &distinctAdvisorCount); err != nil {
		t.Fatalf("read imported People: %v", err)
	}
	if personCount != 1 || advisorCount != 2 || distinctAdvisorCount != 1 {
		t.Fatalf("shared advisor produced people=%d participations=%d distinct_people=%d", personCount, advisorCount, distinctAdvisorCount)
	}
}

func TestPreviewRejectsAmbiguousNameOnlyPerson(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createImportTestDatabase(t, ctx, databaseURL)
	actorID := seedImportCatalog(t, ctx, pool)
	service := Service{Pool: pool, TemporaryRoot: t.TempDir()}
	if _, err := pool.Exec(ctx, `
		INSERT INTO people (id, display_name, normalized_name) VALUES
		('018f0000-0000-7000-8000-000000000811', 'Shared Faculty', 'shared faculty'),
		('018f0000-0000-7000-8000-000000000812', 'Shared Faculty', 'shared faculty')
	`); err != nil {
		t.Fatalf("seed ambiguous People: %v", err)
	}
	source := strings.Replace(validCSV, `"[{""display_name"":""Advisor Name"",""staff_id"":""advisor-1""}]"`, `"[{""display_name"":""Shared Faculty""}]"`, 1)

	batch, err := service.CreatePreview(ctx, actorID, "projects.csv", int64(len(source)), strings.NewReader(source))
	if err != nil || batch.ValidRows != 0 || batch.WarningRows != 0 || batch.ErrorRows != 1 {
		t.Fatalf("CreatePreview returned batch %#v and error %v", batch, err)
	}
	rows, err := service.ListRows(ctx, batch.ID, 20, 0)
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListRows returned rows %#v and error %v", rows, err)
	}
	for _, issue := range rows[0].Issues {
		if issue.Code == "ambiguous_person_match" && issue.Severity == "error" {
			return
		}
	}
	t.Fatalf("ambiguous Person issue was missing from %#v", rows[0].Issues)
}

func TestPreviewPersistsValidationErrorsAndRejectsSelection(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createImportTestDatabase(t, ctx, databaseURL)
	actorID := seedImportCatalog(t, ctx, pool)
	service := Service{Pool: pool, TemporaryRoot: t.TempDir()}
	invalidSource := strings.Replace(validCSV, `[""ai""]`, `[""unknown_category""]`, 1)

	batch, err := service.CreatePreview(ctx, actorID, "projects.csv", int64(len(invalidSource)), strings.NewReader(invalidSource))
	if err != nil {
		t.Fatalf("CreatePreview returned an error: %v", err)
	}
	rows, err := service.ListRows(ctx, batch.ID, 20, 0)
	if err != nil || batch.ErrorRows != 1 || len(rows) != 1 || rows[0].State != "error" {
		t.Fatalf("invalid preview was batch=%#v rows=%#v error=%v", batch, rows, err)
	}
	if _, err := service.UpdateRows(ctx, actorID, batch.ID, batch.Revision, []RowUpdate{{RowNumber: 1, Selected: true}}); err == nil {
		t.Fatal("expected invalid row selection to fail")
	}
}

func TestCommitRollsBackAllRowsWhenCatalogChangesAfterPreview(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createImportTestDatabase(t, ctx, databaseURL)
	actorID := seedImportCatalog(t, ctx, pool)
	service := Service{Pool: pool, TemporaryRoot: t.TempDir()}
	source := validCSV + secondValidCSVRow

	batch, err := service.CreatePreview(ctx, actorID, "projects.csv", int64(len(source)), strings.NewReader(source))
	if err != nil || batch.ValidRows != 2 {
		t.Fatalf("CreatePreview returned batch %#v and error %v", batch, err)
	}
	batch, err = service.UpdateRows(ctx, actorID, batch.ID, batch.Revision, []RowUpdate{{RowNumber: 1, Selected: true}, {RowNumber: 2, Selected: true}})
	if err != nil {
		t.Fatalf("UpdateRows returned an error: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE taxonomy_values SET retired_at=now() WHERE dimension='category' AND key='ml'"); err != nil {
		t.Fatalf("retire taxonomy value: %v", err)
	}
	if _, err := service.Commit(ctx, actorID, batch.ID, batch.Revision); err == nil {
		t.Fatal("expected commit-time catalog validation to fail")
	}
	var projectCount, personCount int
	if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM projects), (SELECT count(*) FROM people)").Scan(&projectCount, &personCount); err != nil {
		t.Fatalf("read aggregate counts: %v", err)
	}
	if projectCount != 0 || personCount != 0 {
		t.Fatalf("failed commit persisted %d Projects and %d People", projectCount, personCount)
	}
	stored, err := service.Get(ctx, batch.ID)
	if err != nil || stored.State != "ready" {
		t.Fatalf("failed commit left batch %#v with error %v", stored, err)
	}
}

func TestCleanupExpiredScrubsPersistedRowPayloads(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createImportTestDatabase(t, ctx, databaseURL)
	actorID := seedImportCatalog(t, ctx, pool)
	service := Service{Pool: pool, TemporaryRoot: t.TempDir()}

	batch, err := service.CreatePreview(ctx, actorID, "projects.csv", int64(len(validCSV)), strings.NewReader(validCSV))
	if err != nil {
		t.Fatalf("CreatePreview returned an error: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE import_batches SET expires_at=$2 WHERE id=$1", batch.ID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("expire import batch: %v", err)
	}
	cleaned, err := service.CleanupExpired(ctx, 100)
	if err != nil || cleaned != 1 {
		t.Fatalf("CleanupExpired cleaned %d batches with error %v", cleaned, err)
	}
	stored, err := service.Get(ctx, batch.ID)
	if err != nil || stored.State != "expired" {
		t.Fatalf("expired batch was %#v with error %v", stored, err)
	}
	var draft, issues string
	var selected, acknowledged bool
	var resolution []byte
	if err := pool.QueryRow(ctx, "SELECT draft::text,issues::text,selected,warnings_acknowledged,duplicate_resolution FROM import_rows WHERE batch_id=$1", batch.ID).Scan(&draft, &issues, &selected, &acknowledged, &resolution); err != nil {
		t.Fatalf("read scrubbed import row: %v", err)
	}
	if draft != "{}" || issues != "[]" || selected || acknowledged || len(resolution) != 0 {
		t.Fatalf("expired row retained draft=%q issues=%q selected=%t acknowledged=%t resolution=%q", draft, issues, selected, acknowledged, resolution)
	}
}

func createImportTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	adminConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	adminConfig.Database = "postgres"
	databaseName := "ause_import_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	adminConnection, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("connect admin database: %v", err)
	}
	if _, err := adminConnection.Exec(ctx, "CREATE DATABASE "+databaseName); err != nil {
		adminConnection.Close(ctx)
		t.Fatalf("create test database: %v", err)
	}
	adminConnection.Close(ctx)
	testConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	testConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
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
		connection, connectErr := pgx.ConnectConfig(ctx, adminConfig)
		if connectErr != nil {
			return
		}
		defer connection.Close(ctx)
		_, _ = connection.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseName+" WITH (FORCE)")
	})
	return pool
}

func seedImportCatalog(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	actorID := uuid.MustParse("018f0000-0000-7000-8000-000000000801")
	statements := []string{
		"INSERT INTO application_users (id, username, status) VALUES ('018f0000-0000-7000-8000-000000000801', 'import-test-admin', 'active')",
		"INSERT INTO programs (id, key) VALUES ('018f0000-0000-7000-8000-000000000802', 'computing')",
		"INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000803', '018f0000-0000-7000-8000-000000000802', 'Computing', 2020)",
		"INSERT INTO courses (id, key) VALUES ('018f0000-0000-7000-8000-000000000804', 'capstone')",
		"INSERT INTO course_versions (id, course_id, label, code, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000805', '018f0000-0000-7000-8000-000000000804', 'Capstone', 'CAP499', 2020)",
		"INSERT INTO course_program_versions (course_version_id, program_version_id) VALUES ('018f0000-0000-7000-8000-000000000805', '018f0000-0000-7000-8000-000000000803')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels) VALUES ('018f0000-0000-7000-8000-000000000806', 'category', 'ai', '{\"en\":\"AI\"}')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels) VALUES ('018f0000-0000-7000-8000-000000000807', 'platform', 'web', '{\"en\":\"Web\"}')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels) VALUES ('018f0000-0000-7000-8000-000000000808', 'category', 'ml', '{\"en\":\"ML\"}')",
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed import catalog: %v", err)
		}
	}
	return actorID
}
