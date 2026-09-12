package projectcontentimport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"time"

	"ause-discovery.local/backend/internal/artifactimport"
	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/projectlogos"
	"ause-discovery.local/backend/internal/projectpool"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Item outcome kinds and states reported on Project progress values.
const (
	KindLogo = "logo"
	KindFile = "file"

	StateUploaded = "uploaded"
	StateSkipped  = "skipped"
	StateFailed   = "failed"
)

// Service coordinates one Project Content import: planning validates the
// complete bundle, and apply dispatches each Project's optional logo and
// Project Files sequentially through the shared bounded Project pool. Logos
// go through the Project Logo service, files through the Artifact service;
// the two domains never share records or validation rules.
type Service struct {
	Pool  *pgxpool.Pool
	Logos projectlogos.Service
	Files artifactimport.Service
}

type Options struct {
	ActorUsername string
	Apply         bool
	Workers       int
	// OnApplyStart is invoked synchronously after planning succeeds and
	// immediately before worker dispatch, on apply runs only.
	OnApplyStart func(ApplyStart)
	// OnProjectDone is invoked exactly once per dispatched Project through
	// the single coordinator goroutine, so calls never overlap.
	OnProjectDone func(ProjectProgress)
}

// ApplyStart summarizes the completed plan at the start of apply work.
type ApplyStart struct {
	ProjectCount     int
	LogoUploads      int
	LogoReplacements int
	FileUploads      int
	UnchangedSkips   int
	TotalBytes       int64
	Workers          int
}

// ItemOutcome describes one planned content item of a completed Project.
type ItemOutcome struct {
	Kind             string
	ArtifactType     string
	OriginalFilename string
	State            string
	SkipReason       string
	Error            string
}

// ProjectProgress reports one completed Project group.
type ProjectProgress struct {
	Completed int
	Total     int
	ProjectID uuid.UUID
	Title     string
	Items     []ItemOutcome
	Duration  time.Duration
}

type Result struct {
	ProjectCount     int
	LogoUploads      int
	LogoReplacements int
	FileUploads      int
	UnchangedSkips   int
	TotalBytes       int64
	FailedProjects   int
	FailedItems      int
}

type plannedLogo struct {
	ProjectID   uuid.UUID
	ProjectName string
	SourcePath  string
	ByteCount   int64
	SHA256      [sha256.Size]byte
	SkipReason  string
	Replaces    bool
}

type projectContent struct {
	ID      uuid.UUID
	Name    string
	Logo    *plannedLogo
	Uploads []artifactimport.PreparedUpload
}

type projectOutcome struct {
	LogosUploaded    int
	LogoReplacements int
	FilesUploaded    int
	UnchangedSkips   int
	FailedItems      int
	Items            []ItemOutcome
}

// Run plans the complete manifest and optionally applies it. Planning is
// synchronous and whole-operation: every Project, logo, and Project File is
// resolved and validated before any write.
func (service Service) Run(ctx context.Context, manifest Manifest, bundleRoot string, options Options) (Result, error) {
	if service.Pool == nil {
		return Result{}, errors.New("database pool is required")
	}
	actorID, err := service.Files.ResolveActor(ctx, options.ActorUsername)
	if err != nil {
		return Result{}, err
	}
	groups, result, err := service.plan(ctx, manifest, bundleRoot)
	if err != nil || !options.Apply {
		return result, err
	}
	workers := projectpool.Normalize(options.Workers)
	start := ApplyStart{
		ProjectCount: len(groups), LogoUploads: result.LogoUploads, LogoReplacements: result.LogoReplacements,
		FileUploads: result.FileUploads, UnchangedSkips: result.UnchangedSkips, TotalBytes: result.TotalBytes, Workers: workers,
	}
	// Planning counts describe the plan; apply folds actual outcomes into
	// the same fields, so they are zeroed first.
	result.LogoUploads, result.LogoReplacements, result.FileUploads, result.UnchangedSkips = 0, 0, 0, 0
	result.FailedProjects, result.FailedItems = 0, 0
	if options.OnApplyStart != nil {
		options.OnApplyStart(start)
	}
	firstError, canceled := projectpool.Dispatch(ctx, contentWorks(groups), workers,
		func(ctx context.Context, work projectpool.Work[projectContent]) projectpool.Outcome[projectOutcome] {
			return service.runProject(ctx, actorID, work.Body)
		},
		func(outcome projectpool.Outcome[projectOutcome]) {
			result.LogoUploads += outcome.Body.LogosUploaded
			result.LogoReplacements += outcome.Body.LogoReplacements
			result.FileUploads += outcome.Body.FilesUploaded
			result.UnchangedSkips += outcome.Body.UnchangedSkips
			result.FailedItems += outcome.Body.FailedItems
			if outcome.Failed {
				result.FailedProjects++
			}
			if options.OnProjectDone != nil {
				options.OnProjectDone(ProjectProgress{
					Completed: outcome.Completed, Total: outcome.Total,
					ProjectID: outcome.ID, Title: outcome.Title,
					Items: outcome.Body.Items, Duration: outcome.Duration,
				})
			}
		})
	if canceled && result.FailedProjects == 0 {
		return result, fmt.Errorf("Project content import was canceled: %w", ctx.Err())
	}
	if result.FailedProjects > 0 {
		summary := fmt.Sprintf("%d of %d Projects failed during import", result.FailedProjects, len(groups))
		if firstError != "" {
			summary += ": first error: " + firstError
		}
		return result, errors.New(summary)
	}
	return result, nil
}

func contentWorks(groups []projectContent) []projectpool.Work[projectContent] {
	works := make([]projectpool.Work[projectContent], len(groups))
	for index, group := range groups {
		works[index] = projectpool.Work[projectContent]{ID: group.ID, Title: group.Name, Body: group}
	}
	return works
}

// plan validates the complete bundle: resolves every Project, validates and
// digests every logo, classifies unchanged logos and files, and delegates
// Project File planning (including whole-operation quota validation) to the
// Artifact importer.
func (service Service) plan(ctx context.Context, manifest Manifest, bundleRoot string) ([]projectContent, Result, error) {
	result := Result{}
	groups := make([]projectContent, 0, len(manifest.Projects))
	positions := map[uuid.UUID]int{}
	fileEntries := []artifactimport.Entry{}
	fileEntryProject := []int{}
	for index := range manifest.Projects {
		entry := &manifest.Projects[index]
		projectID, name, err := service.resolveProject(ctx, entry)
		if err != nil {
			return nil, result, fmt.Errorf("project entry %d: %w", index+1, err)
		}
		// The loader deduplicates textual identities, but one database
		// Project can still be reached through both identity forms. Reject
		// the duplicate after resolution instead of silently merging the
		// entries, which could overwrite the first logo.
		if _, resolved := positions[projectID]; resolved {
			return nil, result, fmt.Errorf("project entry %d resolves to Project %s, which an earlier entry already resolved", index+1, projectID)
		}
		position := len(groups)
		positions[projectID] = position
		groups = append(groups, projectContent{ID: projectID, Name: name})
		if entry.Logo != nil {
			sourcePath, err := artifactimport.ResolveSourcePath(bundleRoot, entry.Logo.FilePath)
			if err != nil {
				return nil, result, fmt.Errorf("project %s logo: %w", name, err)
			}
			file, err := os.Open(sourcePath)
			if err != nil {
				return nil, result, fmt.Errorf("project %s logo: open source file: %w", name, err)
			}
			information, err := file.Stat()
			if err != nil {
				file.Close()
				return nil, result, fmt.Errorf("project %s logo: inspect source file: %w", name, err)
			}
			validated, validateErr := projectlogos.Validate(file, information.Size())
			_ = file.Close()
			if validateErr != nil {
				return nil, result, fmt.Errorf("project %s logo %s: %w", name, entry.Logo.FilePath, validateErr)
			}
			activeDigest, hasActive, digestErr := service.Logos.ActiveDigest(ctx, projectID)
			if digestErr != nil {
				return nil, result, digestErr
			}
			logo := &plannedLogo{ProjectID: projectID, ProjectName: name, SourcePath: sourcePath, ByteCount: validated.ByteCount, SHA256: validated.SHA256, Replaces: hasActive}
			if hasActive && activeDigest == validated.SHA256 {
				logo.SkipReason = "unchanged"
			} else {
				result.LogoUploads++
				if hasActive {
					result.LogoReplacements++
				}
			}
			result.TotalBytes += validated.ByteCount
			groups[position].Logo = logo
		}
		for fileIndex := range entry.Files {
			manifestFile := &entry.Files[fileIndex]
			sourcePath, err := artifactimport.ResolveSourcePath(bundleRoot, manifestFile.FilePath)
			if err != nil {
				return nil, result, fmt.Errorf("project %s file %s: %w", name, manifestFile.FilePath, err)
			}
			fileEntries = append(fileEntries, artifactimport.Entry{
				ProjectID:        projectID,
				ArtifactType:     manifestFile.ArtifactType,
				DisplayName:      manifestFile.DisplayName,
				OriginalFilename: manifestFile.OriginalFilename,
				SourcePath:       sourcePath,
			})
			fileEntryProject = append(fileEntryProject, position)
		}
	}
	if len(fileEntries) > 0 {
		prepared, _, err := service.Files.PlanUploads(ctx, fileEntries)
		if err != nil {
			return nil, result, err
		}
		for index, upload := range prepared {
			groups[fileEntryProject[index]].Uploads = append(groups[fileEntryProject[index]].Uploads, upload)
			if upload.SkipReason != "" {
				result.UnchangedSkips++
			} else {
				result.FileUploads++
				result.TotalBytes += upload.ByteCount
			}
		}
	}
	for _, group := range groups {
		if group.Logo != nil && group.Logo.SkipReason != "" {
			result.UnchangedSkips++
		}
	}
	result.ProjectCount = len(groups)
	return groups, result, nil
}

func (service Service) resolveProject(ctx context.Context, entry *ManifestProject) (uuid.UUID, string, error) {
	if entry.ProjectID != "" {
		projectID, err := uuid.Parse(entry.ProjectID)
		if err != nil {
			return uuid.Nil, "", errors.New("project_id must be a UUID")
		}
		var name string
		var status string
		err = service.Pool.QueryRow(ctx, `SELECT coalesce(title,'Untitled Project'),status FROM projects WHERE id=$1`, projectID).Scan(&name, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", errors.New("Project was not found")
		}
		if err != nil {
			return uuid.Nil, "", err
		}
		if status == "deleted" {
			return uuid.Nil, "", errors.New("Project is deleted")
		}
		return projectID, name, nil
	}
	rows, err := service.Pool.Query(ctx, `SELECT id, coalesce(title,'Untitled Project'), status FROM projects WHERE extra_metadata->>'import_key'=$1 AND status <> 'deleted' ORDER BY id LIMIT 2`, entry.ProjectImportKey)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer rows.Close()
	matches := []struct {
		ID     uuid.UUID
		Name   string
		Status string
	}{}
	for rows.Next() {
		var match struct {
			ID     uuid.UUID
			Name   string
			Status string
		}
		if err := rows.Scan(&match.ID, &match.Name, &match.Status); err != nil {
			return uuid.Nil, "", err
		}
		matches = append(matches, match)
	}
	if err := rows.Err(); err != nil {
		return uuid.Nil, "", err
	}
	if len(matches) == 0 {
		return uuid.Nil, "", fmt.Errorf("Project import key %q was not found", entry.ProjectImportKey)
	}
	if len(matches) > 1 {
		return uuid.Nil, "", fmt.Errorf("Project import key %q is ambiguous", entry.ProjectImportKey)
	}
	return matches[0].ID, matches[0].Name, nil
}

// runProject applies one Project's optional logo then its Project Files
// sequentially, reading the current Project revision before the logo
// mutation and delegating per-file revision handling to the Artifact
// importer. The first failure stops that Project's remaining content
// without discarding committed work.
func (service Service) runProject(ctx context.Context, actorID uuid.UUID, content projectContent) projectpool.Outcome[projectOutcome] {
	outcome := projectpool.Outcome[projectOutcome]{ID: content.ID, Title: content.Name}
	started := time.Now()
	fail := func(item ItemOutcome, err error) {
		outcome.Failed = true
		outcome.Body.FailedItems++
		item.State = StateFailed
		item.Error = err.Error()
		outcome.Body.Items = append(outcome.Body.Items, item)
		outcome.FirstError = err.Error()
		if errors.Is(err, projectlogos.ErrStorageUnavailable) || errors.Is(err, artifacts.ErrStorageUnavailable) || ctx.Err() != nil {
			outcome.Fatal = true
		}
	}
	if content.Logo != nil {
		logo := content.Logo
		item := ItemOutcome{Kind: KindLogo, ArtifactType: KindLogo, OriginalFilename: "logo.png"}
		if logo.SkipReason != "" {
			outcome.Body.UnchangedSkips++
			item.State = StateSkipped
			item.SkipReason = logo.SkipReason
			outcome.Body.Items = append(outcome.Body.Items, item)
		} else {
			file, err := os.Open(logo.SourcePath)
			if err != nil {
				fail(item, fmt.Errorf("open logo source file: %w", err))
			} else {
				validated, validateErr := projectlogos.Validate(file, -1)
				_ = file.Close()
				switch {
				case validateErr != nil:
					fail(item, fmt.Errorf("validate logo: %w", validateErr))
				default:
					activeDigest, hasActive, digestErr := service.Logos.ActiveDigest(ctx, logo.ProjectID)
					if digestErr != nil {
						fail(item, fmt.Errorf("check active logo: %w", digestErr))
					} else if hasActive && activeDigest == validated.SHA256 {
						outcome.Body.UnchangedSkips++
						item.State = StateSkipped
						item.SkipReason = "unchanged"
						outcome.Body.Items = append(outcome.Body.Items, item)
					} else {
						var revision int64
						var status string
						readErr := service.Pool.QueryRow(ctx, "SELECT revision,status FROM projects WHERE id=$1", logo.ProjectID).Scan(&revision, &status)
						if readErr != nil {
							fail(item, fmt.Errorf("read Project %s: %w", logo.ProjectName, readErr))
						} else if status == "deleted" {
							fail(item, fmt.Errorf("Project %s is deleted", logo.ProjectName))
						} else if _, uploadErr := service.Logos.Upload(ctx, actorID, logo.ProjectID, revision, projectlogos.UploadInput{ExpectedSize: validated.ByteCount, Content: bytes.NewReader(validated.Content)}); uploadErr != nil {
							fail(item, fmt.Errorf("upload logo: %w", uploadErr))
						} else {
							outcome.Body.LogosUploaded++
							if hasActive {
								outcome.Body.LogoReplacements++
							}
							item.State = StateUploaded
							outcome.Body.Items = append(outcome.Body.Items, item)
						}
					}
				}
			}
		}
		if outcome.Failed {
			outcome.Duration = time.Since(started)
			return outcome
		}
	}
	for _, upload := range content.Uploads {
		if ctx.Err() != nil {
			fail(ItemOutcome{Kind: KindFile, ArtifactType: upload.Entry.ArtifactType, OriginalFilename: artifactimport.OriginalFilename(upload.Entry)}, fmt.Errorf("canceled before upload: %w", ctx.Err()))
			break
		}
		item := ItemOutcome{Kind: KindFile, ArtifactType: upload.Entry.ArtifactType, OriginalFilename: artifactimport.OriginalFilename(upload.Entry)}
		if upload.SkipReason != "" {
			outcome.Body.UnchangedSkips++
			item.State = StateSkipped
			item.SkipReason = upload.SkipReason
			outcome.Body.Items = append(outcome.Body.Items, item)
			continue
		}
		skipped, err := service.Files.UploadOne(ctx, actorID, upload)
		if err != nil {
			fail(item, err)
			break
		}
		if skipped {
			outcome.Body.UnchangedSkips++
			item.State = StateSkipped
			item.SkipReason = "already present"
			outcome.Body.Items = append(outcome.Body.Items, item)
			continue
		}
		outcome.Body.FilesUploaded++
		item.State = StateUploaded
		outcome.Body.Items = append(outcome.Body.Items, item)
	}
	outcome.Duration = time.Since(started)
	return outcome
}
