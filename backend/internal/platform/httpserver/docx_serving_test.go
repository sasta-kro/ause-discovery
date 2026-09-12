package httpserver

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/auth"
	"ause-discovery.local/backend/internal/platform/config"
)

// docxUploadFixture builds a minimal DOCX-compatible package for serving
// tests.
func docxUploadFixture(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, body := range map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"word/document.xml":   `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"/>`,
	} {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create fixture entry: %v", err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatalf("write fixture entry: %v", err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close fixture archive: %v", err)
	}
	return buffer.Bytes()
}

func TestDOCXReportServesDownloadOnly(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLogoHTTPTestDatabase(t, ctx, databaseURL)
	seedLogoHTTPSharedFixture(t, ctx, pool)
	projectID := seedLogoHTTPProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000d01", "published")

	storageSet := artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: artifacts.LocalStorage{Root: t.TempDir(), MaxBytes: 1 << 20},
	}}
	configuration := config.Config{PublicBasePath: "/ause-discovery/", SessionIdleTTL: time.Minute, SessionAbsoluteTTL: time.Minute}
	handler := NewAPIHandler(pool, configuration, storageSet)
	authService := auth.Service{Pool: pool, SessionIdleTTL: configuration.SessionIdleTTL, SessionAbsoluteTTL: configuration.SessionAbsoluteTTL}
	actorID, createErr := authService.CreateUser(ctx, "docx-serving-admin", "Docx-Serving-2026!x")
	if createErr != nil {
		t.Fatalf("create administrator: %v", createErr)
	}

	docx := docxUploadFixture(t)
	service := artifacts.Service{Pool: pool, Storage: storageSet, MaxProjectBytes: 10 << 20}
	uploaded, err := service.Upload(ctx, actorID, projectID, 1, artifacts.UploadInput{
		ArtifactType: "report", DisplayName: "Final report", OriginalFilename: "final-report.docx",
		ExpectedSize: int64(len(docx)), Content: bytes.NewReader(docx),
	})
	if err != nil {
		t.Fatalf("DOCX report upload returned an error: %v", err)
	}
	if uploaded.Extension != "docx" || uploaded.MIMEType != "application/zip" {
		t.Fatalf("DOCX upload stored as %q %q", uploaded.Extension, uploaded.MIMEType)
	}

	base := "/ause-discovery/api/v1/artifacts/" + uploaded.ID.String()
	view := httptest.NewRequest("GET", base+"/view", nil)
	viewResponse := httptest.NewRecorder()
	handler.ServeHTTP(viewResponse, view)
	if viewResponse.Code != 404 {
		t.Fatalf("DOCX view returned %d, expected 404", viewResponse.Code)
	}

	download := httptest.NewRequest("GET", base+"/download", nil)
	downloadResponse := httptest.NewRecorder()
	handler.ServeHTTP(downloadResponse, download)
	if downloadResponse.Code != 200 {
		t.Fatalf("DOCX download returned %d with body %s", downloadResponse.Code, downloadResponse.Body.String())
	}
	if disposition := downloadResponse.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, `attachment; filename="final-report.docx"`) {
		t.Fatalf("DOCX download disposition was %q", disposition)
	}
	if body, _ := io.ReadAll(downloadResponse.Body); !bytes.Equal(body, docx) {
		t.Fatal("DOCX download served different bytes than the upload")
	}
}
