package projectcontentimport

import (
	"context"
	"fmt"
	"os"
	"testing"

	"ause-discovery.local/backend/internal/artifactimport"
	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/projectlinks"
	"ause-discovery.local/backend/internal/projectlogos"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestExtractorLogoBundlePlansCompletely proves the real extractor output
// bundle plans end to end: every manifest entry resolves to a Project,
// validates, and digests. It runs only when AUSE_PROJECT_CONTENT_MANIFEST
// points at a generated Project Content manifest, because the extractor
// bundle is not part of the repository.
func TestExtractorLogoBundlePlansCompletely(t *testing.T) {
	manifestPath := os.Getenv("AUSE_PROJECT_CONTENT_MANIFEST")
	if manifestPath == "" {
		t.Skip("AUSE_PROJECT_CONTENT_MANIFEST is required for extractor bundle evidence")
	}
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createContentTestDatabase(t, ctx, databaseURL)
	adminUser := "bundle-evidence-admin"

	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("load extractor bundle manifest: %v", err)
	}
	if len(manifest.Projects) == 0 {
		t.Fatal("extractor bundle manifest held no Projects")
	}
	keys := make([]string, 0, len(manifest.Projects))
	for index := range manifest.Projects {
		keys = append(keys, manifest.Projects[index].ProjectImportKey)
	}
	seedBundleEvidenceFixture(t, ctx, pool, adminUser, keys)
	bundleRoot, err := BundleRoot(manifestPath)
	if err != nil {
		t.Fatalf("resolve bundle root: %v", err)
	}
	storageSet := artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: artifacts.LocalStorage{Root: t.TempDir(), MaxBytes: 4 << 20},
	}}
	service := Service{
		Pool:  pool,
		Logos: projectlogos.Service{Pool: pool, Storage: storageSet},
		Files: artifactimport.Service{
			Pool:             pool,
			Artifacts:        artifacts.Service{Pool: pool, Storage: storageSet, MaxProjectBytes: 64 << 20},
			MaxArtifactBytes: 4 << 20,
		},
		Links: projectlinks.Service{Pool: pool},
	}
	result, err := service.Run(ctx, manifest, bundleRoot, Options{ActorUsername: adminUser})
	if err != nil {
		t.Fatalf("plan extractor bundle: %v", err)
	}
	if result.ProjectCount != len(manifest.Projects) {
		t.Fatalf("planned %d Projects for %d manifest entries", result.ProjectCount, len(manifest.Projects))
	}
	if result.LogoUploads != len(manifest.Projects) || result.LogoReplacements != 0 || result.FileUploads != 0 || result.UnchangedSkips != 0 {
		t.Fatalf("planned %+v, expected one fresh logo upload per Project", result)
	}
	t.Logf("planned the extractor bundle: %d Projects, %d logo uploads, %d bytes", result.ProjectCount, result.LogoUploads, result.TotalBytes)
}

func seedBundleEvidenceFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, adminUser string, manifestProjects []string) {
	t.Helper()
	statements := []string{
		fmt.Sprintf("INSERT INTO application_users (id, username, status) VALUES ('018f0000-0000-7000-8000-000000000b50', '%s', 'active')", adminUser),
		"INSERT INTO programs (id, key) VALUES ('018f0000-0000-7000-8000-000000000b51', 'bundle_program')",
		"INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000b52', '018f0000-0000-7000-8000-000000000b51', 'Bundle Program', 1990)",
		"INSERT INTO courses (id, key) VALUES ('018f0000-0000-7000-8000-000000000b53', 'bundle_course')",
		"INSERT INTO course_versions (id, course_id, label, code, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000b54', '018f0000-0000-7000-8000-000000000b53', 'Bundle Course', 'BND499', 1990)",
		"INSERT INTO course_program_versions (course_version_id, program_version_id) VALUES ('018f0000-0000-7000-8000-000000000b54', '018f0000-0000-7000-8000-000000000b52')",
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed bundle evidence fixture: %v", err)
		}
	}
	for index, importKey := range manifestProjects {
		projectID := uuid.New()
		title := fmt.Sprintf("Bundle Project %d", index+1)
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, title, abstract, academic_year, semester, program_version_id, course_version_id, extra_metadata) VALUES ($1, $2, 'Bundle evidence abstract.', 2020, 'first', '018f0000-0000-7000-8000-000000000b52', '018f0000-0000-7000-8000-000000000b54', jsonb_build_object('import_key',$3::text))`, projectID, title, importKey); err != nil {
			t.Fatalf("seed bundle Project %s: %v", importKey, err)
		}
	}
}

// TestExtractorLinkBundlePlansCompletely proves the real extractor-derived
// repository-link manifest plans end to end: every entry resolves, every
// link set validates, and no third-party reference appears. It runs only
// when AUSE_PROJECT_LINK_MANIFEST points at a generated link manifest,
// because the extractor bundle is not part of the repository.
func TestExtractorLinkBundlePlansCompletely(t *testing.T) {
	manifestPath := os.Getenv("AUSE_PROJECT_LINK_MANIFEST")
	if manifestPath == "" {
		t.Skip("AUSE_PROJECT_LINK_MANIFEST is required for extractor link bundle evidence")
	}
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createContentTestDatabase(t, ctx, databaseURL)
	adminUser := "link-evidence-admin"

	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("load extractor link manifest: %v", err)
	}
	declared := 0
	keys := make([]string, 0, len(manifest.Projects))
	for index := range manifest.Projects {
		if manifest.Projects[index].Links == nil {
			t.Fatalf("entry %d omitted its declared link set", index+1)
		}
		declared += len(manifest.Projects[index].Links.Links)
		keys = append(keys, manifest.Projects[index].ProjectImportKey)
	}
	seedBundleEvidenceFixture(t, ctx, pool, adminUser, keys)
	bundleRoot, err := BundleRoot(manifestPath)
	if err != nil {
		t.Fatalf("resolve bundle root: %v", err)
	}
	service := Service{
		Pool:  pool,
		Logos: projectlogos.Service{Pool: pool, Storage: artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{artifacts.BackendLocal: artifacts.LocalStorage{Root: t.TempDir(), MaxBytes: 4 << 20}}}},
		Files: artifactimport.Service{Pool: pool, Artifacts: artifacts.Service{Pool: pool, MaxProjectBytes: 64 << 20}, MaxArtifactBytes: 4 << 20},
		Links: projectlinks.Service{Pool: pool},
	}
	result, err := service.Run(ctx, manifest, bundleRoot, Options{ActorUsername: adminUser})
	if err != nil {
		t.Fatalf("plan extractor link bundle: %v", err)
	}
	if result.ProjectCount != len(manifest.Projects) || result.DeclaredLinks != declared || result.LinkSetsReplaced != len(manifest.Projects) || result.LinkSetsUnchanged != 0 {
		t.Fatalf("planned %+v for %d entries and %d declared links", result, len(manifest.Projects), declared)
	}
	t.Logf("planned the extractor link bundle: %d Projects, %d links", result.ProjectCount, result.DeclaredLinks)
}
