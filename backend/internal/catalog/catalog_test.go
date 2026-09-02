package catalog

import (
	"strings"
	"testing"
)

func TestSourceValidateRejectsInvalidStableKey(t *testing.T) {
	source := validSource()
	source.Academic.Programs[0].Key = "Computer Science"

	err := source.Validate()
	if err == nil || !strings.Contains(err.Error(), "lower-case snake-case") {
		t.Fatalf("Source.Validate() error = %v, expected stable key validation error", err)
	}
}

func TestSourceValidateRejectsOverlappingValidity(t *testing.T) {
	source := validSource()
	source.Academic.Programs[0].Versions = append(source.Academic.Programs[0].Versions, ProgramVersion{
		ID:            "018f0000-0000-7000-8000-000000000012",
		Label:         "Computer Science Updated",
		ValidFromYear: 2020,
	})

	err := source.Validate()
	if err == nil || !strings.Contains(err.Error(), "validity overlaps") {
		t.Fatalf("Source.Validate() error = %v, expected overlapping validity error", err)
	}
}

func TestSourceValidateRejectsUnknownCourseProgramVersion(t *testing.T) {
	source := validSource()
	source.Academic.Courses[0].Versions[0].ProgramVersionIDs = []string{"018f0000-0000-7000-8000-000000000099"}

	err := source.Validate()
	if err == nil || !strings.Contains(err.Error(), "references unknown program version") {
		t.Fatalf("Source.Validate() error = %v, expected broken course program reference error", err)
	}
}

func TestSourceValidateRejectsTaxonomyWithoutEnglishLabel(t *testing.T) {
	source := validSource()
	source.Taxonomy.Values[0].Labels = map[string]string{"th": "ซอฟต์แวร์"}

	err := source.Validate()
	if err == nil || !strings.Contains(err.Error(), "labels.en is required") {
		t.Fatalf("Source.Validate() error = %v, expected English label error", err)
	}
}

func TestSourceValidateRejectsNonCanonicalUUID(t *testing.T) {
	source := validSource()
	source.Academic.Programs[0].ID = "not-a-uuid"

	err := source.Validate()
	if err == nil || !strings.Contains(err.Error(), "canonical UUID") {
		t.Fatalf("Source.Validate() error = %v, expected UUID error", err)
	}
}

func validSource() Source {
	return Source{
		Academic: AcademicCatalog{
			Programs: []Program{{
				ID:  "018f0000-0000-7000-8000-000000000001",
				Key: "computer_science",
				Versions: []ProgramVersion{{
					ID:            "018f0000-0000-7000-8000-000000000011",
					Label:         "Computer Science",
					ValidFromYear: 2020,
				}},
			}},
			Courses: []Course{{
				ID:  "018f0000-0000-7000-8000-000000000021",
				Key: "senior_project",
				Versions: []CourseVersion{{
					ID:                "018f0000-0000-7000-8000-000000000031",
					Label:             "Senior Project",
					Code:              "CS499",
					ValidFromYear:     2020,
					ProgramVersionIDs: []string{"018f0000-0000-7000-8000-000000000011"},
				}},
			}},
		},
		Taxonomy: TaxonomyCatalog{Values: []TaxonomyValue{{
			ID:        "018f0000-0000-7000-8000-000000000101",
			Dimension: "category",
			Key:       "software_application",
			Labels:    map[string]string{"en": "Software / Application"},
		}}},
	}
}
