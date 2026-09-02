package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestSynchronizerSyncIsDeterministicAndRollsBackInvalidInput(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	context, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	databasePool, removeTestDatabase := createCatalogTestDatabase(t, context, databaseURL)
	t.Cleanup(removeTestDatabase)
	applyCatalogTestMigrations(t, context, databasePool)

	if err := databasePool.Ping(context); err != nil {
		t.Fatalf("ping database pool: %v", err)
	}

	source := integrationSource()
	synchronizer := Synchronizer{DatabasePool: databasePool}
	if err := synchronizer.Sync(context, source); err != nil {
		t.Fatalf("first synchronization: %v", err)
	}
	if err := synchronizer.Sync(context, source); err != nil {
		t.Fatalf("second synchronization: %v", err)
	}

	var programCount, courseProgramCount, taxonomyCount int
	if err := databasePool.QueryRow(context, `SELECT count(*) FROM programs WHERE id = $1`, source.Academic.Programs[0].ID).Scan(&programCount); err != nil {
		t.Fatalf("count programs: %v", err)
	}
	if err := databasePool.QueryRow(context, `SELECT count(*) FROM course_program_versions WHERE course_version_id = $1`, source.Academic.Courses[0].Versions[0].ID).Scan(&courseProgramCount); err != nil {
		t.Fatalf("count course program links: %v", err)
	}
	if err := databasePool.QueryRow(context, `SELECT count(*) FROM taxonomy_values WHERE id = $1`, source.Taxonomy.Values[0].ID).Scan(&taxonomyCount); err != nil {
		t.Fatalf("count taxonomy values: %v", err)
	}
	if programCount != 1 || courseProgramCount != 1 || taxonomyCount != 1 {
		t.Fatalf("repeated synchronization inserted duplicates: programs=%d links=%d taxonomy=%d", programCount, courseProgramCount, taxonomyCount)
	}

	correctedSource := source
	correctedSource.Academic.Courses[0].Versions[0].Label = "Corrected Integration Course"
	retiredAt := time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC)
	correctedSource.Taxonomy.Values[0].RetiredAt = &retiredAt
	if err := synchronizer.Sync(context, correctedSource); err != nil {
		t.Fatalf("corrected synchronization: %v", err)
	}
	var correctedCourseLabel string
	var taxonomyRetired bool
	if err := databasePool.QueryRow(context, `SELECT label FROM course_versions WHERE id = $1`, source.Academic.Courses[0].Versions[0].ID).Scan(&correctedCourseLabel); err != nil {
		t.Fatalf("read corrected course version: %v", err)
	}
	if err := databasePool.QueryRow(context, `SELECT retired_at IS NOT NULL FROM taxonomy_values WHERE id = $1`, source.Taxonomy.Values[0].ID).Scan(&taxonomyRetired); err != nil {
		t.Fatalf("read retired taxonomy value: %v", err)
	}
	if correctedCourseLabel != "Corrected Integration Course" || !taxonomyRetired {
		t.Fatalf("catalog corrections were not persisted: label=%q retired=%t", correctedCourseLabel, taxonomyRetired)
	}

	correctedSource.Taxonomy.Values[0].RetiredAt = nil
	if err := synchronizer.Sync(context, correctedSource); err != nil {
		t.Fatalf("synchronization without retirement field: %v", err)
	}
	if err := databasePool.QueryRow(context, `SELECT retired_at IS NOT NULL FROM taxonomy_values WHERE id = $1`, source.Taxonomy.Values[0].ID).Scan(&taxonomyRetired); err != nil {
		t.Fatalf("read taxonomy value after omitted retirement: %v", err)
	}
	if !taxonomyRetired {
		t.Fatal("omitted retirement cleared a retired taxonomy value")
	}

	invalidSource := integrationSource()
	invalidSource.Academic.Courses[0].Versions[0].ProgramVersionIDs = []string{"018f0000-0000-7000-8000-000000000099"}
	if err := synchronizer.Sync(context, invalidSource); err == nil {
		t.Fatal("invalid source synchronized successfully")
	}
	var invalidProgramCount int
	if err := databasePool.QueryRow(context, `SELECT count(*) FROM programs WHERE id = $1`, invalidSource.Academic.Programs[0].ID).Scan(&invalidProgramCount); err != nil {
		t.Fatalf("count invalid program: %v", err)
	}
	if invalidProgramCount != 0 {
		t.Fatalf("invalid source created %d program records before transaction validation", invalidProgramCount)
	}
}

func createCatalogTestDatabase(t *testing.T, testContext context.Context, databaseURL string) (*pgxpool.Pool, func()) {
	t.Helper()
	testPoolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	databaseName := "ause_catalog_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	maintenancePoolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse maintenance database URL: %v", err)
	}
	maintenancePoolConfig.ConnConfig.Database = "postgres"
	maintenancePool, err := pgxpool.NewWithConfig(testContext, maintenancePoolConfig)
	if err != nil {
		t.Fatalf("create maintenance database pool: %v", err)
	}
	if _, err := maintenancePool.Exec(testContext, fmt.Sprintf("CREATE DATABASE %s", databaseName)); err != nil {
		maintenancePool.Close()
		t.Fatalf("create catalog test database: %v", err)
	}

	testPoolConfig.ConnConfig.Database = databaseName
	testPool, err := pgxpool.NewWithConfig(testContext, testPoolConfig)
	if err != nil {
		maintenancePool.Close()
		t.Fatalf("create catalog test pool: %v", err)
	}

	return testPool, func() {
		testPool.Close()
		cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := maintenancePool.Exec(cleanupContext, fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", databaseName)); err != nil {
			t.Errorf("drop catalog test database: %v", err)
		}
		maintenancePool.Close()
	}
}

func applyCatalogTestMigrations(t *testing.T, testContext context.Context, databasePool *pgxpool.Pool) {
	t.Helper()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set migration dialect: %v", err)
	}
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve catalog integration test path")
	}
	sqlDatabase := stdlib.OpenDBFromPool(databasePool)
	defer sqlDatabase.Close()
	migrationDirectory := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../migrations"))
	if err := goose.UpContext(testContext, sqlDatabase, migrationDirectory); err != nil {
		t.Fatalf("apply catalog test migrations: %v", err)
	}
}

func integrationSource() Source {
	programID := uuid.NewString()
	programVersionID := uuid.NewString()
	courseID := uuid.NewString()
	courseVersionID := uuid.NewString()
	taxonomyID := uuid.NewString()

	return Source{
		Academic: AcademicCatalog{
			Programs: []Program{{
				ID:  programID,
				Key: "program_" + programID[0:8],
				Versions: []ProgramVersion{{
					ID:            programVersionID,
					Label:         "Integration Program",
					ValidFromYear: 2020,
				}},
			}},
			Courses: []Course{{
				ID:  courseID,
				Key: "course_" + courseID[0:8],
				Versions: []CourseVersion{{
					ID:                courseVersionID,
					Label:             "Integration Course",
					Code:              "INT499",
					ValidFromYear:     2020,
					ProgramVersionIDs: []string{programVersionID},
				}},
			}},
		},
		Taxonomy: TaxonomyCatalog{Values: []TaxonomyValue{{
			ID:        taxonomyID,
			Dimension: "category",
			Key:       "category_" + taxonomyID[0:8],
			Labels:    map[string]string{"en": "Integration Category"},
		}}},
	}
}
