package auth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestCreateUserRecordsBootstrapAuditWithoutActor(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createAuthTestDatabase(t, ctx, databaseURL)
	service := Service{Pool: pool}
	userID, err := service.CreateUser(ctx, "Bootstrap-Admin", "Checkpoint-Test-2026!")
	if err != nil {
		t.Fatalf("CreateUser returned an error: %v", err)
	}

	var actorID *string
	var targetID string
	if err := pool.QueryRow(ctx, "SELECT actor_id::text, target_id::text FROM audit_events WHERE event_type = 'admin.created'").Scan(&actorID, &targetID); err != nil {
		t.Fatalf("read administrator audit event: %v", err)
	}
	if actorID != nil {
		t.Fatalf("bootstrap audit actor was %q, expected null", *actorID)
	}
	if targetID != userID.String() {
		t.Fatalf("bootstrap audit target was %q, expected %q", targetID, userID)
	}
}

func createAuthTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
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
	databaseName := fmt.Sprintf("ause_auth_%d", time.Now().UnixNano())
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
