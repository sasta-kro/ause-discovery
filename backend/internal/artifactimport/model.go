package artifactimport

import (
	"crypto/sha256"

	"github.com/google/uuid"
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
}

type Result struct {
	ProjectCount   int
	PlannedUploads int
	Uploaded       int
	Skipped        int
}

type plannedUpload struct {
	Entry       Entry
	ProjectID   uuid.UUID
	ProjectName string
	ByteCount   int64
	SHA256      [sha256.Size]byte
	SkipReason  string
}
