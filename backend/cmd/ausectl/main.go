package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"ause-discovery.local/backend/internal/artifactimport"
	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/auth"
	"ause-discovery.local/backend/internal/catalog"
	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/platform/database"
	searchservice "ause-discovery.local/backend/internal/search"
	"github.com/google/uuid"
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
	if len(arguments) >= 2 && arguments[0] == "search" {
		return runSearch(arguments[1:])
	}
	if len(arguments) >= 2 && arguments[0] == "artifacts" {
		return runArtifacts(arguments[1:])
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

type artifactCommand struct {
	Kind            string
	ActorUsername   string
	SourceDirectory string
	ManifestPath    string
	AllPublished    bool
	ProjectID       *uuid.UUID
	Apply           bool
}

func parseArtifactCommand(arguments []string) (artifactCommand, error) {
	if len(arguments) == 0 {
		return artifactCommand{}, usageError()
	}
	command := artifactCommand{Kind: arguments[0]}
	flags := flag.NewFlagSet("artifacts "+command.Kind, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&command.ActorUsername, "actor-username", "", "active administrator username")
	flags.BoolVar(&command.Apply, "apply", false, "write validated files")
	switch command.Kind {
	case "seed-demo":
		flags.StringVar(&command.SourceDirectory, "source-directory", "", "directory containing the four demo files")
		flags.BoolVar(&command.AllPublished, "all-published", false, "target every published Project")
		projectID := ""
		flags.StringVar(&projectID, "project-id", "", "target one published Project")
		if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 {
			return artifactCommand{}, usageError()
		}
		if strings.TrimSpace(command.ActorUsername) == "" || strings.TrimSpace(command.SourceDirectory) == "" || command.AllPublished == (strings.TrimSpace(projectID) != "") {
			return artifactCommand{}, usageError()
		}
		if projectID != "" {
			parsed, err := uuid.Parse(projectID)
			if err != nil {
				return artifactCommand{}, errors.New("project ID must be a UUID")
			}
			command.ProjectID = &parsed
		}
	case "import-manifest":
		flags.StringVar(&command.ManifestPath, "manifest", "", "Project-file manifest path")
		if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 || strings.TrimSpace(command.ActorUsername) == "" || strings.TrimSpace(command.ManifestPath) == "" {
			return artifactCommand{}, usageError()
		}
	default:
		return artifactCommand{}, usageError()
	}
	return command, nil
}

func runArtifacts(arguments []string) error {
	command, err := parseArtifactCommand(arguments)
	if err != nil {
		return err
	}
	configuration, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	operationContext := context.Background()
	databasePool, err := pgxpool.New(operationContext, configuration.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer databasePool.Close()
	if err := databasePool.Ping(operationContext); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	service := artifactimport.Service{
		Pool: databasePool,
		Artifacts: artifacts.Service{
			Pool:            databasePool,
			Storage:         artifacts.Storage{Root: configuration.ArtifactRoot, MaxBytes: configuration.MaxArtifactBytes},
			MaxProjectBytes: configuration.MaxProjectArtifactBytes,
		},
		MaxArtifactBytes: configuration.MaxArtifactBytes,
	}
	var entries []artifactimport.Entry
	if command.Kind == "seed-demo" {
		entries, err = service.DemoEntries(operationContext, command.SourceDirectory, command.ProjectID)
	} else {
		entries, err = artifactimport.LoadManifest(command.ManifestPath)
	}
	if err != nil {
		return err
	}
	result, runErr := service.Run(operationContext, entries, artifactimport.Options{ActorUsername: command.ActorUsername, Apply: command.Apply})
	mode := "dry-run"
	if command.Apply {
		mode = "apply"
	}
	fmt.Fprintf(os.Stdout, "mode: %s\nProjects: %d\nplanned uploads: %d\nuploaded: %d\nskipped: %d\n", mode, result.ProjectCount, result.PlannedUploads, result.Uploaded, result.Skipped)
	return runErr
}

type searchCommand struct {
	Kind      string
	ProjectID uuid.UUID
}

func parseSearchCommand(arguments []string) (searchCommand, error) {
	if len(arguments) == 1 && arguments[0] == "rebuild" {
		return searchCommand{Kind: "rebuild"}, nil
	}
	if len(arguments) == 3 && arguments[0] == "reindex-project" && arguments[1] == "--project-id" {
		projectID, err := uuid.Parse(arguments[2])
		if err != nil {
			return searchCommand{}, fmt.Errorf("project ID must be a UUID: %w", err)
		}
		return searchCommand{Kind: "reindex-project", ProjectID: projectID}, nil
	}
	return searchCommand{}, usageError()
}

func runSearch(arguments []string) error {
	command, err := parseSearchCommand(arguments)
	if err != nil {
		return err
	}
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
	service := searchservice.Service{
		Pool: databasePool,
		Index: searchservice.MeilisearchClient{
			BaseURL:     configuration.MeilisearchURL,
			APIKey:      configuration.MeilisearchAPIKey,
			TaskTimeout: 10 * time.Second,
		},
		IndexUID: configuration.MeilisearchIndex,
	}
	if command.Kind == "reindex-project" {
		result, err := service.ReindexProject(operationContext, uuid.Nil, command.ProjectID)
		if err != nil {
			return fmt.Errorf("queue Project reindex: %w", err)
		}
		fmt.Fprintf(os.Stdout, "Project %s revision %d queued for search reconciliation\n", result.ProjectID, result.DesiredRevision)
		return nil
	}
	operation, err := service.CreateRebuild(operationContext, uuid.Nil)
	if err != nil {
		return fmt.Errorf("queue search rebuild: %w", err)
	}
	fmt.Fprintf(os.Stdout, "search rebuild %s queued\n", operation.ID)
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
	return errors.New("usage: ausectl admin create | admin reset-password --username <value> | admin disable --username <value> | artifacts seed-demo --source-directory <path> --actor-username <username> (--all-published | --project-id <uuid>) [--apply] | artifacts import-manifest --manifest <path> --actor-username <username> [--apply] | catalog validate|sync | search reindex-project --project-id <uuid> | search rebuild | migrations status")
}
