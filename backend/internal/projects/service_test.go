package projects

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateInputRejectsDuplicateAliasesAndParticipations(t *testing.T) {
	personID := uuid.New()
	input := Input{TitleAliases: []string{"Project", " project "}}
	if err := validateInput(input); err == nil {
		t.Fatal("expected duplicate alias rejection")
	}
	input = Input{Participations: []Participation{{PersonID: personID, Role: "student", SortOrder: 0}, {PersonID: personID, Role: "student", SortOrder: 1}}}
	if err := validateInput(input); err == nil {
		t.Fatal("expected duplicate participation rejection")
	}
}

func TestDeleteConfirmationMatchesReferenceOrTitle(t *testing.T) {
	reference := "REF-1"
	title := "Project Title"
	project := Project{ReferenceCode: &reference, Title: &title}
	if !matchesConfirmation(project, reference) || !matchesConfirmation(project, title) {
		t.Fatal("expected accepted confirmation values")
	}
	if matchesConfirmation(project, "other") {
		t.Fatal("expected nonmatching confirmation rejection")
	}
}
