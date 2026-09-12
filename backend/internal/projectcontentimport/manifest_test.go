package projectcontentimport

import (
	"os"
	"path/filepath"
	"testing"
)

const validManifest = `{
  "version": 1,
  "projects": [
    {
      "project_import_key": "sp-1703",
      "logo": {"file_path": "logos/1703.png"},
      "files": [
        {"artifact_type": "report", "display_name": "Final report", "file_path": "files/1703/final-report.pdf"}
      ]
    }
  ]
}`

func writeManifest(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestLoadManifestAcceptsValidContract(t *testing.T) {
	// Whitespace-only trailing content remains valid: the document must end
	// at EOF, and whitespace before EOF is not a second document.
	manifest, err := LoadManifest(writeManifest(t, validManifest+"\n  \n"))
	if err != nil {
		t.Fatalf("LoadManifest returned an error: %v", err)
	}
	if manifest.Version != 1 || len(manifest.Projects) != 1 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	project := manifest.Projects[0]
	if project.ProjectImportKey != "sp-1703" || project.Logo.FilePath != "logos/1703.png" || len(project.Files) != 1 {
		t.Fatalf("unexpected project: %#v", project)
	}
	if project.Files[0].OriginalFilename != "final-report.pdf" {
		t.Fatalf("original_filename default was %q", project.Files[0].OriginalFilename)
	}
}

func TestLoadManifestKeepsLinkPresenceDistinctFromEmptiness(t *testing.T) {
	omitted, err := LoadManifest(writeManifest(t, `{"version": 1, "projects": [{"project_import_key": "sp-1", "logo": {"file_path": "l.png"}}]}`))
	if err != nil {
		t.Fatalf("manifest without links returned an error: %v", err)
	}
	if omitted.Projects[0].Links != nil {
		t.Fatal("omitted links decoded as present")
	}

	empty, err := LoadManifest(writeManifest(t, `{"version": 1, "projects": [{"project_import_key": "sp-1", "links": []}]}`))
	if err != nil {
		t.Fatalf("link-only empty entry returned an error: %v", err)
	}
	if empty.Projects[0].Links == nil || len(empty.Projects[0].Links.Links) != 0 {
		t.Fatal("present empty links decoded as omitted or nonempty")
	}

	declared, err := LoadManifest(writeManifest(t, `{"version": 1, "projects": [{"project_import_key": "sp-1", "links": [{"url": "https://github.com/example/repo", "primary": true, "availability": "unverified", "checked_at": "2026-09-11T07:32:27Z"}]}]}`))
	if err != nil {
		t.Fatalf("link-only entry returned an error: %v", err)
	}
	link := declared.Projects[0].Links.Links[0]
	if link.URL != "https://github.com/example/repo" || !link.IsPrimary || link.Availability != "unverified" {
		t.Fatalf("link decoded to %#v", link)
	}

	// An explicit primary:false is present and valid at load time; whether
	// the set needs exactly one primary is the planner's contract.
	explicitFalse, err := LoadManifest(writeManifest(t, `{"version": 1, "projects": [{"project_import_key": "sp-1", "links": [{"url": "https://github.com/example/repo", "primary": false, "availability": "accessible", "checked_at": "2026-09-11T07:32:27Z"}]}]}`))
	if err != nil {
		t.Fatalf("explicit primary false returned an error: %v", err)
	}
	if explicitFalse.Projects[0].Links.Links[0].IsPrimary {
		t.Fatal("explicit primary false decoded as true")
	}
}

func TestLoadManifestRejectsInvalidContracts(t *testing.T) {
	cases := map[string]string{
		"unsupported version":      `{"version": 2, "projects": [{"project_import_key": "sp-1", "files": [{"artifact_type": "report", "display_name": "r", "file_path": "f.pdf"}]}]}`,
		"empty projects":           `{"version": 1, "projects": []}`,
		"both identifiers":         `{"version": 1, "projects": [{"project_id": "018f0000-0000-7000-8000-000000000001", "project_import_key": "sp-1", "files": [{"artifact_type": "report", "display_name": "r", "file_path": "f.pdf"}]}]}`,
		"neither identifier":       `{"version": 1, "projects": [{"files": [{"artifact_type": "report", "display_name": "r", "file_path": "f.pdf"}]}]}`,
		"duplicate projects":       `{"version": 1, "projects": [{"project_import_key": "sp-1", "files": [{"artifact_type": "report", "display_name": "r", "file_path": "f.pdf"}]}, {"project_import_key": "sp-1", "logo": {"file_path": "l.png"}}]}`,
		"duplicate by uuid":        `{"version": 1, "projects": [{"project_id": "018f0000-0000-7000-8000-000000000001", "logo": {"file_path": "l.png"}}, {"project_id": "018f0000-0000-7000-8000-000000000001", "logo": {"file_path": "m.png"}}]}`,
		"no content":               `{"version": 1, "projects": [{"project_import_key": "sp-1"}]}`,
		"blank logo path":          `{"version": 1, "projects": [{"project_import_key": "sp-1", "logo": {"file_path": "  "}}]}`,
		"blank file field":         `{"version": 1, "projects": [{"project_import_key": "sp-1", "files": [{"artifact_type": "report", "display_name": "", "file_path": "f.pdf"}]}]}`,
		"unknown field":            `{"version": 1, "projects": [{"project_import_key": "sp-1", "logo": {"file_path": "l.png", "crop": true}}]}`,
		"unknown top-level field":  `{"version": 1, "source": "extractor", "projects": [{"project_import_key": "sp-1", "logo": {"file_path": "l.png"}}]}`,
		"invalid project uuid":     `{"version": 1, "projects": [{"project_id": "not-a-uuid", "logo": {"file_path": "l.png"}}]}`,
		"null links":               `{"version": 1, "projects": [{"project_import_key": "sp-1", "links": null}]}`,
		"unknown link field":       `{"version": 1, "projects": [{"project_import_key": "sp-1", "links": [{"url": "https://github.com/example/repo", "primary": true, "availability": "accessible", "checked_at": "2026-09-11T07:32:27Z", "final_url": "https://github.com/example/repo"}]}]}`,
		"link missing url":         `{"version": 1, "projects": [{"project_import_key": "sp-1", "links": [{"primary": true, "availability": "accessible", "checked_at": "2026-09-11T07:32:27Z"}]}]}`,
		"link bad timestamp":       `{"version": 1, "projects": [{"project_import_key": "sp-1", "links": [{"url": "https://github.com/example/repo", "primary": true, "availability": "accessible", "checked_at": "not-a-timestamp"}]}]}`,
		"link omitted primary":     `{"version": 1, "projects": [{"project_import_key": "sp-1", "links": [{"url": "https://github.com/example/one", "primary": true, "availability": "accessible", "checked_at": "2026-09-11T07:32:27Z"}, {"url": "https://github.com/example/two", "availability": "accessible", "checked_at": "2026-09-11T07:32:30Z"}]}]}`,
		"link omitted url":         `{"version": 1, "projects": [{"project_import_key": "sp-1", "links": [{"primary": true, "availability": "accessible", "checked_at": "2026-09-11T07:32:27Z"}]}]}`,
		"link null object":         `{"version": 1, "projects": [{"project_import_key": "sp-1", "links": [null]}]}`,
		"second json document":     validManifest + "\n{}",
		"trailing closing brace":   validManifest + "\n}",
		"trailing closing bracket": validManifest + "\n]",
		"trailing non-json text":   validManifest + "\nextraneous",
		"trailing garbage byte":    validManifest + "\n\x00",
		"malformed json":           `{"version": 1, "projects": [`,
	}
	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadManifest(writeManifest(t, contents)); err == nil {
				t.Fatal("expected a manifest validation error")
			}
		})
	}
}

func TestBundleRootUsesManifestDirectory(t *testing.T) {
	path := writeManifest(t, validManifest)
	root, err := BundleRoot(path)
	if err != nil {
		t.Fatalf("BundleRoot returned an error: %v", err)
	}
	if filepath.Base(root) != filepath.Base(filepath.Dir(path)) {
		t.Fatalf("BundleRoot returned %q", root)
	}
}
