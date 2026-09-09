package imports

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/people"
	"ause-discovery.local/backend/internal/platform/database"
	"ause-discovery.local/backend/internal/projects"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound          = errors.New("import batch not found")
	ErrInvalidFile       = errors.New("import file is invalid")
	ErrRevisionConflict  = errors.New("revision conflict")
	ErrInvalidState      = errors.New("import batch state does not allow this operation")
	ErrExpired           = errors.New("import batch expired")
	ErrInvalidSelection  = errors.New("import row selection is invalid")
	ErrResultUnavailable = errors.New("import result is unavailable")
)

type Service struct {
	Pool          *pgxpool.Pool
	TemporaryRoot string
	PreviewTTL    time.Duration
	ParseTimeout  time.Duration
}

func (service Service) CreatePreview(ctx context.Context, actorID uuid.UUID, sourceFilename string, expectedSize int64, source io.Reader) (Batch, error) {
	filename := filepath.Base(strings.TrimSpace(sourceFilename))
	if filename == "." || filename == "" || len(filename) > 512 {
		return Batch{}, fmt.Errorf("%w: source filename is invalid", ErrInvalidFile)
	}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if format != "csv" && format != "xlsx" {
		return Batch{}, fmt.Errorf("%w: file extension must be .csv or .xlsx", ErrInvalidFile)
	}
	if expectedSize > MaximumUploadBytes {
		return Batch{}, fmt.Errorf("%w: upload exceeds 25 MiB", ErrInvalidFile)
	}
	if strings.TrimSpace(service.TemporaryRoot) == "" {
		return Batch{}, errors.New("import temporary root is required")
	}
	if err := os.MkdirAll(service.TemporaryRoot, 0750); err != nil {
		return Batch{}, fmt.Errorf("create import temporary root: %w", err)
	}
	temporaryFile, err := os.CreateTemp(service.TemporaryRoot, ".import-")
	if err != nil {
		return Batch{}, fmt.Errorf("create import temporary file: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporaryFile, hash), io.LimitReader(source, MaximumUploadBytes+1))
	if closeErr := temporaryFile.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return Batch{}, fmt.Errorf("store import upload: %w", copyErr)
	}
	if written > MaximumUploadBytes {
		return Batch{}, fmt.Errorf("%w: upload exceeds 25 MiB", ErrInvalidFile)
	}
	if expectedSize >= 0 && written != expectedSize {
		return Batch{}, fmt.Errorf("%w: upload size did not match the received file", ErrInvalidFile)
	}
	storedSource, err := os.Open(temporaryPath)
	if err != nil {
		return Batch{}, fmt.Errorf("open stored import upload: %w", err)
	}
	defer storedSource.Close()
	parseTimeout := service.ParseTimeout
	if parseTimeout <= 0 {
		parseTimeout = 60 * time.Second
	}
	parseContext, cancel := context.WithTimeout(ctx, parseTimeout)
	defer cancel()
	drafts, err := Parse(parseContext, format, storedSource)
	if err != nil {
		return Batch{}, fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}
	rows, err := validateDrafts(parseContext, service.Pool, drafts)
	if err != nil {
		return Batch{}, err
	}
	ttl := service.PreviewTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	batchID := uuid.Must(uuid.NewV7())
	var batch Batch
	err = database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		if _, err := transaction.Exec(ctx, `INSERT INTO import_batches (id, source_filename, source_sha256, format, state, expires_at, initiated_by) VALUES ($1,$2,$3,$4,'previewing',$5,$6)`, batchID, filename, hash.Sum(nil), format, time.Now().UTC().Add(ttl), actorID); err != nil {
			return err
		}
		for _, row := range rows {
			draftJSON, err := json.Marshal(row.Draft)
			if err != nil {
				return err
			}
			issuesJSON, err := json.Marshal(row.Issues)
			if err != nil {
				return err
			}
			if _, err := transaction.Exec(ctx, `INSERT INTO import_rows (id,batch_id,row_number,import_key,draft,issues,selected,warnings_acknowledged) VALUES ($1,$2,$3,$4,$5,$6,false,false)`, row.ID, batchID, row.RowNumber, row.ImportKey, draftJSON, issuesJSON); err != nil {
				return err
			}
		}
		validRows, warningRows, errorRows := previewCounts(rows)
		stored, err := scanBatch(transaction.QueryRow(ctx, `UPDATE import_batches SET state='ready',total_row_count=$2,valid_row_count=$3,warning_row_count=$4,error_row_count=$5,revision=revision+1,updated_at=now() WHERE id=$1 AND state='previewing' RETURNING id,source_filename,source_sha256,format,state,total_row_count,valid_row_count,warning_row_count,error_row_count,revision,created_at,expires_at,committed_at`, batchID, len(rows), validRows, warningRows, errorRows))
		if err != nil {
			return err
		}
		if err := audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "import.preview_created", TargetType: "import_batch", TargetID: batchID, Metadata: map[string]any{"format": format, "total_rows": len(rows), "warning_rows": warningRows, "error_rows": errorRows}}); err != nil {
			return err
		}
		batch = stored
		return nil
	})
	return batch, err
}

func (service Service) Get(ctx context.Context, batchID uuid.UUID) (Batch, error) {
	return scanBatch(service.Pool.QueryRow(ctx, `SELECT id,source_filename,source_sha256,format,state,total_row_count,valid_row_count,warning_row_count,error_row_count,revision,created_at,expires_at,committed_at FROM import_batches WHERE id=$1`, batchID))
}

func (service Service) ListRows(ctx context.Context, batchID uuid.UUID, limit, offset int) ([]Row, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var exists bool
	if err := service.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM import_batches WHERE id=$1)", batchID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := service.Pool.Query(ctx, `SELECT rows.id,rows.batch_id,rows.row_number,rows.import_key,rows.draft,rows.issues,rows.selected,rows.warnings_acknowledged,rows.duplicate_resolution,rows.committed_project_id,batches.state FROM import_rows rows JOIN import_batches batches ON batches.id=rows.batch_id WHERE rows.batch_id=$1 ORDER BY rows.row_number LIMIT $2 OFFSET $3`, batchID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Row{}
	for rows.Next() {
		row, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (service Service) UpdateRows(ctx context.Context, actorID, batchID uuid.UUID, expectedRevision int64, updates []RowUpdate) (Batch, error) {
	if len(updates) == 0 || len(updates) > 1000 {
		return Batch{}, fmt.Errorf("%w: one to 1000 row updates are required", ErrInvalidSelection)
	}
	seenRows := map[int]bool{}
	var result Batch
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		batch, err := scanBatch(transaction.QueryRow(ctx, `SELECT id,source_filename,source_sha256,format,state,total_row_count,valid_row_count,warning_row_count,error_row_count,revision,created_at,expires_at,committed_at FROM import_batches WHERE id=$1 FOR UPDATE`, batchID))
		if err != nil {
			return err
		}
		if batch.State != "ready" {
			return ErrInvalidState
		}
		if time.Now().After(batch.ExpiresAt) {
			return ErrExpired
		}
		if batch.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		for _, update := range updates {
			if update.RowNumber <= 0 || seenRows[update.RowNumber] {
				return fmt.Errorf("%w: duplicate or invalid row number", ErrInvalidSelection)
			}
			seenRows[update.RowNumber] = true
			row, err := scanRow(transaction.QueryRow(ctx, `SELECT rows.id,rows.batch_id,rows.row_number,rows.import_key,rows.draft,rows.issues,rows.selected,rows.warnings_acknowledged,rows.duplicate_resolution,rows.committed_project_id,batches.state FROM import_rows rows JOIN import_batches batches ON batches.id=rows.batch_id WHERE rows.batch_id=$1 AND rows.row_number=$2 FOR UPDATE`, batchID, update.RowNumber))
			if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrNotFound) {
				return fmt.Errorf("%w: row %d was not found", ErrInvalidSelection, update.RowNumber)
			}
			if err != nil {
				return err
			}
			if update.Selected && rowHasErrors(row) {
				return fmt.Errorf("%w: row %d contains errors", ErrInvalidSelection, update.RowNumber)
			}
			if update.Selected && rowHasWarnings(row) && !update.AcknowledgeWarnings {
				return fmt.Errorf("%w: row %d warnings require acknowledgement", ErrInvalidSelection, update.RowNumber)
			}
			if update.Selected && len(row.DuplicateCandidateIDs) > 0 && (update.DuplicateResolution == nil || (*update.DuplicateResolution != "create" && *update.DuplicateResolution != "skip")) {
				return fmt.Errorf("%w: row %d requires a duplicate resolution", ErrInvalidSelection, update.RowNumber)
			}
			if len(row.DuplicateCandidateIDs) == 0 && update.DuplicateResolution != nil {
				return fmt.Errorf("%w: row %d has no duplicate candidates", ErrInvalidSelection, update.RowNumber)
			}
			var resolutionJSON []byte
			if update.DuplicateResolution != nil {
				resolutionJSON, err = json.Marshal(map[string]string{"action": *update.DuplicateResolution})
				if err != nil {
					return err
				}
			}
			if _, err := transaction.Exec(ctx, `UPDATE import_rows SET selected=$3,warnings_acknowledged=$4,duplicate_resolution=$5 WHERE batch_id=$1 AND row_number=$2`, batchID, update.RowNumber, update.Selected, update.AcknowledgeWarnings, resolutionJSON); err != nil {
				return err
			}
		}
		result, err = scanBatch(transaction.QueryRow(ctx, `UPDATE import_batches SET revision=revision+1,updated_at=now() WHERE id=$1 AND revision=$2 AND state='ready' RETURNING id,source_filename,source_sha256,format,state,total_row_count,valid_row_count,warning_row_count,error_row_count,revision,created_at,expires_at,committed_at`, batchID, expectedRevision))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRevisionConflict
		}
		if err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "import.selection_updated", TargetType: "import_batch", TargetID: batchID, Metadata: map[string]any{"updated_rows": len(updates)}})
	})
	return result, err
}

func (service Service) Commit(ctx context.Context, actorID, batchID uuid.UUID, expectedRevision int64) (CommitResult, error) {
	var result CommitResult
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		batch, err := scanBatch(transaction.QueryRow(ctx, `SELECT id,source_filename,source_sha256,format,state,total_row_count,valid_row_count,warning_row_count,error_row_count,revision,created_at,expires_at,committed_at FROM import_batches WHERE id=$1 FOR UPDATE`, batchID))
		if err != nil {
			return err
		}
		if batch.State == "committed" {
			return transaction.QueryRow(ctx, "SELECT result FROM import_batches WHERE id=$1", batchID).Scan(&result)
		}
		if batch.State != "ready" {
			return ErrInvalidState
		}
		if time.Now().After(batch.ExpiresAt) {
			return ErrExpired
		}
		if batch.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		if _, err := transaction.Exec(ctx, `UPDATE import_batches SET state='committing',updated_at=now() WHERE id=$1 AND revision=$2 AND state='ready'`, batchID, expectedRevision); err != nil {
			return err
		}
		rows, err := transaction.Query(ctx, `SELECT rows.id,rows.batch_id,rows.row_number,rows.import_key,rows.draft,rows.issues,rows.selected,rows.warnings_acknowledged,rows.duplicate_resolution,rows.committed_project_id,batches.state FROM import_rows rows JOIN import_batches batches ON batches.id=rows.batch_id WHERE rows.batch_id=$1 AND rows.selected ORDER BY rows.row_number FOR UPDATE`, batchID)
		if err != nil {
			return err
		}
		selectedRows := []Row{}
		for rows.Next() {
			row, err := scanRow(rows)
			if err != nil {
				rows.Close()
				return err
			}
			selectedRows = append(selectedRows, row)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(selectedRows) == 0 {
			return fmt.Errorf("%w: at least one row must be selected", ErrInvalidSelection)
		}
		result = CommitResult{BatchID: batchID, CommittedAt: time.Now().UTC(), CreatedProjectIDs: []uuid.UUID{}, SkippedRows: []int{}}
		for _, row := range selectedRows {
			if rowHasErrors(row) || (rowHasWarnings(row) && !row.WarningsAcknowledged) {
				return fmt.Errorf("%w: row %d is unresolved", ErrInvalidSelection, row.RowNumber)
			}
			if len(row.DuplicateCandidateIDs) > 0 && row.DuplicateResolution == nil {
				return fmt.Errorf("%w: row %d duplicate is unresolved", ErrInvalidSelection, row.RowNumber)
			}
			if row.DuplicateResolution != nil && *row.DuplicateResolution == "skip" {
				result.SkippedRows = append(result.SkippedRows, row.RowNumber)
				continue
			}
			issues, err := validateDraft(ctx, transaction, row.Draft)
			if err != nil {
				return err
			}
			if rowState(issues) == "error" {
				return fmt.Errorf("%w: row %d no longer passes import validation", ErrInvalidSelection, row.RowNumber)
			}
			input, err := service.resolveProjectInput(ctx, transaction, actorID, row.Draft)
			if err != nil {
				return fmt.Errorf("resolve row %d: %w", row.RowNumber, err)
			}
			project, err := (projects.Service{}).CreateInTransaction(ctx, transaction, actorID, input)
			if err != nil {
				return fmt.Errorf("create row %d Project: %w", row.RowNumber, err)
			}
			if _, err := (projects.Service{}).PublishInTransaction(ctx, transaction, actorID, project.ID, project.Revision); err != nil {
				return fmt.Errorf("publish row %d Project: %w", row.RowNumber, err)
			}
			if _, err := transaction.Exec(ctx, `UPDATE import_rows SET committed_project_id=$3 WHERE id=$1 AND batch_id=$2`, row.ID, batchID, project.ID); err != nil {
				return err
			}
			result.CreatedProjectIDs = append(result.CreatedProjectIDs, project.ID)
		}
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := transaction.Exec(ctx, `UPDATE import_batches SET state='committed',result=$3,committed_at=$4,revision=revision+1,updated_at=now() WHERE id=$1 AND revision=$2 AND state='committing'`, batchID, expectedRevision, resultJSON, result.CommittedAt); err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "import.committed", TargetType: "import_batch", TargetID: batchID, Metadata: map[string]any{"created_projects": len(result.CreatedProjectIDs), "skipped_rows": len(result.SkippedRows)}})
	})
	return result, err
}

func (service Service) GetResult(ctx context.Context, batchID uuid.UUID) (CommitResult, error) {
	var state string
	var raw []byte
	err := service.Pool.QueryRow(ctx, "SELECT state,result FROM import_batches WHERE id=$1", batchID).Scan(&state, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return CommitResult{}, ErrNotFound
	}
	if err != nil {
		return CommitResult{}, err
	}
	if state != "committed" || len(raw) == 0 {
		return CommitResult{}, ErrResultUnavailable
	}
	var result CommitResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return CommitResult{}, err
	}
	return result, nil
}

func (service Service) CleanupExpired(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	cleaned := 0
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		rows, err := transaction.Query(ctx, `WITH candidates AS (SELECT id FROM import_batches WHERE state IN ('previewing','ready','failed') AND expires_at <= now() ORDER BY expires_at LIMIT $1 FOR UPDATE SKIP LOCKED) UPDATE import_batches batches SET state='expired',revision=revision+1,updated_at=now() FROM candidates WHERE batches.id=candidates.id RETURNING batches.id`, limit)
		if err != nil {
			return err
		}
		batchIDs := []uuid.UUID{}
		for rows.Next() {
			var batchID uuid.UUID
			if err := rows.Scan(&batchID); err != nil {
				rows.Close()
				return err
			}
			batchIDs = append(batchIDs, batchID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(batchIDs) == 0 {
			return nil
		}
		if _, err := transaction.Exec(ctx, `UPDATE import_rows SET draft='{}',issues='[]',selected=false,warnings_acknowledged=false,duplicate_resolution=NULL WHERE batch_id=ANY($1)`, batchIDs); err != nil {
			return err
		}
		cleaned = len(batchIDs)
		return nil
	})
	return cleaned, err
}

func (service Service) resolveProjectInput(ctx context.Context, transaction pgx.Tx, actorID uuid.UUID, draft Draft) (projects.Input, error) {
	programID, found, err := catalogVersion(ctx, transaction, "program", draft.Project.ProgramKey, draft.Project.AcademicYear)
	if err != nil || !found {
		return projects.Input{}, errors.New("Program version is unavailable")
	}
	courseID, found, err := catalogVersion(ctx, transaction, "course", draft.Project.CourseKey, draft.Project.AcademicYear)
	if err != nil || !found {
		return projects.Input{}, errors.New("Course version is unavailable")
	}
	var majorID *uuid.UUID
	if draft.Project.MajorKey != "" {
		id, found, err := catalogVersion(ctx, transaction, "major", draft.Project.MajorKey, draft.Project.AcademicYear)
		if err != nil || !found {
			return projects.Input{}, errors.New("Major version is unavailable")
		}
		majorID = &id
	}
	participations := make([]projects.Participation, 0, len(draft.Participations))
	for _, value := range draft.Participations {
		personID, err := resolvePerson(ctx, transaction, actorID, value)
		if err != nil {
			return projects.Input{}, err
		}
		participations = append(participations, projects.Participation{PersonID: personID, Role: value.Role, SortOrder: rolePosition(participations, value.Role)})
	}
	taxonomyValues := make([]projects.TaxonomyValue, 0, len(draft.Classifications))
	dimensionPositions := map[string]int{}
	for _, value := range draft.Classifications {
		var taxonomyID uuid.UUID
		if err := transaction.QueryRow(ctx, `SELECT id FROM taxonomy_values WHERE dimension=$1 AND key=$2 AND retired_at IS NULL`, value.Dimension, value.Key).Scan(&taxonomyID); err != nil {
			return projects.Input{}, err
		}
		taxonomyValues = append(taxonomyValues, projects.TaxonomyValue{ID: taxonomyID, SortOrder: dimensionPositions[value.Dimension]})
		dimensionPositions[value.Dimension]++
	}
	title := strings.TrimSpace(draft.Project.Title)
	abstract := strings.TrimSpace(draft.Project.Abstract)
	semester := strings.TrimSpace(draft.Project.Semester)
	year := draft.Project.AcademicYear
	var referenceCode *string
	if trimmed := strings.TrimSpace(draft.Project.ReferenceCode); trimmed != "" {
		referenceCode = &trimmed
	}
	return projects.Input{ReferenceCode: referenceCode, Title: &title, Abstract: &abstract, AcademicYear: &year, Semester: &semester, ProgramVersionID: &programID, MajorVersionID: majorID, CourseVersionID: &courseID, TitleAliases: draft.Project.TitleAliases, Participations: participations, TaxonomyValues: taxonomyValues, ExtensionMetadata: map[string]any{"import_key": draft.ImportKey, "import_schema_version": draft.SchemaVersion}}, nil
}

func resolvePerson(ctx context.Context, transaction pgx.Tx, actorID uuid.UUID, value Participation) (uuid.UUID, error) {
	var personID uuid.UUID
	var err error
	if value.StudentID != "" {
		err = transaction.QueryRow(ctx, "SELECT id FROM people WHERE student_id=$1", value.StudentID).Scan(&personID)
	} else if value.StaffID != "" {
		err = transaction.QueryRow(ctx, "SELECT id FROM people WHERE staff_id=$1", strings.ToLower(value.StaffID)).Scan(&personID)
	} else {
		err = pgx.ErrNoRows
	}
	if err == nil {
		return personID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	if value.StudentID == "" && value.StaffID == "" {
		matches, err := findPeopleByNormalizedName(ctx, transaction, normalizeText(value.DisplayName))
		if err != nil {
			return uuid.Nil, err
		}
		switch len(matches) {
		case 1:
			return matches[0], nil
		case 2:
			return uuid.Nil, errors.New("Person name matches multiple records; add a Student ID or Staff ID")
		}
	}
	studentID := optionalString(value.StudentID)
	staffID := optionalString(value.StaffID)
	person, err := (people.Service{}).CreateInTransaction(ctx, transaction, actorID, people.Input{DisplayName: value.DisplayName, StudentID: studentID, StaffID: staffID})
	if err != nil {
		return uuid.Nil, err
	}
	return person.ID, nil
}

func findPeopleByNormalizedName(ctx context.Context, transaction pgx.Tx, normalizedName string) ([]uuid.UUID, error) {
	rows, err := transaction.Query(ctx, `SELECT id FROM people WHERE normalized_name=$1 ORDER BY id LIMIT 2`, normalizedName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	matches := []uuid.UUID{}
	for rows.Next() {
		var personID uuid.UUID
		if err := rows.Scan(&personID); err != nil {
			return nil, err
		}
		matches = append(matches, personID)
	}
	return matches, rows.Err()
}

func scanBatch(row pgx.Row) (Batch, error) {
	var batch Batch
	var sha []byte
	err := row.Scan(&batch.ID, &batch.SourceFilename, &sha, &batch.Format, &batch.State, &batch.TotalRows, &batch.ValidRows, &batch.WarningRows, &batch.ErrorRows, &batch.Revision, &batch.CreatedAt, &batch.ExpiresAt, &batch.CommittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Batch{}, ErrNotFound
	}
	if err != nil {
		return Batch{}, err
	}
	batch.SourceSHA256 = hex.EncodeToString(sha)
	return batch, nil
}

func scanRow(scanner interface{ Scan(...any) error }) (Row, error) {
	var row Row
	var draftJSON, issuesJSON []byte
	var resolutionJSON []byte
	var batchState string
	err := scanner.Scan(&row.ID, &row.BatchID, &row.RowNumber, &row.ImportKey, &draftJSON, &issuesJSON, &row.Selected, &row.WarningsAcknowledged, &resolutionJSON, &row.CommittedProjectID, &batchState)
	if err != nil {
		return Row{}, err
	}
	if len(draftJSON) > 0 {
		if err := json.Unmarshal(draftJSON, &row.Draft); err != nil {
			return Row{}, err
		}
	}
	if len(issuesJSON) > 0 {
		if err := json.Unmarshal(issuesJSON, &row.Issues); err != nil {
			return Row{}, err
		}
	}
	row.DuplicateCandidateIDs = candidateProjectIDs(row.Issues)
	if len(resolutionJSON) > 0 {
		var resolution struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(resolutionJSON, &resolution); err != nil {
			return Row{}, err
		}
		if resolution.Action != "" {
			row.DuplicateResolution = &resolution.Action
		}
	}
	row.State = rowState(row.Issues)
	if row.Selected {
		row.State = "selected"
	}
	if batchState == "committed" && row.CommittedProjectID != nil {
		row.State = "committed"
	} else if batchState == "committed" && row.DuplicateResolution != nil && *row.DuplicateResolution == "skip" {
		row.State = "skipped"
	}
	return row, nil
}

func previewCounts(rows []Row) (int, int, int) {
	valid, warnings, failures := 0, 0, 0
	for _, row := range rows {
		switch row.State {
		case "valid":
			valid++
		case "warning":
			warnings++
		case "error":
			failures++
		}
	}
	return valid, warnings, failures
}

func rowHasErrors(row Row) bool {
	return rowState(row.Issues) == "error"
}

func rowHasWarnings(row Row) bool {
	for _, issue := range row.Issues {
		if issue.Severity == "warning" {
			return true
		}
	}
	return false
}

func rolePosition(values []projects.Participation, role string) int {
	position := 0
	for _, value := range values {
		if value.Role == role {
			position++
		}
	}
	return position
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
