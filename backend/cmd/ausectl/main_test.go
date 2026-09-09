package main

import (
	"reflect"
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

func TestParseArtifactCommand(t *testing.T) {
	projectID := "018f0000-0000-7000-8000-000000000001"
	tests := []struct {
		name      string
		arguments []string
		want      artifactCommand
		valid     bool
	}{
		{
			name:      "demo dry run for all published Projects",
			arguments: []string{"seed-demo", "--source-directory", "/bulk", "--actor-username", "admin", "--all-published"},
			want:      artifactCommand{Kind: "seed-demo", SourceDirectory: "/bulk", ActorUsername: "admin", AllPublished: true},
			valid:     true,
		},
		{
			name:      "demo apply for one Project",
			arguments: []string{"seed-demo", "--source-directory", "/bulk", "--actor-username", "admin", "--project-id", projectID, "--apply"},
			want:      artifactCommand{Kind: "seed-demo", SourceDirectory: "/bulk", ActorUsername: "admin", ProjectID: uuidPointer(projectID), Apply: true},
			valid:     true,
		},
		{
			name:      "manifest dry run",
			arguments: []string{"import-manifest", "--manifest", "/bulk/manifest.csv", "--actor-username", "admin"},
			want:      artifactCommand{Kind: "import-manifest", ManifestPath: "/bulk/manifest.csv", ActorUsername: "admin"},
			valid:     true,
		},
		{name: "demo requires explicit scope", arguments: []string{"seed-demo", "--source-directory", "/bulk", "--actor-username", "admin"}},
		{name: "demo rejects overlapping scopes", arguments: []string{"seed-demo", "--source-directory", "/bulk", "--actor-username", "admin", "--all-published", "--project-id", projectID}},
		{name: "manifest requires actor", arguments: []string{"import-manifest", "--manifest", "/bulk/manifest.csv"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command, err := parseArtifactCommand(test.arguments)
			if test.valid && err != nil {
				t.Fatalf("parse Artifact command: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("expected command validation error")
			}
			if test.valid && !reflect.DeepEqual(command, test.want) {
				t.Fatalf("unexpected command: %+v", command)
			}
		})
	}
}

func uuidPointer(value string) *uuid.UUID {
	id := uuid.MustParse(value)
	return &id
}
