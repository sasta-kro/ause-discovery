package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"ause-discovery.local/backend/internal/auth"
	"ause-discovery.local/backend/internal/catalog"
	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/platform/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) >= 2 && arguments[0] == "admin" {
		return runAdmin(arguments[1:])
	}
	if len(arguments) != 2 {
		return usageError()
	}
	if arguments[0] == "migrations" && arguments[1] == "status" {
		return runMigrationStatus()
	}
	if arguments[0] != "catalog" || (arguments[1] != "validate" && arguments[1] != "sync") {
		return usageError()
	}

	configuration, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	source, err := catalog.Load(configuration.AcademicCatalogPath(), configuration.TaxonomyCatalogPath())
	if err != nil {
		return err
	}
	if arguments[1] == "validate" {
		fmt.Fprintln(os.Stdout, "catalog validation succeeded")
		return nil
	}

	operationContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	databasePool, err := pgxpool.New(operationContext, configuration.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer databasePool.Close()
	if err := databasePool.Ping(operationContext); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	if err := (catalog.Synchronizer{DatabasePool: databasePool}).Sync(operationContext, source); err != nil {
		return fmt.Errorf("synchronize catalog: %w", err)
	}
	fmt.Fprintln(os.Stdout, "catalog synchronization succeeded")
	return nil
}

func runAdmin(arguments []string) error {
	if len(arguments) == 1 && arguments[0] == "create" {
		username, err := prompt("Username: ")
		if err != nil {
			return err
		}
		password, err := promptPassword()
		if err != nil {
			return err
		}
		return withAuthService(func(service auth.Service) error {
			_, err := service.CreateUser(context.Background(), username, password)
			return err
		})
	}
	if len(arguments) == 3 && arguments[0] == "reset-password" && arguments[1] == "--username" {
		password, err := promptPassword()
		if err != nil {
			return err
		}
		return withAuthService(func(service auth.Service) error {
			return service.ResetPassword(context.Background(), arguments[2], password)
		})
	}
	if len(arguments) == 3 && arguments[0] == "disable" && arguments[1] == "--username" {
		return withAuthService(func(service auth.Service) error { return service.DisableUser(context.Background(), arguments[2]) })
	}
	return usageError()
}

func withAuthService(operation func(auth.Service) error) error {
	configuration, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	operationContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	databasePool, err := pgxpool.New(operationContext, configuration.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer databasePool.Close()
	if err := databasePool.Ping(operationContext); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return operation(auth.Service{Pool: databasePool, SessionIdleTTL: configuration.SessionIdleTTL, SessionAbsoluteTTL: configuration.SessionAbsoluteTTL})
}

func prompt(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	var value string
	if _, err := fmt.Fscanln(os.Stdin, &value); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func promptPassword() (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("password prompt requires a terminal")
	}
	fmt.Fprint(os.Stderr, "Password: ")
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func runMigrationStatus() error {
	configuration, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	operationContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	databasePool, err := pgxpool.New(operationContext, configuration.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer databasePool.Close()

	status, err := database.MigrationStatus(operationContext, databasePool)
	if err != nil {
		return fmt.Errorf("read migration status: %w", err)
	}
	fmt.Fprintf(os.Stdout, "migration version: %d\n", status.Version)
	fmt.Fprintf(os.Stdout, "migration applied: %t\n", status.Applied)
	return nil
}

func usageError() error {
	return errors.New("usage: ausectl admin create | admin reset-password --username <value> | admin disable --username <value> | catalog validate|sync | migrations status")
}
