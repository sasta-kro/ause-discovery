package artifactimport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestResolvesProjectIdentifiersAndRelativeFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "report.pdf"), []byte("%PDF-1.7\nreport"), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	manifest := "project_id,project_import_key,artifact_type,display_name,file_path\n" +
		"018f0000-0000-7000-8000-000000000001,,report,Final report,report.pdf\n" +
		",sp-1703,report,Final report,report.pdf\n"
	manifestPath := filepath.Join(root, "manifest.csv")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	entries, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest returned an error: %v", err)
	}
	if len(entries) != 2 || entries[0].ProjectID.String() != "018f0000-0000-7000-8000-000000000001" || entries[1].ProjectImportKey != "sp-1703" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	if entries[0].SourcePath != filepath.Join(root, "report.pdf") {
		t.Fatalf("source path was %q", entries[0].SourcePath)
	}
}

func TestLoadManifestRejectsAmbiguousProjectIdentityAndEscapingPath(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.pdf")
	if err := os.WriteFile(outside, []byte("%PDF-1.7\noutside"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	manifestPath := filepath.Join(root, "manifest.csv")
	ambiguous := "project_id,project_import_key,artifact_type,display_name,file_path\n" +
		"018f0000-0000-7000-8000-000000000001,sp-1703,report,Final report,../outside.pdf\n"
	if err := os.WriteFile(manifestPath, []byte(ambiguous), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if _, err := LoadManifest(manifestPath); err == nil {
		t.Fatal("expected ambiguous Project identity to fail")
	}

	escaping := "project_id,project_import_key,artifact_type,display_name,file_path\n" +
		"018f0000-0000-7000-8000-000000000001,,report,Final report,../outside.pdf\n"
	if err := os.WriteFile(manifestPath, []byte(escaping), 0o600); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}
	if _, err := LoadManifest(manifestPath); err == nil {
		t.Fatal("expected escaping source path to fail")
	}
}
