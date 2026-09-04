package main

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseSearchCommand(t *testing.T) {
	projectID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	tests := []struct {
		name      string
		arguments []string
		kind      string
		projectID uuid.UUID
		valid     bool
	}{
		{name: "rebuild", arguments: []string{"rebuild"}, kind: "rebuild", valid: true},
		{name: "project reindex", arguments: []string{"reindex-project", "--project-id", projectID.String()}, kind: "reindex-project", projectID: projectID, valid: true},
		{name: "missing project ID", arguments: []string{"reindex-project"}},
		{name: "invalid project ID", arguments: []string{"reindex-project", "--project-id", "invalid"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command, err := parseSearchCommand(test.arguments)
			if test.valid && err != nil {
				t.Fatalf("parse search command: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("expected command validation error")
			}
			if test.valid && (command.Kind != test.kind || command.ProjectID != test.projectID) {
				t.Fatalf("unexpected command: %+v", command)
			}
		})
	}
}
