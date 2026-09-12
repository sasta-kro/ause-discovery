package artifactimport

import (
	"crypto/sha256"
	"time"

	"ause-discovery.local/backend/internal/projectpool"
	"github.com/google/uuid"
)

// File outcome states reported on Project progress values.
const (
	FileUploaded = "uploaded"
	FileSkipped  = "skipped"
	FileFailed   = "failed"
)

// Worker bounds are shared with the other bulk importers.
const (
	DefaultWorkers = projectpool.DefaultWorkers
	MaxWorkers     = projectpool.MaxWorkers
)

type Entry struct {
	ProjectID             uuid.UUID
	ProjectImportKey      string
	ArtifactType          string
	DisplayName           string
	OriginalFilename      string
	SourcePath            string
	SkipIfArtifactTypeSet bool
}

type Options struct {
	ActorUsername string
	Apply         bool
	// Workers bounds concurrent Project processing. Values below 1 select
	// DefaultWorkers; values above MaxWorkers are clamped.
	Workers int
	// OnApplyStart is invoked synchronously after planning succeeds and
	// immediately before worker dispatch, on apply runs only.
	OnApplyStart func(ApplyStart)
	// OnProjectDone is invoked exactly once per dispatched Project through
	// the single coordinator goroutine, so calls never overlap. The CLI
	// owns formatting and output.
	OnProjectDone func(ProjectProgress)
}

// ApplyStart summarizes the completed plan at the start of apply work.
type ApplyStart struct {
	ProjectCount   int
	PlannedUploads int
	Workers        int
}

// FileOutcome describes one planned file of a completed Project.
type FileOutcome struct {
	ArtifactType     string
	OriginalFilename string
	State            string
	SkipReason       string
	Error            string
}

// ProjectProgress reports one completed Project group. Completed and Total
// are assigned by the coordinator in completion order.
type ProjectProgress struct {
	Completed int
	Total     int
	ProjectID uuid.UUID
	Title     string
	Files     []FileOutcome
	Duration  time.Duration
}

type Result struct {
	ProjectCount   int
	PlannedUploads int
	Uploaded       int
	Skipped        int
	FailedProjects int
	FailedFiles    int
}

// PreparedUpload is one validated Project File with its resolved Project,
// digest, and planned skip reason.
type PreparedUpload struct {
	Entry       Entry
	ProjectID   uuid.UUID
	ProjectName string
	ByteCount   int64
	SHA256      [sha256.Size]byte
	SkipReason  string
}

// projectWork is one Project's ordered share of the validated plan.
type projectWork struct {
	ID      uuid.UUID
	Name    string
	Uploads []PreparedUpload
}

// projectFiles carries the per-Project fold of file outcomes through the
// shared dispatch pool.
type projectFiles struct {
	Uploaded    int
	Skipped     int
	FailedFiles int
	Files       []FileOutcome
}
