package artifactimport

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/auth"
	"ause-discovery.local/backend/internal/projectpool"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	Pool             *pgxpool.Pool
	Artifacts        artifacts.Service
	MaxArtifactBytes int64
}

type projectState struct {
	ID              uuid.UUID
	Name            string
	Status          string
	ActiveBytes     int64
	PlannedBytes    int64
	ArtifactTypes   map[string]bool
	ArtifactDigests map[string]bool
}

var demoFiles = []struct {
	Filename         string
	ArtifactType     string
	DisplayName      string
	OriginalFilename string
}{
	{Filename: "mock-report.pdf", ArtifactType: "report", DisplayName: "Final report", OriginalFilename: "final-report.pdf"},
	{Filename: "mock-slides.pdf", ArtifactType: "slides", DisplayName: "Presentation slides", OriginalFilename: "presentation-slides.pdf"},
	{Filename: "mock-poster.png", ArtifactType: "poster", DisplayName: "Project poster", OriginalFilename: "project-poster.png"},
	{Filename: "mock-source-code.zip", ArtifactType: "source_code", DisplayName: "Source code", OriginalFilename: "source-code.zip"},
}

func (service Service) DemoEntries(ctx context.Context, sourceDirectory string, projectID *uuid.UUID) ([]Entry, error) {
	root, err := filepath.Abs(strings.TrimSpace(sourceDirectory))
	if err != nil || strings.TrimSpace(sourceDirectory) == "" {
		return nil, errors.New("source directory is required")
	}
	projectIDs := []uuid.UUID{}
	if projectID != nil {
		var status string
		if err := service.Pool.QueryRow(ctx, "SELECT status FROM projects WHERE id=$1", *projectID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Project was not found")
		} else if err != nil {
			return nil, err
		} else if status != "published" {
			return nil, errors.New("demo Project must be published")
		}
		projectIDs = append(projectIDs, *projectID)
	} else {
		rows, err := service.Pool.Query(ctx, "SELECT id FROM projects WHERE status='published' ORDER BY id")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			projectIDs = append(projectIDs, id)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	if len(projectIDs) == 0 {
		return nil, errors.New("no published Projects were found")
	}
	resolvedFiles := map[string]string{}
	for _, demoFile := range demoFiles {
		resolved, err := resolveSourcePath(root, demoFile.Filename)
		if err != nil {
			return nil, fmt.Errorf("demo file %s: %w", demoFile.Filename, err)
		}
		resolvedFiles[demoFile.Filename] = resolved
	}
	entries := make([]Entry, 0, len(projectIDs)*len(demoFiles))
	for _, id := range projectIDs {
		for _, demoFile := range demoFiles {
			entries = append(entries, Entry{
				ProjectID:             id,
				ArtifactType:          demoFile.ArtifactType,
				DisplayName:           demoFile.DisplayName,
				OriginalFilename:      demoFile.OriginalFilename,
				SourcePath:            resolvedFiles[demoFile.Filename],
				SkipIfArtifactTypeSet: true,
			})
		}
	}
	return entries, nil
}

// ResolveActor resolves and validates the audit actor username.
func (service Service) ResolveActor(ctx context.Context, username string) (uuid.UUID, error) {
	return service.activeActorID(ctx, username)
}

func (service Service) Run(ctx context.Context, entries []Entry, options Options) (Result, error) {
	if service.Pool == nil {
		return Result{}, errors.New("database pool is required")
	}
	actorID, err := service.activeActorID(ctx, options.ActorUsername)
	if err != nil {
		return Result{}, err
	}
	plan, result, err := service.plan(ctx, entries)
	if err != nil || !options.Apply {
		return result, err
	}
	workers := projectpool.Normalize(options.Workers)
	groups := groupByProject(plan)
	if options.OnApplyStart != nil {
		options.OnApplyStart(ApplyStart{ProjectCount: len(groups), PlannedUploads: result.PlannedUploads, Workers: workers})
	}
	firstError, canceled := projectpool.Dispatch(ctx, projectpoolWork(groups), workers,
		func(ctx context.Context, work projectpool.Work[projectWork]) projectpool.Outcome[projectFiles] {
			return service.runProject(ctx, actorID, work.Body)
		},
		func(outcome projectpool.Outcome[projectFiles]) {
			result.Uploaded += outcome.Body.Uploaded
			result.Skipped += outcome.Body.Skipped
			result.FailedFiles += outcome.Body.FailedFiles
			if outcome.Failed {
				result.FailedProjects++
			}
			if options.OnProjectDone != nil {
				options.OnProjectDone(ProjectProgress{
					Completed: outcome.Completed, Total: outcome.Total,
					ProjectID: outcome.ID, Title: outcome.Title,
					Files: outcome.Body.Files, Duration: outcome.Duration,
				})
			}
		})
	if canceled && result.FailedProjects == 0 {
		return result, fmt.Errorf("Project file import was canceled: %w", ctx.Err())
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

func projectpoolWork(groups []projectWork) []projectpool.Work[projectWork] {
	works := make([]projectpool.Work[projectWork], len(groups))
	for index, group := range groups {
		works[index] = projectpool.Work[projectWork]{ID: group.ID, Title: group.Name, Body: group}
	}
	return works
}

// groupByProject splits the validated plan into Project groups, preserving
// first-appearance Project order for dispatch and file order within each
// Project.
func groupByProject(plan []PreparedUpload) []projectWork {
	index := map[uuid.UUID]int{}
	groups := []projectWork{}
	for _, upload := range plan {
		position, found := index[upload.ProjectID]
		if !found {
			position = len(groups)
			index[upload.ProjectID] = position
			groups = append(groups, projectWork{ID: upload.ProjectID, Name: upload.ProjectName})
		}
		groups[position].Uploads = append(groups[position].Uploads, upload)
	}
	return groups
}

// UploadOne performs the final skip check, revision read, and upload for
// one prepared file. It returns whether the file was skipped.
func (service Service) UploadOne(ctx context.Context, actorID uuid.UUID, upload PreparedUpload) (bool, error) {
	skip, err := service.shouldSkip(ctx, upload)
	if err != nil {
		return false, fmt.Errorf("check existing Artifacts: %w", err)
	}
	if skip {
		return true, nil
	}
	var revision int64
	var status string
	if err := service.Pool.QueryRow(ctx, "SELECT revision,status FROM projects WHERE id=$1", upload.ProjectID).Scan(&revision, &status); err != nil {
		return false, fmt.Errorf("read Project %s: %w", upload.ProjectName, err)
	}
	if status == "deleted" {
		return false, fmt.Errorf("Project %s is deleted", upload.ProjectName)
	}
	file, err := os.Open(upload.Entry.SourcePath)
	if err != nil {
		return false, fmt.Errorf("open source file %s: %w", filepath.Base(upload.Entry.SourcePath), err)
	}
	_, uploadErr := service.Artifacts.Upload(ctx, actorID, upload.ProjectID, revision, artifacts.UploadInput{
		ArtifactType:     upload.Entry.ArtifactType,
		DisplayName:      upload.Entry.DisplayName,
		OriginalFilename: originalFilename(upload.Entry),
		ExpectedSize:     upload.ByteCount,
		Content:          file,
	})
	closeErr := file.Close()
	if uploadErr != nil {
		return false, fmt.Errorf("upload %s: %w", filepath.Base(upload.Entry.SourcePath), uploadErr)
	}
	if closeErr != nil {
		return false, fmt.Errorf("close source file %s: %w", filepath.Base(upload.Entry.SourcePath), closeErr)
	}
	return false, nil
}

// runProject uploads one Project's planned files sequentially, preserving
// the final skip check and the fresh revision read before every upload. The
// first failure stops that Project's remaining files without discarding
// committed work.
func (service Service) runProject(ctx context.Context, actorID uuid.UUID, work projectWork) projectpool.Outcome[projectFiles] {
	outcome := projectpool.Outcome[projectFiles]{ID: work.ID, Title: work.Name}
	started := time.Now()
	fail := func(upload PreparedUpload, err error) {
		outcome.Failed = true
		outcome.Body.FailedFiles++
		outcome.FirstError = err.Error()
		outcome.Body.Files = append(outcome.Body.Files, FileOutcome{
			ArtifactType:     upload.Entry.ArtifactType,
			OriginalFilename: originalFilename(upload.Entry),
			State:            FileFailed,
			Error:            err.Error(),
		})
		if errors.Is(err, artifacts.ErrStorageUnavailable) || ctx.Err() != nil {
			outcome.Fatal = true
		}
	}
	for _, upload := range work.Uploads {
		if ctx.Err() != nil {
			fail(upload, fmt.Errorf("canceled before upload: %w", ctx.Err()))
			break
		}
		if upload.SkipReason != "" {
			// Planned skips are already counted by plan(); report them on
			// the progress line without counting them again.
			outcome.Body.Files = append(outcome.Body.Files, FileOutcome{
				ArtifactType:     upload.Entry.ArtifactType,
				OriginalFilename: originalFilename(upload.Entry),
				State:            FileSkipped,
				SkipReason:       upload.SkipReason,
			})
			continue
		}
		skipped, err := service.UploadOne(ctx, actorID, upload)
		if err != nil {
			fail(upload, err)
			break
		}
		if skipped {
			outcome.Body.Skipped++
			outcome.Body.Files = append(outcome.Body.Files, FileOutcome{
				ArtifactType:     upload.Entry.ArtifactType,
				OriginalFilename: originalFilename(upload.Entry),
				State:            FileSkipped,
				SkipReason:       "already present",
			})
			continue
		}
		outcome.Body.Uploaded++
		outcome.Body.Files = append(outcome.Body.Files, FileOutcome{
			ArtifactType:     upload.Entry.ArtifactType,
			OriginalFilename: originalFilename(upload.Entry),
			State:            FileUploaded,
		})
	}
	outcome.Duration = time.Since(started)
	return outcome
}

// PlanUploads validates every entry, resolves its Project, and classifies
// planned skips. It performs the whole-operation quota validation.
func (service Service) PlanUploads(ctx context.Context, entries []Entry) ([]PreparedUpload, Result, error) {
	return service.plan(ctx, entries)
}

// ShouldSkip performs the final pre-upload duplicate check for one prepared
// file.
func (service Service) ShouldSkip(ctx context.Context, upload PreparedUpload) (bool, error) {
	return service.shouldSkip(ctx, upload)
}

func (service Service) plan(ctx context.Context, entries []Entry) ([]PreparedUpload, Result, error) {
	if len(entries) == 0 {
		return nil, Result{}, errors.New("no Project files were supplied")
	}
	projects := map[uuid.UUID]*projectState{}
	inspections := map[string]artifacts.UploadInspection{}
	plan := make([]PreparedUpload, 0, len(entries))
	result := Result{}
	for index, entry := range entries {
		project, err := service.resolveProject(ctx, entry, projects)
		if err != nil {
			return nil, result, fmt.Errorf("entry %d: %w", index+1, err)
		}
		cacheKey := entry.ArtifactType + "\x00" + entry.DisplayName + "\x00" + originalFilename(entry) + "\x00" + entry.SourcePath
		inspection, found := inspections[cacheKey]
		if !found {
			file, err := os.Open(entry.SourcePath)
			if err != nil {
				return nil, result, fmt.Errorf("entry %d: open source file: %w", index+1, err)
			}
			information, err := file.Stat()
			if err != nil {
				file.Close()
				return nil, result, fmt.Errorf("entry %d: inspect source file: %w", index+1, err)
			}
			inspection, err = artifacts.InspectUpload(ctx, artifacts.UploadInput{
				ArtifactType:     entry.ArtifactType,
				DisplayName:      entry.DisplayName,
				OriginalFilename: originalFilename(entry),
				ExpectedSize:     information.Size(),
				Content:          file,
			}, service.MaxArtifactBytes)
			closeErr := file.Close()
			if err != nil {
				return nil, result, fmt.Errorf("entry %d: validate source file: %w", index+1, err)
			}
			if closeErr != nil {
				return nil, result, fmt.Errorf("entry %d: close source file: %w", index+1, closeErr)
			}
			inspections[cacheKey] = inspection
		}
		upload := PreparedUpload{Entry: entry, ProjectID: project.ID, ProjectName: project.Name, ByteCount: inspection.ByteCount, SHA256: inspection.SHA256}
		digestKey := artifactDigestKey(entry.ArtifactType, inspection.SHA256[:])
		if entry.SkipIfArtifactTypeSet && project.ArtifactTypes[entry.ArtifactType] {
			upload.SkipReason = "Project already has an active file of this type"
		} else if project.ArtifactDigests[digestKey] {
			upload.SkipReason = "Project already has this active file"
		} else if project.ActiveBytes+project.PlannedBytes+inspection.ByteCount > service.Artifacts.MaxProjectBytes {
			return nil, result, fmt.Errorf("entry %d: Project %s would exceed its active file quota", index+1, project.Name)
		} else {
			project.PlannedBytes += inspection.ByteCount
			project.ArtifactTypes[entry.ArtifactType] = true
			project.ArtifactDigests[digestKey] = true
			result.PlannedUploads++
		}
		if upload.SkipReason != "" {
			result.Skipped++
		}
		plan = append(plan, upload)
	}
	result.ProjectCount = len(projects)
	return plan, result, nil
}

func (service Service) resolveProject(ctx context.Context, entry Entry, cache map[uuid.UUID]*projectState) (*projectState, error) {
	if entry.ProjectID == uuid.Nil && strings.TrimSpace(entry.ProjectImportKey) == "" {
		return nil, errors.New("Project ID or import key is required")
	}
	if entry.ProjectID != uuid.Nil && strings.TrimSpace(entry.ProjectImportKey) != "" {
		return nil, errors.New("only one Project identifier may be supplied")
	}
	projectID := entry.ProjectID
	if projectID == uuid.Nil {
		rows, err := service.Pool.Query(ctx, `SELECT id FROM projects WHERE extra_metadata->>'import_key'=$1 AND status <> 'deleted' ORDER BY id LIMIT 2`, strings.TrimSpace(entry.ProjectImportKey))
		if err != nil {
			return nil, err
		}
		matches := []uuid.UUID{}
		for rows.Next() {
			var match uuid.UUID
			if err := rows.Scan(&match); err != nil {
				rows.Close()
				return nil, err
			}
			matches = append(matches, match)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("Project import key %q was not found", entry.ProjectImportKey)
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("Project import key %q is ambiguous", entry.ProjectImportKey)
		}
		projectID = matches[0]
	}
	if cached := cache[projectID]; cached != nil {
		return cached, nil
	}
	project := &projectState{ID: projectID, ArtifactTypes: map[string]bool{}, ArtifactDigests: map[string]bool{}}
	if err := service.Pool.QueryRow(ctx, `SELECT coalesce(title,'Untitled Project'),status FROM projects WHERE id=$1`, projectID).Scan(&project.Name, &project.Status); errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("Project was not found")
	} else if err != nil {
		return nil, err
	}
	if project.Status == "deleted" {
		return nil, errors.New("Project is deleted")
	}
	rows, err := service.Pool.Query(ctx, `SELECT type,sha256,byte_count FROM artifacts WHERE project_id=$1 AND status='active'`, projectID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var artifactType string
		var digest []byte
		var byteCount int64
		if err := rows.Scan(&artifactType, &digest, &byteCount); err != nil {
			rows.Close()
			return nil, err
		}
		project.ArtifactTypes[artifactType] = true
		project.ArtifactDigests[artifactDigestKey(artifactType, digest)] = true
		project.ActiveBytes += byteCount
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	cache[projectID] = project
	return project, nil
}

func (service Service) shouldSkip(ctx context.Context, upload PreparedUpload) (bool, error) {
	if upload.Entry.SkipIfArtifactTypeSet {
		var exists bool
		err := service.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM artifacts WHERE project_id=$1 AND type=$2 AND status='active')`, upload.ProjectID, upload.Entry.ArtifactType).Scan(&exists)
		return exists, err
	}
	var exists bool
	err := service.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM artifacts WHERE project_id=$1 AND type=$2 AND sha256=$3 AND status='active')`, upload.ProjectID, upload.Entry.ArtifactType, upload.SHA256[:]).Scan(&exists)
	return exists, err
}

func (service Service) activeActorID(ctx context.Context, username string) (uuid.UUID, error) {
	normalized := auth.NormalizeUsername(username)
	if normalized == "" {
		return uuid.Nil, errors.New("actor username is required")
	}
	var actorID uuid.UUID
	if err := service.Pool.QueryRow(ctx, "SELECT id FROM application_users WHERE username=$1 AND status='active'", normalized).Scan(&actorID); errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("active administrator %q was not found", normalized)
	} else if err != nil {
		return uuid.Nil, err
	}
	return actorID, nil
}

func artifactDigestKey(artifactType string, digest []byte) string {
	return artifactType + ":" + hex.EncodeToString(digest)
}

// OriginalFilename returns the entry's explicit filename or the source
// basename.
func OriginalFilename(entry Entry) string {
	return originalFilename(entry)
}

func originalFilename(entry Entry) string {
	if filename := strings.TrimSpace(entry.OriginalFilename); filename != "" {
		return filename
	}
	return filepath.Base(entry.SourcePath)
}

// ResolveSourcePath resolves one manifest-relative path against the bundle
// root with symlink and path-escape protection.
func ResolveSourcePath(root, relativePath string) (string, error) {
	return resolveSourcePath(root, relativePath)
}

func resolveSourcePath(root, relativePath string) (string, error) {
	if relativePath == "" {
		return "", errors.New("path is required")
	}
	if filepath.IsAbs(relativePath) {
		return "", errors.New("path must be relative")
	}
	rootPath, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve source directory: %w", err)
	}
	filePath, err := filepath.EvalSymlinks(filepath.Join(rootPath, filepath.Clean(relativePath)))
	if err != nil {
		return "", fmt.Errorf("resolve source file: %w", err)
	}
	relative, err := filepath.Rel(rootPath, filePath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes the source directory")
	}
	information, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("inspect source file: %w", err)
	}
	if !information.Mode().IsRegular() {
		return "", errors.New("path must identify a regular file")
	}
	return filePath, nil
}
