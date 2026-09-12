package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureStorageDirectoriesDoesNotRecreateMissingDirectories(t *testing.T) {
	root := t.TempDir()
	configuration := Config{ArtifactRoot: filepath.Join(root, "artifacts"), ImportTemporaryRoot: filepath.Join(root, "imports")}
	if err := configuration.EnsureStorageDirectories(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(configuration.ArtifactRoot); err != nil {
		t.Fatal(err)
	}
	// Startup initialization may create required directories; only the
	// readiness probes must never recreate them.
	if err := configuration.EnsureStorageDirectories(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(configuration.ArtifactRoot); err != nil {
		t.Fatalf("startup did not recreate missing storage: %v", err)
	}
}

func TestImportTemporaryDirectoryProbeStaysIndependentOfArtifactStorage(t *testing.T) {
	root := t.TempDir()
	configuration := Config{ArtifactRoot: filepath.Join(root, "artifacts"), ImportTemporaryRoot: filepath.Join(root, "imports")}
	if err := configuration.EnsureStorageDirectories(); err != nil {
		t.Fatal(err)
	}
	if err := configuration.CheckImportTemporaryDirectory(); err != nil {
		t.Fatal(err)
	}
	// The artifact root is provider-owned now: removing it must not fail the
	// configuration-level probe.
	if err := os.Remove(configuration.ArtifactRoot); err != nil {
		t.Fatal(err)
	}
	if err := configuration.CheckImportTemporaryDirectory(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(configuration.ImportTemporaryRoot); err != nil {
		t.Fatal(err)
	}
	if err := configuration.CheckImportTemporaryDirectory(); err == nil {
		t.Fatal("missing import directory reported ready")
	}
	if _, err := os.Stat(configuration.ImportTemporaryRoot); !os.IsNotExist(err) {
		t.Fatal("probe recreated missing import directory")
	}
}

func TestNormalizePublicBasePath(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "root", input: "/", expected: "/"},
		{name: "default path", input: "/ause-discovery/", expected: "/ause-discovery/"},
		{name: "duplicate separators", input: "//ause-discovery//admin//", expected: "/ause-discovery/admin/"},
		{name: "missing separators", input: "ause-discovery", expected: "/ause-discovery/"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := NormalizePublicBasePath(testCase.input)
			if err != nil {
				t.Fatalf("NormalizePublicBasePath returned an error: %v", err)
			}
			if actual != testCase.expected {
				t.Fatalf("NormalizePublicBasePath = %q, expected %q", actual, testCase.expected)
			}
		})
	}
}

func TestNormalizePublicBasePathRejectsQueryAndFragment(t *testing.T) {
	for _, input := range []string{"/ause-discovery/?preview=1", "/ause-discovery/#section"} {
		if _, err := NormalizePublicBasePath(input); err == nil {
			t.Fatalf("NormalizePublicBasePath(%q) returned no error", input)
		}
	}
}
