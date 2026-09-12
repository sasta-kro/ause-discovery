package projectcontentimport

import (
	"context"
	"fmt"
	"os"
	"testing"

	"ause-discovery.local/backend/internal/artifactimport"
	"ause-discovery.local/backend/internal/artifacts"
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
	seedBundleEvidenceFixture(t, ctx, pool, adminUser)

	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("load extractor bundle manifest: %v", err)
	}
	if len(manifest.Projects) == 0 {
		t.Fatal("extractor bundle manifest held no Projects")
	}
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

func seedBundleEvidenceFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, adminUser string) {
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
	manifestProjects := loadBundleEvidenceKeys(t)
	for index, importKey := range manifestProjects {
		projectID := uuid.New()
		title := fmt.Sprintf("Bundle Project %d", index+1)
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, title, abstract, academic_year, semester, program_version_id, course_version_id, extra_metadata) VALUES ($1, $2, 'Bundle evidence abstract.', 2020, 'first', '018f0000-0000-7000-8000-000000000b52', '018f0000-0000-7000-8000-000000000b54', jsonb_build_object('import_key',$3::text))`, projectID, title, importKey); err != nil {
			t.Fatalf("seed bundle Project %s: %v", importKey, err)
		}
	}
}

// loadBundleEvidenceKeys reads the manifest a second time through the strict
// loader so the seeded Projects exactly match what the planner will resolve.
func loadBundleEvidenceKeys(t *testing.T) []string {
	t.Helper()
	manifest, err := LoadManifest(os.Getenv("AUSE_PROJECT_CONTENT_MANIFEST"))
	if err != nil {
		t.Fatalf("load extractor bundle manifest for seeding: %v", err)
	}
	keys := make([]string, 0, len(manifest.Projects))
	for index := range manifest.Projects {
		if manifest.Projects[index].ProjectImportKey == "" {
			t.Fatalf("bundle entry %d carried no project_import_key", index+1)
		}
		keys = append(keys, manifest.Projects[index].ProjectImportKey)
	}
	return keys
}
