package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type TransactionRunner struct {
	pool *pgxpool.Pool
}

type MigrationState struct {
	Version int64
	Applied bool
}

// Schema compatibility changes with the migration set shipped by this binary.
const SupportedSchemaVersion int64 = 3

func CheckSchema(ctx context.Context, pool *pgxpool.Pool) error {
	state, err := MigrationStatus(ctx, pool)
	if err != nil {
		return fmt.Errorf("read migration state: %w", err)
	}
	if !state.Applied || state.Version != SupportedSchemaVersion {
		return fmt.Errorf("unsupported database schema version %d (expected %d applied)", state.Version, SupportedSchemaVersion)
	}
	return nil
}

func NewTransactionRunner(pool *pgxpool.Pool) TransactionRunner {
	return TransactionRunner{pool: pool}
}

func (runner TransactionRunner) Run(ctx context.Context, operation func(pgx.Tx) error) error {
	transaction, err := runner.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer transaction.Rollback(ctx)

	if err := operation(transaction); err != nil {
		return err
	}

	return transaction.Commit(ctx)
}

func InTransaction(ctx context.Context, pool *pgxpool.Pool, operation func(pgx.Tx) error) error {
	return NewTransactionRunner(pool).Run(ctx, operation)
}

func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	sqlDatabase := stdlib.OpenDBFromPool(pool)
	defer sqlDatabase.Close()

	return goose.UpContext(ctx, sqlDatabase, migrationDirectory())
}

func MigrationStatus(ctx context.Context, pool *pgxpool.Pool) (MigrationState, error) {
	var status MigrationState
	err := pool.QueryRow(ctx, `
		SELECT version_id, is_applied
		FROM goose_db_version
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&status.Version, &status.Applied)
	if errors.Is(err, pgx.ErrNoRows) {
		return MigrationState{}, nil
	}
	return status, err
}

func migrationDirectory() string {
	for _, candidate := range []string{"migrations", "../../../migrations"} {
		entries, err := os.ReadDir(candidate)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
				return candidate
			}
		}
	}

	return "migrations"
}
