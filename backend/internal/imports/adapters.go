package imports

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

var csvHeaders = []string{"import_key", "title", "reference_code", "abstract", "academic_year", "semester", "program_key", "major_key", "course_key", "title_aliases", "students", "advisors", "co_advisors", "committee_members", "categories", "platforms", "domains", "topics", "technologies"}
var projectSheetHeaders = []string{"import_key", "title", "reference_code", "abstract", "academic_year", "semester", "program_key", "major_key", "course_key", "title_aliases"}
var participationSheetHeaders = []string{"import_key", "role", "display_name", "student_id", "staff_id"}
var classificationSheetHeaders = []string{"import_key", "dimension", "key"}

func Parse(ctx context.Context, format string, source io.Reader) ([]Draft, error) {
	content, err := io.ReadAll(io.LimitReader(source, MaximumUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read import source: %w", err)
	}
	if int64(len(content)) > MaximumUploadBytes {
		return nil, errors.New("import upload exceeds 25 MiB")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch format {
	case "csv":
		return parseCSV(ctx, content)
	case "xlsx":
		return parseXLSX(ctx, content)
	default:
		return nil, errors.New("import format must be csv or xlsx")
	}
}

func parseCSV(ctx context.Context, content []byte) ([]Draft, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	reader.FieldsPerRecord = len(csvHeaders)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV headers: %w", err)
	}
	if !equalHeaders(headers, csvHeaders) {
		return nil, fmt.Errorf("CSV headers must exactly match %s", strings.Join(csvHeaders, ","))
	}
	drafts := []Draft{}
	seenImportKeys := map[string]bool{}
	for rowNumber := 2; ; rowNumber++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV row %d: %w", rowNumber, err)
		}
		if emptyRow(record) {
			continue
		}
		if len(drafts) >= MaximumProjectRows {
			return nil, fmt.Errorf("Project row limit exceeded at CSV row %d", rowNumber)
		}
		if err := validateCells(record, fmt.Sprintf("CSV row %d", rowNumber)); err != nil {
			return nil, err
		}
		draft, err := csvRecordDraft(record)
		if err != nil {
			return nil, fmt.Errorf("CSV row %d: %w", rowNumber, err)
		}
		if seenImportKeys[draft.ImportKey] {
			return nil, fmt.Errorf("CSV row %d: duplicate import_key %q", rowNumber, draft.ImportKey)
		}
		seenImportKeys[draft.ImportKey] = true
		drafts = append(drafts, draft)
	}
	if len(drafts) == 0 {
		return nil, errors.New("import contains no Project rows")
	}
	return drafts, nil
}

func csvRecordDraft(record []string) (Draft, error) {
	academicYear, err := parseAcademicYear(record[4])
	if err != nil {
		return Draft{}, err
	}
	titleAliases, err := decodeArray[string](record[9], "title_aliases")
	if err != nil {
		return Draft{}, err
	}
	draft := Draft{
		SchemaVersion: SchemaVersion,
		ImportKey:     strings.TrimSpace(record[0]),
		Project: ProjectDraft{
			Title: strings.TrimSpace(record[1]), ReferenceCode: strings.TrimSpace(record[2]), Abstract: strings.TrimSpace(record[3]),
			AcademicYear: academicYear, Semester: strings.TrimSpace(record[5]), ProgramKey: strings.TrimSpace(record[6]),
			MajorKey: strings.TrimSpace(record[7]), CourseKey: strings.TrimSpace(record[8]), TitleAliases: trimValues(titleAliases),
		},
		Participations:  []Participation{},
		Classifications: []Classification{},
	}
	peopleFields := []struct {
		Index int
		Role  string
		Name  string
	}{{10, "student", "students"}, {11, "advisor", "advisors"}, {12, "co_advisor", "co_advisors"}, {13, "committee_member", "committee_members"}}
	for _, field := range peopleFields {
		people, err := decodeArray[Participation](record[field.Index], field.Name)
		if err != nil {
			return Draft{}, err
		}
		for _, person := range people {
			person.Role = field.Role
			person.DisplayName = strings.TrimSpace(person.DisplayName)
			person.StudentID = strings.TrimSpace(person.StudentID)
			person.StaffID = strings.TrimSpace(person.StaffID)
			draft.Participations = append(draft.Participations, person)
		}
	}
	classificationFields := []struct {
		Index     int
		Dimension string
		Name      string
	}{{14, "category", "categories"}, {15, "platform", "platforms"}, {16, "domain", "domains"}, {17, "topic", "topics"}, {18, "technology", "technologies"}}
	for _, field := range classificationFields {
		keys, err := decodeArray[string](record[field.Index], field.Name)
		if err != nil {
			return Draft{}, err
		}
		for _, key := range keys {
			draft.Classifications = append(draft.Classifications, Classification{Dimension: field.Dimension, Key: strings.TrimSpace(key)})
		}
	}
	return draft, nil
}

func parseXLSX(ctx context.Context, content []byte) ([]Draft, error) {
	if err := inspectWorkbookArchive(content); err != nil {
		return nil, err
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("open XLSX import: %w", err)
	}
	defer workbook.Close()
	expectedSheets := map[string]bool{"Projects": true, "Participations": true, "Classifications": true}
	sheets := workbook.GetSheetList()
	if len(sheets) != len(expectedSheets) {
		return nil, errors.New("XLSX must contain exactly three visible sheets: Projects, Participations, Classifications")
	}
	for _, sheet := range sheets {
		if !expectedSheets[sheet] {
			return nil, fmt.Errorf("XLSX contains unsupported sheet %q", sheet)
		}
		visible, err := workbook.GetSheetVisible(sheet)
		if err != nil || !visible {
			return nil, fmt.Errorf("XLSX sheet %q must be visible", sheet)
		}
	}
	projectRows, err := workbookRows(ctx, workbook, "Projects", projectSheetHeaders, MaximumProjectRows)
	if err != nil {
		return nil, err
	}
	participationRows, err := workbookRows(ctx, workbook, "Participations", participationSheetHeaders, MaximumParticipationRows)
	if err != nil {
		return nil, err
	}
	classificationRows, err := workbookRows(ctx, workbook, "Classifications", classificationSheetHeaders, MaximumClassificationRows)
	if err != nil {
		return nil, err
	}
	drafts := make([]Draft, 0, len(projectRows))
	draftPositions := map[string]int{}
	for rowIndex, row := range projectRows {
		academicYear, err := parseAcademicYear(row[4])
		if err != nil {
			return nil, fmt.Errorf("Projects row %d: %w", rowIndex+2, err)
		}
		titleAliases, err := decodeArray[string](row[9], "title_aliases")
		if err != nil {
			return nil, fmt.Errorf("Projects row %d: %w", rowIndex+2, err)
		}
		importKey := strings.TrimSpace(row[0])
		if _, exists := draftPositions[importKey]; exists {
			return nil, fmt.Errorf("Projects row %d: duplicate import_key %q", rowIndex+2, importKey)
		}
		draftPositions[importKey] = len(drafts)
		drafts = append(drafts, Draft{SchemaVersion: SchemaVersion, ImportKey: importKey, Project: ProjectDraft{
			Title: strings.TrimSpace(row[1]), ReferenceCode: strings.TrimSpace(row[2]), Abstract: strings.TrimSpace(row[3]), AcademicYear: academicYear,
			Semester: strings.TrimSpace(row[5]), ProgramKey: strings.TrimSpace(row[6]), MajorKey: strings.TrimSpace(row[7]), CourseKey: strings.TrimSpace(row[8]), TitleAliases: trimValues(titleAliases),
		}, Participations: []Participation{}, Classifications: []Classification{}})
	}
	if len(drafts) == 0 {
		return nil, errors.New("import contains no Project rows")
	}
	for rowIndex, row := range participationRows {
		position, exists := draftPositions[strings.TrimSpace(row[0])]
		if !exists {
			return nil, fmt.Errorf("Participations row %d references unknown import_key %q", rowIndex+2, row[0])
		}
		cell, _ := excelize.CoordinatesToCellName(4, rowIndex+2)
		cellType, err := workbook.GetCellType("Participations", cell)
		if err != nil {
			return nil, fmt.Errorf("read Student ID cell type: %w", err)
		}
		if strings.TrimSpace(row[3]) != "" && cellType == excelize.CellTypeNumber {
			return nil, fmt.Errorf("Participations row %d contains a numeric Student ID; text is required to preserve leading zeroes", rowIndex+2)
		}
		drafts[position].Participations = append(drafts[position].Participations, Participation{
			Role: strings.TrimSpace(row[1]), DisplayName: strings.TrimSpace(row[2]), StudentID: strings.TrimSpace(row[3]), StaffID: strings.TrimSpace(row[4]),
		})
	}
	for rowIndex, row := range classificationRows {
		position, exists := draftPositions[strings.TrimSpace(row[0])]
		if !exists {
			return nil, fmt.Errorf("Classifications row %d references unknown import_key %q", rowIndex+2, row[0])
		}
		drafts[position].Classifications = append(drafts[position].Classifications, Classification{Dimension: strings.TrimSpace(row[1]), Key: strings.TrimSpace(row[2])})
	}
	return drafts, nil
}

func workbookRows(ctx context.Context, workbook *excelize.File, sheet string, headers []string, maximumRows int) ([][]string, error) {
	rows, err := workbook.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read XLSX sheet %q: %w", sheet, err)
	}
	if len(rows) == 0 || !equalHeaders(padRow(rows[0], len(headers)), headers) {
		return nil, fmt.Errorf("XLSX sheet %q headers must exactly match %s", sheet, strings.Join(headers, ","))
	}
	result := [][]string{}
	for sourceIndex, sourceRow := range rows[1:] {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row := padRow(sourceRow, len(headers))
		if len(sourceRow) > len(headers) && !emptyRow(sourceRow[len(headers):]) {
			return nil, fmt.Errorf("XLSX sheet %q row %d has unknown columns", sheet, sourceIndex+2)
		}
		if emptyRow(row) {
			continue
		}
		if len(result) >= maximumRows {
			return nil, fmt.Errorf("XLSX sheet %q row limit exceeded", sheet)
		}
		if err := validateCells(row, fmt.Sprintf("XLSX sheet %q row %d", sheet, sourceIndex+2)); err != nil {
			return nil, err
		}
		for column := range headers {
			cell, _ := excelize.CoordinatesToCellName(column+1, sourceIndex+2)
			formula, err := workbook.GetCellFormula(sheet, cell)
			if err != nil {
				return nil, fmt.Errorf("read XLSX formula at %s!%s: %w", sheet, cell, err)
			}
			if formula != "" {
				return nil, fmt.Errorf("XLSX formula at %s!%s is unsupported", sheet, cell)
			}
		}
		result = append(result, row)
	}
	return result, nil
}

func inspectWorkbookArchive(content []byte) error {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return fmt.Errorf("read XLSX archive: %w", err)
	}
	var expandedBytes uint64
	for _, file := range archive.File {
		expandedBytes += file.UncompressedSize64
		if expandedBytes > uint64(MaximumExpandedBytes) {
			return errors.New("XLSX expanded content exceeds 250 MiB")
		}
		name := strings.ToLower(file.Name)
		if strings.Contains(name, "vbaproject.bin") {
			return errors.New("XLSX macros are unsupported")
		}
		if strings.HasPrefix(name, "xl/externallinks/") {
			return errors.New("XLSX external links are unsupported")
		}
	}
	return nil
}

func decodeArray[T any](value, field string) ([]T, error) {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(value)))
	decoder.DisallowUnknownFields()
	var result []T
	if err := decoder.Decode(&result); err != nil || result == nil {
		return nil, fmt.Errorf("%s must be a strict JSON array", field)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, fmt.Errorf("%s must be a strict JSON array", field)
	}
	if len(result) > MaximumArrayItems {
		return nil, fmt.Errorf("%s exceeds %d items", field, MaximumArrayItems)
	}
	return result, nil
}

func parseAcademicYear(value string) (int, error) {
	year, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, errors.New("academic_year must be an integer")
	}
	return year, nil
}

func equalHeaders(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func padRow(row []string, length int) []string {
	result := make([]string, length)
	copy(result, row)
	return result
}

func emptyRow(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func validateCells(values []string, location string) error {
	for _, value := range values {
		if !utf8.ValidString(value) {
			return fmt.Errorf("%s contains invalid UTF-8", location)
		}
		if utf8.RuneCountInString(value) > MaximumCellRunes {
			return fmt.Errorf("%s contains a cell longer than %d Unicode code points", location, MaximumCellRunes)
		}
	}
	return nil
}

func trimValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strings.TrimSpace(value))
	}
	return result
}
