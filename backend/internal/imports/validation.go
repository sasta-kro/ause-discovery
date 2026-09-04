package imports

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var studentIDPattern = regexp.MustCompile(`^[0-9]{7}$`)
var stableKeyPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func validateDrafts(ctx context.Context, database queryRower, drafts []Draft) ([]Row, error) {
	rows := make([]Row, 0, len(drafts))
	for index, draft := range drafts {
		issues, err := validateDraft(ctx, database, draft)
		if err != nil {
			return nil, err
		}
		rows = append(rows, Row{ID: uuid.Must(uuid.NewV7()), RowNumber: index + 1, ImportKey: draft.ImportKey, State: rowState(issues), Draft: draft, Issues: issues, DuplicateCandidateIDs: candidateProjectIDs(issues)})
	}
	return rows, nil
}

func validateDraft(ctx context.Context, database queryRower, draft Draft) ([]Issue, error) {
	issues := []Issue{}
	addError := func(field, code, message string) { issues = append(issues, newIssue(field, code, "error", message)) }
	if strings.TrimSpace(draft.ImportKey) == "" || utf8.RuneCountInString(draft.ImportKey) > 256 {
		addError("import_key", "invalid_import_key", "Import key is required and must not exceed 256 characters.")
	}
	if strings.TrimSpace(draft.Project.Title) == "" || utf8.RuneCountInString(draft.Project.Title) > 500 {
		addError("title", "invalid_title", "Title is required and must not exceed 500 characters.")
	}
	if strings.TrimSpace(draft.Project.Abstract) == "" || utf8.RuneCountInString(draft.Project.Abstract) > 30000 {
		addError("abstract", "invalid_abstract", "Abstract is required and must not exceed 30000 characters.")
	}
	if utf8.RuneCountInString(draft.Project.ReferenceCode) > 200 {
		addError("reference_code", "invalid_reference_code", "Reference code must not exceed 200 characters.")
	}
	if draft.Project.AcademicYear < 1900 || draft.Project.AcademicYear > 9999 || !validSemester(draft.Project.Semester) {
		addError("academic_year", "invalid_academic_period", "Academic year and semester must identify a supported period.")
	}
	if len(draft.Project.TitleAliases) > 100 {
		addError("title_aliases", "limit_exceeded", "Title aliases exceed the supported limit.")
	}
	seenAliases := map[string]bool{}
	for _, alias := range draft.Project.TitleAliases {
		normalized := normalizeText(alias)
		if normalized == "" || utf8.RuneCountInString(alias) > 500 || seenAliases[normalized] {
			addError("title_aliases", "invalid_title_alias", "Title aliases must be non-empty, unique, and no longer than 500 characters.")
			break
		}
		seenAliases[normalized] = true
	}
	if draft.Project.AcademicYear >= 1900 && draft.Project.AcademicYear <= 9999 {
		programID, programFound, err := catalogVersion(ctx, database, "program", draft.Project.ProgramKey, draft.Project.AcademicYear)
		if err != nil {
			return nil, err
		}
		if !programFound {
			addError("program_key", "unknown_catalog_key", "Program key is unknown for the academic year.")
		} else {
			var majorRequired bool
			if err := database.QueryRow(ctx, "SELECT major_required FROM program_versions WHERE id=$1", programID).Scan(&majorRequired); err != nil {
				return nil, err
			}
			if majorRequired && strings.TrimSpace(draft.Project.MajorKey) == "" {
				addError("major_key", "missing_major", "A Major key is required for the selected Program and academic year.")
			}
		}
		if strings.TrimSpace(draft.Project.MajorKey) != "" {
			if _, found, err := catalogVersion(ctx, database, "major", draft.Project.MajorKey, draft.Project.AcademicYear); err != nil {
				return nil, err
			} else if !found {
				addError("major_key", "unknown_catalog_key", "Major key is unknown for the academic year.")
			}
		}
		courseID, courseFound, err := catalogVersion(ctx, database, "course", draft.Project.CourseKey, draft.Project.AcademicYear)
		if err != nil {
			return nil, err
		}
		if !courseFound {
			addError("course_key", "unknown_catalog_key", "Course key is unknown for the academic year.")
		}
		if programFound && courseFound {
			var supported bool
			if err := database.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM course_program_versions WHERE course_version_id=$1 AND program_version_id=$2)`, courseID, programID).Scan(&supported); err != nil {
				return nil, err
			}
			if !supported {
				addError("course_key", "unsupported_course_program", "Course and Program keys are not linked for the academic year.")
			}
		}
	}
	participationIssues, err := validateParticipations(ctx, database, draft.Participations)
	if err != nil {
		return nil, err
	}
	issues = append(issues, participationIssues...)
	classificationIssues, err := validateClassifications(ctx, database, draft.Classifications)
	if err != nil {
		return nil, err
	}
	issues = append(issues, classificationIssues...)
	candidates, err := findProjectDuplicates(ctx, database, draft)
	if err != nil {
		return nil, err
	}
	if len(candidates) > 0 {
		issue := newIssue("reference_code", "project_duplicate_candidate", "warning", "Possible Project duplicates require an explicit create or skip decision.")
		issue.CandidateProjectIDs = candidates
		issues = append(issues, issue)
	}
	return issues, nil
}

func validateParticipations(ctx context.Context, database queryRower, participations []Participation) ([]Issue, error) {
	issues := []Issue{}
	roleCounts := map[string]int{}
	seen := map[string]bool{}
	for _, participation := range participations {
		role := strings.TrimSpace(participation.Role)
		roleCounts[role]++
		if !validRole(role) {
			issues = append(issues, newIssue("participations", "invalid_participation_role", "error", "Participation role is unsupported."))
		}
		if strings.TrimSpace(participation.DisplayName) == "" || utf8.RuneCountInString(participation.DisplayName) > 300 {
			issues = append(issues, newIssue("participations", "invalid_person_name", "error", "Participant display name is required and must not exceed 300 characters."))
		}
		if participation.StudentID != "" && !studentIDPattern.MatchString(participation.StudentID) {
			issues = append(issues, newIssue("participations", "invalid_student_id", "error", "Student ID must contain exactly seven digits."))
		}
		identityKey := participation.StudentID + "|" + strings.ToLower(participation.StaffID) + "|" + normalizeText(participation.DisplayName) + "|" + role
		if seen[identityKey] {
			issues = append(issues, newIssue("participations", "duplicate_participation", "error", "Duplicate participation is not allowed."))
		}
		seen[identityKey] = true
		if participation.StudentID == "" && participation.StaffID == "" && strings.TrimSpace(participation.DisplayName) != "" {
			var candidates int
			if err := database.QueryRow(ctx, "SELECT count(*) FROM people WHERE normalized_name=$1", normalizeText(participation.DisplayName)).Scan(&candidates); err != nil {
				return nil, err
			}
			if candidates > 0 {
				issues = append(issues, newIssue("participations", "person_match_required", "warning", "A matching Person name exists. Warning acknowledgement creates a distinct Person instead of merging by name."))
			}
		}
	}
	if roleCounts["student"] == 0 {
		issues = append(issues, newIssue("participations", "missing_student", "error", "At least one student is required."))
	}
	if roleCounts["advisor"] == 0 {
		issues = append(issues, newIssue("participations", "missing_advisor", "error", "At least one advisor is required."))
	}
	return issues, nil
}

func validateClassifications(ctx context.Context, database queryRower, classifications []Classification) ([]Issue, error) {
	issues := []Issue{}
	dimensionCounts := map[string]int{}
	seen := map[string]bool{}
	for _, classification := range classifications {
		dimension := strings.TrimSpace(classification.Dimension)
		key := strings.TrimSpace(classification.Key)
		dimensionCounts[dimension]++
		identity := dimension + ":" + key
		if !validDimension(dimension) || !stableKeyPattern.MatchString(key) || seen[identity] {
			issues = append(issues, newIssue("classifications", "invalid_classification", "error", "Classification dimensions and keys must be valid and unique."))
			continue
		}
		seen[identity] = true
		var exists bool
		if err := database.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM taxonomy_values WHERE dimension=$1 AND key=$2 AND retired_at IS NULL)`, dimension, key).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			issues = append(issues, newIssue("classifications", "unknown_taxonomy_key", "error", "Taxonomy key is unknown or retired."))
		}
	}
	if dimensionCounts["category"] == 0 {
		issues = append(issues, newIssue("classifications", "missing_category", "error", "At least one category is required."))
	}
	if dimensionCounts["platform"] == 0 {
		issues = append(issues, newIssue("classifications", "missing_platform", "error", "At least one platform is required."))
	}
	return issues, nil
}

func catalogVersion(ctx context.Context, database queryRower, kind, key string, year int) (uuid.UUID, bool, error) {
	if !stableKeyPattern.MatchString(strings.TrimSpace(key)) {
		return uuid.Nil, false, nil
	}
	table := kind + "s"
	versionTable := kind + "_versions"
	foreignKey := kind + "_id"
	statement := `SELECT versions.id FROM ` + versionTable + ` versions JOIN ` + table + ` identities ON identities.id=versions.` + foreignKey + ` WHERE identities.key=$1 AND versions.valid_from_year <= $2 AND (versions.valid_to_year IS NULL OR versions.valid_to_year >= $2)`
	var id uuid.UUID
	err := database.QueryRow(ctx, statement, strings.TrimSpace(key), year).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

func findProjectDuplicates(ctx context.Context, database queryRower, draft Draft) ([]uuid.UUID, error) {
	rowsDatabase, ok := database.(interface {
		Query(context.Context, string, ...any) (pgx.Rows, error)
	})
	if !ok {
		return nil, errors.New("duplicate validation requires query support")
	}
	rows, err := rowsDatabase.Query(ctx, `SELECT id FROM projects WHERE status <> 'deleted' AND (($1 <> '' AND lower(reference_code)=lower($1)) OR (lower(btrim(title))=lower(btrim($2)) AND academic_year=$3)) ORDER BY id LIMIT 20`, strings.TrimSpace(draft.Project.ReferenceCode), strings.TrimSpace(draft.Project.Title), draft.Project.AcademicYear)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func newIssue(field, code, severity, message string) Issue {
	fieldValue := field
	return Issue{Field: &fieldValue, Code: code, Severity: severity, Message: message}
}

func rowState(issues []Issue) string {
	state := "valid"
	for _, issue := range issues {
		if issue.Severity == "error" {
			return "error"
		}
		state = "warning"
	}
	return state
}

func candidateProjectIDs(issues []Issue) []uuid.UUID {
	result := []uuid.UUID{}
	for _, issue := range issues {
		result = append(result, issue.CandidateProjectIDs...)
	}
	return result
}

func validSemester(value string) bool {
	return value == "first" || value == "second" || value == "summer"
}

func validRole(value string) bool {
	return value == "student" || value == "advisor" || value == "co_advisor" || value == "committee_member"
}

func validDimension(value string) bool {
	return value == "category" || value == "platform" || value == "domain" || value == "topic" || value == "technology"
}

func normalizeText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}
