package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"ause-discovery.local/backend/internal/artifactimport"
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
			want:      artifactCommand{Kind: "seed-demo", SourceDirectory: "/bulk", ActorUsername: "admin", AllPublished: true, Workers: artifactimport.DefaultWorkers},
			valid:     true,
		},
		{
			name:      "demo apply for one Project",
			arguments: []string{"seed-demo", "--source-directory", "/bulk", "--actor-username", "admin", "--project-id", projectID, "--apply"},
			want:      artifactCommand{Kind: "seed-demo", SourceDirectory: "/bulk", ActorUsername: "admin", ProjectID: uuidPointer(projectID), Apply: true, Workers: artifactimport.DefaultWorkers},
			valid:     true,
		},
		{
			name:      "manifest dry run",
			arguments: []string{"import-manifest", "--manifest", "/bulk/manifest.csv", "--actor-username", "admin"},
			want:      artifactCommand{Kind: "import-manifest", ManifestPath: "/bulk/manifest.csv", ActorUsername: "admin", Workers: artifactimport.DefaultWorkers},
			valid:     true,
		},
		{
			name:      "demo accepts minimum workers",
			arguments: []string{"seed-demo", "--source-directory", "/bulk", "--actor-username", "admin", "--all-published", "--workers", "1"},
			want:      artifactCommand{Kind: "seed-demo", SourceDirectory: "/bulk", ActorUsername: "admin", AllPublished: true, Workers: 1},
			valid:     true,
		},
		{
			name:      "manifest accepts maximum workers",
			arguments: []string{"import-manifest", "--manifest", "/bulk/manifest.csv", "--actor-username", "admin", "--workers", "8"},
			want:      artifactCommand{Kind: "import-manifest", ManifestPath: "/bulk/manifest.csv", ActorUsername: "admin", Workers: 8},
			valid:     true,
		},
		{name: "demo rejects zero workers", arguments: []string{"seed-demo", "--source-directory", "/bulk", "--actor-username", "admin", "--all-published", "--workers", "0"}},
		{name: "demo rejects workers above eight", arguments: []string{"seed-demo", "--source-directory", "/bulk", "--actor-username", "admin", "--all-published", "--workers", "9"}},
		{name: "manifest rejects negative workers", arguments: []string{"import-manifest", "--manifest", "/bulk/manifest.csv", "--actor-username", "admin", "--workers", "-1"}},
		{name: "manifest rejects malformed workers", arguments: []string{"import-manifest", "--manifest", "/bulk/manifest.csv", "--actor-username", "admin", "--workers", "many"}},
		{name: "manifest rejects repeated workers", arguments: []string{"import-manifest", "--manifest", "/bulk/manifest.csv", "--actor-username", "admin", "--workers", "2", "--workers", "4"}},
		{name: "migrate rejects workers flag", arguments: []string{"migrate", "--workers", "4"}},
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

func TestFormatProjectProgressGroupsAndSanitizes(t *testing.T) {
	projectID := uuid.MustParse("018f0000-0000-7000-8000-0000000000ab")
	progress := artifactimport.ProjectProgress{
		Completed: 17,
		Total:     209,
		ProjectID: projectID,
		Title:     "Senior Project\nwith injected\rcontrol characters",
		Files: []artifactimport.FileOutcome{
			{ArtifactType: "report", OriginalFilename: "final-report.pdf", State: artifactimport.FileUploaded},
			{ArtifactType: "slides", OriginalFilename: "presentation-slides.pdf", State: artifactimport.FileUploaded},
			{ArtifactType: "poster", OriginalFilename: "project-poster.png", State: artifactimport.FileSkipped, SkipReason: "Project already has an active file of this type"},
			{ArtifactType: "source_code", OriginalFilename: "source-code.zip", State: artifactimport.FileFailed, Error: "upload source-code.zip: artifact storage backend is unavailable: status 503"},
		},
		Duration: 2100 * time.Millisecond,
	}
	line := formatProjectProgress(progress)
	if strings.ContainsAny(line, "\n\r") {
		t.Fatalf("progress line contained a line break: %q", line)
	}
	expected := "[17/209] Senior Project with injected control characters (" + projectID.String() + "): uploaded report (final-report.pdf), slides (presentation-slides.pdf); skipped poster (project-poster.png): Project already has an active file of this type; failed source_code (source-code.zip): upload source-code.zip: artifact storage backend is unavailable: status 503; 2.1s"
	if line != expected {
		t.Fatalf("progress line mismatch:\n got %q\nwant %q", line, expected)
	}
}

func TestFormatProjectProgressBoundsAndCoversEmpty(t *testing.T) {
	long := strings.Repeat("t", 200)
	line := formatProjectProgress(artifactimport.ProjectProgress{
		Completed: 1,
		Total:     1,
		Title:     long,
		Files:     []artifactimport.FileOutcome{{ArtifactType: "report", OriginalFilename: long, State: artifactimport.FileUploaded}},
		Duration:  time.Millisecond,
	})
	if !strings.HasSuffix(line, "s") {
		t.Fatalf("unexpected line shape: %q", line)
	}
	if strings.Contains(line, strings.Repeat("t", 121)) {
		t.Fatalf("long filename was not bounded: %q", line)
	}

	empty := formatProjectProgress(artifactimport.ProjectProgress{Completed: 1, Total: 1, Title: "Untouched", Duration: time.Millisecond})
	if !strings.Contains(empty, "no files processed") {
		t.Fatalf("empty progress line lost its outcome: %q", empty)
	}
}
