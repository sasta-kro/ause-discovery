package artifactimport

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/auth"
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
	for _, upload := range plan {
		if upload.SkipReason != "" {
			continue
		}
		skip, err := service.shouldSkip(ctx, upload)
		if err != nil {
			return result, err
		}
		if skip {
			result.Skipped++
			continue
		}
		var revision int64
		var status string
		if err := service.Pool.QueryRow(ctx, "SELECT revision,status FROM projects WHERE id=$1", upload.ProjectID).Scan(&revision, &status); err != nil {
			return result, fmt.Errorf("read Project %s: %w", upload.ProjectName, err)
		}
		if status == "deleted" {
			return result, fmt.Errorf("Project %s is deleted", upload.ProjectName)
		}
		file, err := os.Open(upload.Entry.SourcePath)
		if err != nil {
			return result, fmt.Errorf("open %s for Project %s: %w", upload.Entry.SourcePath, upload.ProjectName, err)
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
			return result, fmt.Errorf("upload %s to Project %s: %w", filepath.Base(upload.Entry.SourcePath), upload.ProjectName, uploadErr)
		}
		if closeErr != nil {
			return result, fmt.Errorf("close %s: %w", upload.Entry.SourcePath, closeErr)
		}
		result.Uploaded++
	}
	return result, nil
}

func (service Service) plan(ctx context.Context, entries []Entry) ([]plannedUpload, Result, error) {
	if len(entries) == 0 {
		return nil, Result{}, errors.New("no Project files were supplied")
	}
	projects := map[uuid.UUID]*projectState{}
	inspections := map[string]artifacts.UploadInspection{}
	plan := make([]plannedUpload, 0, len(entries))
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
		upload := plannedUpload{Entry: entry, ProjectID: project.ID, ProjectName: project.Name, ByteCount: inspection.ByteCount, SHA256: inspection.SHA256}
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

func (service Service) shouldSkip(ctx context.Context, upload plannedUpload) (bool, error) {
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

func originalFilename(entry Entry) string {
	if filename := strings.TrimSpace(entry.OriginalFilename); filename != "" {
		return filename
	}
	return filepath.Base(entry.SourcePath)
}
