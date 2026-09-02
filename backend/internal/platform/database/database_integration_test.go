package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDatabaseContract(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	context := context.Background()
	pool := createEmptyDatabase(t, context, databaseURL)
	defer pool.Close()

	if err := ApplyMigrations(context, pool); err != nil {
		t.Fatalf("ApplyMigrations returned an error: %v", err)
	}
	if err := ApplyMigrations(context, pool); err != nil {
		t.Fatalf("second ApplyMigrations returned an error: %v", err)
	}
	migrationState, err := MigrationStatus(context, pool)
	if err != nil {
		t.Fatalf("MigrationStatus returned an error: %v", err)
	}
	if migrationState.Version != 1 || !migrationState.Applied {
		t.Fatalf("unexpected migration state: %+v", migrationState)
	}

	assertMigrationCreatedTables(t, context, pool)
	assertConstraints(t, context, pool)
	assertTransactionRollback(t, context, pool)
}

func createEmptyDatabase(t *testing.T, context context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()

	adminConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("pgx.ParseConfig returned an error: %v", err)
	}
	adminConfig.Database = "postgres"
	adminConnection, err := pgx.ConnectConfig(context, adminConfig)
	if err != nil {
		t.Fatalf("pgx.ConnectConfig returned an error: %v", err)
	}

	databaseName := fmt.Sprintf("ause_contract_%d", time.Now().UnixNano())
	if _, err := adminConnection.Exec(context, "CREATE DATABASE "+databaseName); err != nil {
		adminConnection.Close(context)
		t.Fatalf("create temporary database: %v", err)
	}
	adminConnection.Close(context)

	databaseConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.ParseConfig returned an error: %v", err)
	}
	databaseConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(context, databaseConfig)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig returned an error: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		adminConfig.Database = "postgres"
		adminConnection, err := pgx.ConnectConfig(context, adminConfig)
		if err != nil {
			return
		}
		defer adminConnection.Close(context)
		_, _ = adminConnection.Exec(context, "DROP DATABASE IF EXISTS "+databaseName+" WITH (FORCE)")
	})

	return pool
}

func assertMigrationCreatedTables(t *testing.T, context context.Context, pool *pgxpool.Pool) {
	t.Helper()

	for _, tableName := range []string{
		"programs",
		"projects",
		"project_search_sync",
		"import_batches",
		"audit_events",
	} {
		var relationName *string
		if err := pool.QueryRow(context, "SELECT to_regclass($1)", tableName).Scan(&relationName); err != nil {
			t.Fatalf("look up table %q: %v", tableName, err)
		}
		if relationName == nil {
			t.Fatalf("migration did not create table %q", tableName)
		}
	}
}

func assertConstraints(t *testing.T, context context.Context, pool *pgxpool.Pool) {
	t.Helper()

	const programID = "00000000-0000-0000-0000-000000000001"
	const firstProgramVersionID = "00000000-0000-0000-0000-000000000002"
	const secondProgramVersionID = "00000000-0000-0000-0000-000000000003"
	const openEndedProgramID = "00000000-0000-0000-0000-000000000009"
	const openEndedProgramVersionID = "00000000-0000-0000-0000-000000000010"
	const overlappingOpenEndedProgramVersionID = "00000000-0000-0000-0000-000000000011"

	if _, err := pool.Exec(context, "INSERT INTO programs (id, key) VALUES ($1, 'computer_science')", programID); err != nil {
		t.Fatalf("insert program: %v", err)
	}
	if _, err := pool.Exec(context, "INSERT INTO program_versions (id, program_id, label, valid_from_year, valid_to_year) VALUES ($1, $2, 'Computer Science', 2020, 2022)", firstProgramVersionID, programID); err != nil {
		t.Fatalf("insert program version: %v", err)
	}
	assertStatementFails(t, context, pool, "INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ($1, $2, 'Computer Science', 2022)", secondProgramVersionID, programID)
	if _, err := pool.Exec(context, "INSERT INTO programs (id, key) VALUES ($1, 'information_technology')", openEndedProgramID); err != nil {
		t.Fatalf("insert open-ended program: %v", err)
	}
	if _, err := pool.Exec(context, "INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ($1, $2, 'Information Technology', 2020)", openEndedProgramVersionID, openEndedProgramID); err != nil {
		t.Fatalf("insert open-ended program version: %v", err)
	}
	assertStatementFails(t, context, pool, "INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ($1, $2, 'Information Technology', 2030)", overlappingOpenEndedProgramVersionID, openEndedProgramID)
	assertStatementFails(t, context, pool, "INSERT INTO people (id, display_name, normalized_name, student_id) VALUES ('00000000-0000-0000-0000-000000000004', 'Invalid Student', 'invalid student', '123456')")
	assertStatementFails(t, context, pool, "INSERT INTO projects (id, status, published_at) VALUES ('00000000-0000-0000-0000-000000000005', 'published', now())")

	if _, err := pool.Exec(context, "INSERT INTO search_rebuild_operations (id, state, physical_index_name) VALUES ('00000000-0000-0000-0000-000000000006', 'pending', 'projects_a')"); err != nil {
		t.Fatalf("insert active rebuild: %v", err)
	}
	assertStatementFails(t, context, pool, "INSERT INTO search_rebuild_operations (id, state, physical_index_name) VALUES ('00000000-0000-0000-0000-000000000007', 'processing', 'projects_b')")
}

func assertTransactionRollback(t *testing.T, context context.Context, pool *pgxpool.Pool) {
	t.Helper()

	rollbackError := errors.New("rollback contract")
	err := NewTransactionRunner(pool).Run(context, func(transaction pgx.Tx) error {
		if _, err := transaction.Exec(context, "INSERT INTO application_users (id, username, status) VALUES ('00000000-0000-0000-0000-000000000008', 'rollback-user', 'active')"); err != nil {
			return err
		}
		return rollbackError
	})
	if !errors.Is(err, rollbackError) {
		t.Fatalf("transaction returned %v, expected rollback error", err)
	}

	var count int
	if err := pool.QueryRow(context, "SELECT count(*) FROM application_users WHERE username = 'rollback-user'").Scan(&count); err != nil {
		t.Fatalf("count rollback user: %v", err)
	}
	if count != 0 {
		t.Fatalf("transaction rollback persisted %d application user records", count)
	}
}

func assertStatementFails(t *testing.T, context context.Context, pool *pgxpool.Pool, statement string, arguments ...any) {
	t.Helper()

	if _, err := pool.Exec(context, statement, arguments...); err == nil {
		t.Fatalf("statement did not fail: %s", statement)
	}
}
