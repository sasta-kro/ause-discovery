package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"ause-discovery.local/backend/internal/artifactimport"
	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/auth"
	"ause-discovery.local/backend/internal/catalog"
	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/platform/database"
	"ause-discovery.local/backend/internal/projectcontentimport"
	"ause-discovery.local/backend/internal/projectlinks"
	"ause-discovery.local/backend/internal/projectlogos"
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
	if len(arguments) >= 2 && arguments[0] == "project-content" {
		return runProjectContent(arguments[1:])
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
	AllPublished    bool
	ProjectID       *uuid.UUID
	Apply           bool
	Workers         int
}

func runArtifactMigration(operationContext context.Context, databasePool *pgxpool.Pool, storageSet artifacts.StorageSet, apply bool) error {
	result, err := artifacts.MigrateStorage(operationContext, databasePool, storageSet, apply)
	mode := "dry-run"
	if apply {
		mode = "apply"
	}
	fmt.Fprintf(os.Stdout, "mode: %s\ntarget backend: %s\non source backend: %d\non target backend: %d\nplanned: %d\ncopied: %d\ndigest-verified: %d\nfailed: %d\n",
		mode, storageSet.DefaultName, result.OnSourceBackend, result.OnTargetBackend, result.Planned, result.Copied, result.Verified, len(result.Failed))
	for _, failure := range result.Failed {
		fmt.Fprintf(os.Stdout, "failed artifact %s (%s): %v\n", failure.ArtifactID, failure.StorageKey, failure.Err)
	}
	if err != nil {
		return err
	}
	if apply && len(result.Failed) == 0 && result.OnSourceBackend > 0 && result.Copied == 0 {
		return errors.New("artifact storage migration copied nothing although source artifacts exist")
	}
	return nil
}

// onceWorkersFlag rejects a repeated --workers flag, which the standard
// flag package would silently overwrite.
type onceWorkersFlag struct {
	target *int
	seen   bool
}

func (value *onceWorkersFlag) String() string {
	if value == nil || value.target == nil {
		return strconv.Itoa(artifactimport.DefaultWorkers)
	}
	return strconv.Itoa(*value.target)
}

func (value *onceWorkersFlag) Set(text string) error {
	if value.seen {
		return errors.New("workers may only be set once")
	}
	parsed, err := strconv.Atoi(text)
	if err != nil {
		return err
	}
	value.seen = true
	*value.target = parsed
	return nil
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
	registerWorkers := func() {
		command.Workers = artifactimport.DefaultWorkers
		flags.Var(&onceWorkersFlag{target: &command.Workers}, "workers", "concurrent Projects, 1 through 8")
	}
	switch command.Kind {
	case "seed-demo":
		flags.StringVar(&command.SourceDirectory, "source-directory", "", "directory containing the four demo files")
		flags.BoolVar(&command.AllPublished, "all-published", false, "target every published Project")
		registerWorkers()
		projectID := ""
		flags.StringVar(&projectID, "project-id", "", "target one published Project")
		if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 {
			return artifactCommand{}, usageError()
		}
		if strings.TrimSpace(command.ActorUsername) == "" || strings.TrimSpace(command.SourceDirectory) == "" || command.AllPublished == (strings.TrimSpace(projectID) != "") {
			return artifactCommand{}, usageError()
		}
		if command.Workers < 1 || command.Workers > artifactimport.MaxWorkers {
			return artifactCommand{}, usageError()
		}
		if projectID != "" {
			parsed, err := uuid.Parse(projectID)
			if err != nil {
				return artifactCommand{}, errors.New("project ID must be a UUID")
			}
			command.ProjectID = &parsed
		}
	case "migrate":
		if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 || strings.TrimSpace(command.ActorUsername) != "" {
			return artifactCommand{}, usageError()
		}
		command.ActorUsername = ""
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
	storageSet, err := artifacts.NewStorageSet(artifacts.StorageOptions{
		DefaultName:      configuration.ArtifactStorageBackend,
		LocalRoot:        configuration.ArtifactRoot,
		MaxArtifactBytes: configuration.MaxArtifactBytes,
		B2Endpoint:       configuration.ArtifactB2Endpoint,
		B2Bucket:         configuration.ArtifactB2Bucket,
		B2KeyID:          configuration.ArtifactB2KeyID,
		B2ApplicationKey: configuration.ArtifactB2ApplicationKey,
	})
	if err != nil {
		return fmt.Errorf("configure artifact storage: %w", err)
	}
	if command.Kind == "migrate" {
		return runArtifactMigration(operationContext, databasePool, storageSet, command.Apply)
	}
	service := artifactimport.Service{
		Pool: databasePool,
		Artifacts: artifacts.Service{
			Pool:            databasePool,
			Storage:         storageSet,
			MaxProjectBytes: configuration.MaxProjectArtifactBytes,
		},
		MaxArtifactBytes: configuration.MaxArtifactBytes,
	}
	entries, err := service.DemoEntries(operationContext, command.SourceDirectory, command.ProjectID)
	if err != nil {
		return err
	}
	result, runErr := service.Run(operationContext, entries, artifactimport.Options{
		ActorUsername: command.ActorUsername,
		Apply:         command.Apply,
		Workers:       command.Workers,
		OnApplyStart: func(start artifactimport.ApplyStart) {
			fmt.Fprintf(os.Stdout, "Starting Project file import: %d Projects, %d planned uploads, %d workers\n", start.ProjectCount, start.PlannedUploads, start.Workers)
		},
		OnProjectDone: func(progress artifactimport.ProjectProgress) {
			fmt.Fprintln(os.Stdout, formatProjectProgress(progress))
		},
	})
	mode := "dry-run"
	if command.Apply {
		mode = "apply"
	}
	fmt.Fprintf(os.Stdout, "mode: %s\nProjects: %d\nplanned uploads: %d\nuploaded: %d\nskipped: %d\n", mode, result.ProjectCount, result.PlannedUploads, result.Uploaded, result.Skipped)
	if result.FailedProjects > 0 {
		fmt.Fprintf(os.Stdout, "failed Projects: %d\nfailed files: %d\n", result.FailedProjects, result.FailedFiles)
	}
	return runErr
}

// formatProjectProgress renders one completed Project as a single physical
// line. Every entry carries its Artifact type and original filename, skips
// carry their reason, failures carry the controlled error, and the Project
// ID keeps duplicate titles distinguishable. All text passes through
// sanitizeProgressText so titles, filenames, reasons, and errors can never
// inject line breaks or control characters.
func formatProjectProgress(progress artifactimport.ProjectProgress) string {
	var uploaded, skipped, failed []string
	for _, file := range progress.Files {
		switch file.State {
		case artifactimport.FileUploaded:
			uploaded = append(uploaded, fmt.Sprintf("%s (%s)", sanitizeProgressText(file.ArtifactType, 40), sanitizeProgressText(file.OriginalFilename, 120)))
		case artifactimport.FileSkipped:
			skipped = append(skipped, fmt.Sprintf("%s (%s): %s", sanitizeProgressText(file.ArtifactType, 40), sanitizeProgressText(file.OriginalFilename, 120), sanitizeProgressText(file.SkipReason, 80)))
		case artifactimport.FileFailed:
			failed = append(failed, fmt.Sprintf("%s (%s): %s", sanitizeProgressText(file.ArtifactType, 40), sanitizeProgressText(file.OriginalFilename, 120), sanitizeProgressText(file.Error, 160)))
		}
	}
	segments := []string{}
	if len(uploaded) > 0 {
		segments = append(segments, "uploaded "+strings.Join(uploaded, ", "))
	}
	if len(skipped) > 0 {
		segments = append(segments, "skipped "+strings.Join(skipped, ", "))
	}
	if len(failed) > 0 {
		segments = append(segments, "failed "+strings.Join(failed, ", "))
	}
	if len(segments) == 0 {
		segments = append(segments, "no files processed")
	}
	identity := sanitizeProgressText(progress.Title, 80) + " (" + sanitizeProgressText(progress.ProjectID.String(), 0) + ")"
	return fmt.Sprintf("[%d/%d] %s: %s; %.1fs", progress.Completed, progress.Total, identity, strings.Join(segments, "; "), progress.Duration.Seconds())
}

// sanitizeProgressText collapses control characters and line breaks and
// bounds the rendered length.
func sanitizeProgressText(text string, limit int) string {
	var builder strings.Builder
	for _, character := range text {
		if character < 0x20 || character == 0x7f {
			builder.WriteRune(' ')
		} else {
			builder.WriteRune(character)
		}
	}
	collapsed := strings.Join(strings.Fields(builder.String()), " ")
	if limit > 0 {
		runes := []rune(collapsed)
		if len(runes) > limit {
			collapsed = string(runes[:limit]) + "..."
		}
	}
	return collapsed
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
	return errors.New("usage: ausectl admin create | admin reset-password --username <value> | admin disable --username <value> | artifacts seed-demo --source-directory <path> --actor-username <username> (--all-published | --project-id <uuid>) [--workers <1-8>] [--apply] | artifacts migrate [--apply] | project-content import-manifest --manifest <path> --actor-username <username> [--workers <1-8>] [--apply] | catalog validate|sync | search reindex-project --project-id <uuid> | search rebuild | migrations status")
}

type projectContentCommand struct {
	Kind          string
	ManifestPath  string
	ActorUsername string
	Apply         bool
	Workers       int
}

func parseProjectContentCommand(arguments []string) (projectContentCommand, error) {
	if len(arguments) == 0 || arguments[0] != "import-manifest" {
		return projectContentCommand{}, usageError()
	}
	command := projectContentCommand{Kind: "import-manifest"}
	flags := flag.NewFlagSet("project-content import-manifest", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&command.ManifestPath, "manifest", "", "Project Content JSON manifest path")
	flags.StringVar(&command.ActorUsername, "actor-username", "", "active administrator username")
	flags.BoolVar(&command.Apply, "apply", false, "write validated content")
	command.Workers = projectcontentimport.DefaultWorkers
	flags.Var(&onceWorkersFlag{target: &command.Workers}, "workers", "concurrent Projects, 1 through 8")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 || strings.TrimSpace(command.ManifestPath) == "" || strings.TrimSpace(command.ActorUsername) == "" {
		return projectContentCommand{}, usageError()
	}
	if command.Workers < 1 || command.Workers > projectcontentimport.MaxWorkers {
		return projectContentCommand{}, usageError()
	}
	return command, nil
}

func runProjectContent(arguments []string) error {
	command, err := parseProjectContentCommand(arguments)
	if err != nil {
		return err
	}
	manifest, err := projectcontentimport.LoadManifest(command.ManifestPath)
	if err != nil {
		return err
	}
	bundleRoot, err := projectcontentimport.BundleRoot(command.ManifestPath)
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
	storageSet, err := artifacts.NewStorageSet(artifacts.StorageOptions{
		DefaultName:      configuration.ArtifactStorageBackend,
		LocalRoot:        configuration.ArtifactRoot,
		MaxArtifactBytes: configuration.MaxArtifactBytes,
		B2Endpoint:       configuration.ArtifactB2Endpoint,
		B2Bucket:         configuration.ArtifactB2Bucket,
		B2KeyID:          configuration.ArtifactB2KeyID,
		B2ApplicationKey: configuration.ArtifactB2ApplicationKey,
	})
	if err != nil {
		return fmt.Errorf("configure artifact storage: %w", err)
	}
	service := projectcontentimport.Service{
		Pool:  databasePool,
		Logos: projectlogos.Service{Pool: databasePool, Storage: storageSet},
		Files: artifactimport.Service{
			Pool:             databasePool,
			Artifacts:        artifacts.Service{Pool: databasePool, Storage: storageSet, MaxProjectBytes: configuration.MaxProjectArtifactBytes},
			MaxArtifactBytes: configuration.MaxArtifactBytes,
		},
		Links: projectlinks.Service{Pool: databasePool},
	}
	result, runErr := service.Run(operationContext, manifest, bundleRoot, projectcontentimport.Options{
		ActorUsername: command.ActorUsername,
		Apply:         command.Apply,
		Workers:       command.Workers,
		OnApplyStart: func(start projectcontentimport.ApplyStart) {
			fmt.Fprintf(os.Stdout, "Starting Project content import: %d Projects, %d logo uploads, %d logo replacements, %d planned file uploads, %d unchanged, %d repository links (%d Projects to replace, %d unchanged), %.1f MiB, %d workers\n",
				start.ProjectCount, start.LogoUploads, start.LogoReplacements, start.FileUploads, start.UnchangedSkips, start.DeclaredLinks, start.LinkSetsReplaced, start.LinkSetsUnchanged, float64(start.TotalBytes)/1024/1024, start.Workers)
		},
		OnProjectDone: func(progress projectcontentimport.ProjectProgress) {
			fmt.Fprintln(os.Stdout, formatProjectContentProgress(progress))
		},
	})
	mode := "dry-run"
	if command.Apply {
		mode = "apply"
	}
	fmt.Fprintf(os.Stdout, "mode: %s\nProjects: %d\nlogo uploads: %d\nlogo replacements: %d\nfile uploads: %d\nunchanged skips: %d\nrepository links declared: %d\nlink sets replaced: %d\nlink sets unchanged: %d\n", mode, result.ProjectCount, result.LogoUploads, result.LogoReplacements, result.FileUploads, result.UnchangedSkips, result.DeclaredLinks, result.LinkSetsReplaced, result.LinkSetsUnchanged)
	if command.Apply {
		fmt.Fprintf(os.Stdout, "planned bytes: %.1f MiB\n", float64(result.TotalBytes)/1024/1024)
	}
	if result.FailedProjects > 0 {
		fmt.Fprintf(os.Stdout, "failed Projects: %d\nfailed items: %d\n", result.FailedProjects, result.FailedItems)
	}
	return runErr
}

// formatProjectContentProgress renders one completed Project as a single
// physical line with the same sanitization guarantees as the Project File
// importer.
func formatProjectContentProgress(progress projectcontentimport.ProjectProgress) string {
	var uploaded, skipped, failed []string
	for _, item := range progress.Items {
		descriptor := sanitizeProgressText(item.ArtifactType, 40)
		if item.Kind == projectcontentimport.KindFile {
			descriptor = fmt.Sprintf("%s (%s)", sanitizeProgressText(item.ArtifactType, 40), sanitizeProgressText(item.OriginalFilename, 120))
		}
		if item.Kind == projectcontentimport.KindLink {
			descriptor = fmt.Sprintf("links %s", sanitizeProgressText(item.OriginalFilename, 40))
		}
		switch item.State {
		case projectcontentimport.StateUploaded:
			uploaded = append(uploaded, descriptor)
		case projectcontentimport.StateSkipped:
			skipped = append(skipped, fmt.Sprintf("%s (%s)", descriptor, sanitizeProgressText(item.SkipReason, 80)))
		case projectcontentimport.StateFailed:
			failed = append(failed, fmt.Sprintf("%s (%s)", descriptor, sanitizeProgressText(item.Error, 160)))
		}
	}
	segments := []string{}
	if len(uploaded) > 0 {
		segments = append(segments, "uploaded "+strings.Join(uploaded, ", "))
	}
	if len(skipped) > 0 {
		segments = append(segments, "unchanged "+strings.Join(skipped, ", "))
	}
	if len(failed) > 0 {
		segments = append(segments, "failed "+strings.Join(failed, ", "))
	}
	if len(segments) == 0 {
		segments = append(segments, "no content processed")
	}
	identity := sanitizeProgressText(progress.Title, 80) + " (" + sanitizeProgressText(progress.ProjectID.String(), 0) + ")"
	return fmt.Sprintf("[%d/%d] %s: %s; %.1fs", progress.Completed, progress.Total, identity, strings.Join(segments, "; "), progress.Duration.Seconds())
}
