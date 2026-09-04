package imports

import (
	"time"

	"github.com/google/uuid"
)

const (
	SchemaVersion                   = 1
	MaximumUploadBytes        int64 = 25 << 20
	MaximumProjectRows              = 5000
	MaximumParticipationRows        = 30000
	MaximumClassificationRows       = 50000
	MaximumCellRunes                = 10000
	MaximumArrayItems               = 200
	MaximumExpandedBytes      int64 = 250 << 20
)

type Draft struct {
	SchemaVersion   int              `json:"schema_version"`
	ImportKey       string           `json:"import_key"`
	Project         ProjectDraft     `json:"project"`
	Participations  []Participation  `json:"participations"`
	Classifications []Classification `json:"classifications"`
}

type ProjectDraft struct {
	Title         string   `json:"title"`
	ReferenceCode string   `json:"reference_code,omitempty"`
	Abstract      string   `json:"abstract"`
	AcademicYear  int      `json:"academic_year"`
	Semester      string   `json:"semester"`
	ProgramKey    string   `json:"program_key"`
	MajorKey      string   `json:"major_key,omitempty"`
	CourseKey     string   `json:"course_key"`
	TitleAliases  []string `json:"title_aliases"`
}

type Participation struct {
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	StudentID   string `json:"student_id,omitempty"`
	StaffID     string `json:"staff_id,omitempty"`
}

type Classification struct {
	Dimension string `json:"dimension"`
	Key       string `json:"key"`
}

type Issue struct {
	Field               *string     `json:"field,omitempty"`
	Code                string      `json:"code"`
	Severity            string      `json:"severity"`
	Message             string      `json:"message,omitempty"`
	CandidateProjectIDs []uuid.UUID `json:"candidate_project_ids,omitempty"`
}

type Batch struct {
	ID             uuid.UUID
	SourceFilename string
	SourceSHA256   string
	Format         string
	State          string
	TotalRows      int
	ValidRows      int
	WarningRows    int
	ErrorRows      int
	Revision       int64
	CreatedAt      time.Time
	ExpiresAt      time.Time
	CommittedAt    *time.Time
}

type Row struct {
	ID                    uuid.UUID
	BatchID               uuid.UUID
	RowNumber             int
	ImportKey             string
	State                 string
	Selected              bool
	WarningsAcknowledged  bool
	DuplicateResolution   *string
	Draft                 Draft
	Issues                []Issue
	DuplicateCandidateIDs []uuid.UUID
	CommittedProjectID    *uuid.UUID
}

type RowUpdate struct {
	RowNumber           int
	Selected            bool
	AcknowledgeWarnings bool
	DuplicateResolution *string
}

type CommitResult struct {
	BatchID           uuid.UUID   `json:"batch_id"`
	CommittedAt       time.Time   `json:"committed_at"`
	CreatedProjectIDs []uuid.UUID `json:"created_project_ids"`
	SkippedRows       []int       `json:"skipped_rows"`
}
