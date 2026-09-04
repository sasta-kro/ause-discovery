package imports

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestBundledTemplatesProduceCanonicalDrafts(t *testing.T) {
	paths := map[string]string{
		"csv":  "../../../frontend/public/templates/ause-discovery-import.csv",
		"xlsx": "../../../frontend/public/templates/ause-discovery-import.xlsx",
	}
	for format, path := range paths {
		t.Run(format, func(t *testing.T) {
			source, err := os.Open(path)
			if err != nil {
				t.Fatalf("open bundled template: %v", err)
			}
			defer source.Close()
			drafts, err := Parse(context.Background(), format, source)
			if err != nil {
				t.Fatalf("parse bundled template: %v", err)
			}
			if len(drafts) != 1 || len(drafts[0].Participations) != 2 || drafts[0].Participations[0].StudentID != "0123456" {
				t.Fatalf("bundled template produced unexpected drafts: %#v", drafts)
			}
		})
	}
}

const validCSV = `import_key,title,reference_code,abstract,academic_year,semester,program_key,major_key,course_key,title_aliases,students,advisors,co_advisors,committee_members,categories,platforms,domains,topics,technologies
row-001,Imported Project,REF-001,Imported abstract,2026,first,computing,,capstone,"[""Earlier Title""]","[{""display_name"":""Leading Student"",""student_id"":""0123456""}]","[{""display_name"":""Advisor Name"",""staff_id"":""advisor-1""}]",[],[],"[""ai""]","[""web""]",[],[],[]
`

func TestParseCSVProducesCanonicalDraftAndPreservesLeadingZeroStudentID(t *testing.T) {
	drafts, err := Parse(context.Background(), "csv", strings.NewReader(validCSV))
	if err != nil {
		t.Fatalf("Parse returned an error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("draft count was %d", len(drafts))
	}
	draft := drafts[0]
	if draft.SchemaVersion != 1 || draft.ImportKey != "row-001" || draft.Project.Title != "Imported Project" {
		t.Fatalf("unexpected canonical draft: %#v", draft)
	}
	if len(draft.Participations) != 2 || draft.Participations[0].StudentID != "0123456" {
		t.Fatalf("leading-zero Student ID was not preserved: %#v", draft.Participations)
	}
	if len(draft.Classifications) != 2 || draft.Classifications[0].Key != "ai" {
		t.Fatalf("unexpected classifications: %#v", draft.Classifications)
	}
}

func TestParseCSVRejectsUnknownHeadersAndInvalidRepeatedFields(t *testing.T) {
	unknownHeader := strings.Replace(validCSV, "technologies", "unknown", 1)
	if _, err := Parse(context.Background(), "csv", strings.NewReader(unknownHeader)); err == nil || !strings.Contains(err.Error(), "headers") {
		t.Fatalf("unknown-header error was %v", err)
	}
	invalidJSON := strings.Replace(validCSV, `"[""ai""]"`, `not-json`, 1)
	if _, err := Parse(context.Background(), "csv", strings.NewReader(invalidJSON)); err == nil || !strings.Contains(err.Error(), "strict JSON array") {
		t.Fatalf("invalid repeated-field error was %v", err)
	}
}

func TestParseXLSXProducesCanonicalDraft(t *testing.T) {
	workbook := validWorkbook(t)
	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	drafts, err := Parse(context.Background(), "xlsx", bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatalf("Parse returned an error: %v", err)
	}
	if len(drafts) != 1 || len(drafts[0].Participations) != 2 || len(drafts[0].Classifications) != 2 {
		t.Fatalf("unexpected XLSX draft: %#v", drafts)
	}
	if drafts[0].Participations[0].StudentID != "0123456" {
		t.Fatalf("leading-zero Student ID was %q", drafts[0].Participations[0].StudentID)
	}
}

func TestParseXLSXRejectsFormulasAndHiddenSheets(t *testing.T) {
	formulaWorkbook := validWorkbook(t)
	if err := formulaWorkbook.SetCellFormula("Projects", "E2", "=2025+1"); err != nil {
		t.Fatalf("set formula: %v", err)
	}
	formulaBuffer, _ := formulaWorkbook.WriteToBuffer()
	if _, err := Parse(context.Background(), "xlsx", bytes.NewReader(formulaBuffer.Bytes())); err == nil || !strings.Contains(err.Error(), "formula") {
		t.Fatalf("formula error was %v", err)
	}

	hiddenWorkbook := validWorkbook(t)
	if err := hiddenWorkbook.SetSheetVisible("Participations", false); err != nil {
		t.Fatalf("hide sheet: %v", err)
	}
	hiddenBuffer, _ := hiddenWorkbook.WriteToBuffer()
	if _, err := Parse(context.Background(), "xlsx", bytes.NewReader(hiddenBuffer.Bytes())); err == nil || !strings.Contains(err.Error(), "visible") {
		t.Fatalf("hidden-sheet error was %v", err)
	}
}

func validWorkbook(t *testing.T) *excelize.File {
	t.Helper()
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", "Projects"); err != nil {
		t.Fatalf("rename sheet: %v", err)
	}
	if _, err := workbook.NewSheet("Participations"); err != nil {
		t.Fatalf("create Participations sheet: %v", err)
	}
	if _, err := workbook.NewSheet("Classifications"); err != nil {
		t.Fatalf("create Classifications sheet: %v", err)
	}
	setRows := func(sheet string, rows [][]any) {
		for rowIndex, row := range rows {
			cell, _ := excelize.CoordinatesToCellName(1, rowIndex+1)
			if err := workbook.SetSheetRow(sheet, cell, &row); err != nil {
				t.Fatalf("set %s row: %v", sheet, err)
			}
		}
	}
	setRows("Projects", [][]any{
		{"import_key", "title", "reference_code", "abstract", "academic_year", "semester", "program_key", "major_key", "course_key", "title_aliases"},
		{"row-001", "Imported Project", "REF-001", "Imported abstract", "2026", "first", "computing", "", "capstone", `["Earlier Title"]`},
	})
	setRows("Participations", [][]any{
		{"import_key", "role", "display_name", "student_id", "staff_id"},
		{"row-001", "student", "Leading Student", "0123456", ""},
		{"row-001", "advisor", "Advisor Name", "", "advisor-1"},
	})
	setRows("Classifications", [][]any{
		{"import_key", "dimension", "key"},
		{"row-001", "category", "ai"},
		{"row-001", "platform", "web"},
	})
	return workbook
}
