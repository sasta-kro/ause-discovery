package httpserver

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/auth"
	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/projectlogos"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// unavailableLogoStorage reports the controlled storage-unavailable failure
// for every open, simulating a failed provider without network access.
type unavailableLogoStorage struct {
	artifacts.LocalStorage
}

func (storage *unavailableLogoStorage) Open(ctx context.Context, storageKey string) (io.ReadSeekCloser, int64, error) {
	return nil, 0, artifacts.ErrStorageUnavailable
}

// countingLogoStorage records how often content is opened so the handler
// tests can prove stale version requests never reach the storage provider.
type countingLogoStorage struct {
	artifacts.LocalStorage
	opens int
}

func (storage *countingLogoStorage) Open(ctx context.Context, storageKey string) (io.ReadSeekCloser, int64, error) {
	storage.opens++
	return storage.LocalStorage.Open(ctx, storageKey)
}

func logoTestPNG(t *testing.T, shade uint8) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			picture.Set(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatalf("encode logo PNG: %v", err)
	}
	return buffer.Bytes()
}

func logoMultipartBody(t *testing.T, revision string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("expected_project_revision", revision)
	fileField, _ := writer.CreateFormFile("file", "logo.png")
	_, _ = fileField.Write(content)
	_ = writer.Close()
	return body, writer.FormDataContentType()
}

// loginWithCsrfThroughHandler logs in through the real handler and captures
// both the session cookie and the CSRF token issued alongside it.
func loginWithCsrfThroughHandler(t *testing.T, handler http.Handler, username, password string) (string, string) {
	t.Helper()
	cookieValue := ""
	csrfValue := ""
	login := httptest.NewRequest("POST", "/ause-discovery/api/v1/admin/auth/login", strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != 200 {
		t.Fatalf("login returned %d with body %s", loginResponse.Code, loginResponse.Body.String())
	}
	for _, cookie := range loginResponse.Header().Values("Set-Cookie") {
		if value, found := strings.CutPrefix(cookie, sessionCookieName+"="); found {
			cookieValue = sessionCookieName + "=" + strings.Split(value, ";")[0]
		}
		if value, found := strings.CutPrefix(cookie, csrfCookieName+"="); found {
			csrfValue = strings.Split(value, ";")[0]
		}
	}
	if cookieValue == "" || csrfValue == "" {
		t.Fatalf("login response set cookies but missed session or csrf values")
	}
	return cookieValue, csrfValue
}

func TestProjectLogoEndpointsServePublishAndAuthorize(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLogoHTTPTestDatabase(t, ctx, databaseURL)
	seedLogoHTTPSharedFixture(t, ctx, pool)
	publishedID := seedLogoHTTPProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000c01", "published")
	draftID := seedLogoHTTPProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000c02", "draft")
	deletedID := seedLogoHTTPProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000c03", "deleted")
	removedID := seedLogoHTTPProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000c04", "published")
	otherID := seedLogoHTTPProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000c05", "published")

	counting := &countingLogoStorage{LocalStorage: artifacts.LocalStorage{Root: t.TempDir(), MaxBytes: 1 << 20}}
	storageSet := artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: counting,
	}}
	configuration := config.Config{PublicBasePath: "/ause-discovery/", SessionIdleTTL: 30 * time.Minute, SessionAbsoluteTTL: 12 * time.Hour}
	handler := NewAPIHandler(pool, configuration, storageSet)
	authService := auth.Service{Pool: pool, SessionIdleTTL: configuration.SessionIdleTTL, SessionAbsoluteTTL: configuration.SessionAbsoluteTTL}
	seedActor, createErr := authService.CreateUser(ctx, "logo-http-seed", "Logo-Http-2026!x")
	if createErr != nil {
		t.Fatalf("create seed administrator: %v", createErr)
	}

	logoService := projectlogos.Service{Pool: pool, Storage: storageSet}
	logoBytes := logoTestPNG(t, 90)
	if _, err := logoService.Upload(ctx, seedActor, publishedID, 1, projectlogos.UploadInput{ExpectedSize: int64(len(logoBytes)), Content: bytes.NewReader(logoBytes)}); err != nil {
		t.Fatalf("seed published logo: %v", err)
	}
	if _, err := logoService.Upload(ctx, seedActor, removedID, 1, projectlogos.UploadInput{ExpectedSize: int64(len(logoBytes)), Content: bytes.NewReader(logoBytes)}); err != nil {
		t.Fatalf("seed removable logo: %v", err)
	}
	if err := logoService.Remove(ctx, seedActor, removedID, 2); err != nil {
		t.Fatalf("remove logo: %v", err)
	}

	logoPath := func(projectID uuid.UUID, version string) string {
		path := "/ause-discovery/api/v1/projects/" + projectID.String() + "/logo"
		if version != "" {
			path += "?v=" + version
		}
		return path
	}
	adminLogoPath := func(projectID uuid.UUID) string {
		return "/ause-discovery/api/v1/admin/projects/" + projectID.String() + "/logo"
	}
	get := func(path string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("GET", path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	// A published Project with the matching active revision serves the exact
	// bytes immutably and inline.
	served := get(logoPath(publishedID, "1"))
	if served.Code != 200 {
		t.Fatalf("published logo with matching version returned %d", served.Code)
	}
	if contentType := served.Header().Get("Content-Type"); contentType != "image/png" {
		t.Fatalf("logo Content-Type was %q", contentType)
	}
	if cacheControl := served.Header().Get("Cache-Control"); cacheControl != "public, max-age=31536000, immutable" {
		t.Fatalf("logo Cache-Control was %q", cacheControl)
	}
	if disposition := served.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "inline") {
		t.Fatalf("logo Content-Disposition was %q", disposition)
	}
	if body, _ := io.ReadAll(served.Body); !bytes.Equal(body, logoBytes) {
		t.Fatalf("served logo bytes differ from the upload")
	}

	for name, path := range map[string]string{
		"stale version":          logoPath(publishedID, "99"),
		"absent Project":         logoPath(uuid.MustParse("018f0000-0000-7000-8000-000000000c99"), "1"),
		"draft Project":          logoPath(draftID, "1"),
		"deleted Project":        logoPath(deletedID, "1"),
		"removed logo":           logoPath(removedID, "1"),
		"Project without a logo": logoPath(otherID, "1"),
	} {
		if response := get(path); response.Code != 404 {
			t.Fatalf("%s returned %d with body %s", name, response.Code, response.Body.String())
		}
	}
	// None of the missing or stale lookups above may reach the storage
	// provider: the expected revision is part of the metadata lookup.
	if counting.opens != 1 {
		t.Fatalf("missing and stale lookups opened storage %d times, expected only the one successful serve", counting.opens)
	}

	// The version query is required, so a request without it is a controlled
	// validation failure rather than a cacheable miss.
	if response := get(logoPath(publishedID, "")); response.Code != 400 || !strings.Contains(response.Body.String(), "validation_error") {
		t.Fatalf("logo request without version returned %d with body %s", response.Code, response.Body.String())
	}

	// Provider failure maps to the controlled 503 problem.
	unavailableSet := artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: &unavailableLogoStorage{},
	}}
	unavailableHandler := NewAPIHandler(pool, configuration, unavailableSet)
	unavailableResponse := httptest.NewRecorder()
	unavailableHandler.ServeHTTP(unavailableResponse, httptest.NewRequest("GET", logoPath(publishedID, "1"), nil))
	if unavailableResponse.Code != 503 || !strings.Contains(unavailableResponse.Body.String(), "project_logo_storage_unavailable") {
		t.Fatalf("unavailable storage returned %d with body %s", unavailableResponse.Code, unavailableResponse.Body.String())
	}

	// Unauthenticated mutations are rejected before any state change. The
	// CSRF header is present so generated parameter binding passes and the
	// session check is what fails.
	unauthenticated := httptest.NewRequest("PUT", adminLogoPath(otherID), bytes.NewReader(nil))
	unauthenticated.Header.Set("X-CSRF-Token", "unauthenticated")
	unauthenticatedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticatedResponse, unauthenticated)
	if unauthenticatedResponse.Code != 401 {
		t.Fatalf("unauthenticated logo upload returned %d", unauthenticatedResponse.Code)
	}

	if _, err := authService.CreateUser(ctx, "logo-http-admin", "Logo-Admin-2026!x"); err != nil {
		t.Fatalf("create logo administrator: %v", err)
	}
	sessionCookie, csrfToken := loginWithCsrfThroughHandler(t, handler, "logo-http-admin", "Logo-Admin-2026!x")

	put := func(revision string, content []byte, csrf string) *httptest.ResponseRecorder {
		body, contentType := logoMultipartBody(t, revision, content)
		request := httptest.NewRequest("PUT", adminLogoPath(otherID), body)
		request.Header.Set("Content-Type", contentType)
		request.Header.Set("Cookie", sessionCookie)
		if csrf != "" {
			request.Header.Set("X-CSRF-Token", csrf)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	// The transport contract rejects a missing CSRF header and the handler
	// rejects a well-formed but wrong token; neither changes state.
	if response := put("1", logoBytes, ""); response.Code != 400 || !strings.Contains(response.Body.String(), "validation_error") {
		t.Fatalf("logo upload without a CSRF header returned %d with body %s", response.Code, response.Body.String())
	}
	if response := put("1", logoBytes, "definitely-not-the-token"); response.Code != 403 || !strings.Contains(response.Body.String(), "csrf_invalid") {
		t.Fatalf("logo upload with an invalid CSRF token returned %d with body %s", response.Code, response.Body.String())
	}

	// Invalid PNG content is a controlled validation failure.
	invalid := put("1", []byte("definitely not a png"), csrfToken)
	if invalid.Code != 400 || !strings.Contains(invalid.Body.String(), "validation_error") {
		t.Fatalf("invalid PNG upload returned %d with body %s", invalid.Code, invalid.Body.String())
	}

	// A stale Project revision conflicts without changing state.
	conflict := put("99", logoBytes, csrfToken)
	if conflict.Code != 409 {
		t.Fatalf("stale revision logo upload returned %d with body %s", conflict.Code, conflict.Body.String())
	}

	// Authenticated upload succeeds. Administrator responses carry the
	// authenticated preview URL, never the public versioned URL.
	uploaded := put("1", logoTestPNG(t, 200), csrfToken)
	if uploaded.Code != 200 {
		t.Fatalf("authenticated logo upload returned %d with body %s", uploaded.Code, uploaded.Body.String())
	}
	if !strings.Contains(uploaded.Body.String(), "/ause-discovery/api/v1/admin/projects/"+otherID.String()+"/logo") {
		t.Fatalf("upload response omitted the administrator preview logo URL: %s", uploaded.Body.String())
	}
	if strings.Contains(uploaded.Body.String(), "logo?v=") {
		t.Fatalf("administrator response leaked the public versioned logo URL: %s", uploaded.Body.String())
	}
	if response := get(logoPath(otherID, "1")); response.Code != 200 {
		t.Fatalf("fresh public version returned %d", response.Code)
	}

	// A replacement strictly increases the public version: the soft-deleted
	// row consumes one revision, so the replacement serves v3 and both
	// earlier versions stop serving without opening storage.
	replacement := put("2", logoTestPNG(t, 220), csrfToken)
	if replacement.Code != 200 {
		t.Fatalf("logo replacement returned %d with body %s", replacement.Code, replacement.Body.String())
	}
	opensBeforeStale := counting.opens
	for _, version := range []string{"1", "2"} {
		if response := get(logoPath(otherID, version)); response.Code != 404 {
			t.Fatalf("stale logo version %s returned %d after replacement", version, response.Code)
		}
	}
	if counting.opens != opensBeforeStale {
		t.Fatalf("stale version requests opened storage %d times", counting.opens-opensBeforeStale)
	}
	if response := get(logoPath(otherID, "3")); response.Code != 200 {
		t.Fatalf("current logo version returned %d after replacement", response.Code)
	}

	// Authenticated removal succeeds and clears the URL.
	removeRequest := httptest.NewRequest("DELETE", adminLogoPath(otherID), strings.NewReader(`{"expected_revision":3}`))
	removeRequest.Header.Set("Content-Type", "application/json")
	removeRequest.Header.Set("Cookie", sessionCookie)
	removeRequest.Header.Set("X-CSRF-Token", csrfToken)
	removeResponse := httptest.NewRecorder()
	handler.ServeHTTP(removeResponse, removeRequest)
	if removeResponse.Code != 200 {
		t.Fatalf("authenticated logo removal returned %d with body %s", removeResponse.Code, removeResponse.Body.String())
	}
	if strings.Contains(removeResponse.Body.String(), "admin/projects/"+otherID.String()+"/logo") {
		t.Fatalf("removal response still carried a logo URL: %s", removeResponse.Body.String())
	}

	// A later re-upload never reuses an earlier public version: the removed
	// row consumed v4, so the re-added logo serves v5.
	readded := put("4", logoTestPNG(t, 240), csrfToken)
	if readded.Code != 200 {
		t.Fatalf("re-upload returned %d with body %s", readded.Code, readded.Body.String())
	}
	for _, version := range []string{"1", "2", "3", "4"} {
		if response := get(logoPath(otherID, version)); response.Code != 404 {
			t.Fatalf("earlier logo version %s returned %d after re-upload", version, response.Code)
		}
	}
	if response := get(logoPath(otherID, "5")); response.Code != 200 {
		t.Fatalf("re-added logo version returned %d, expected 5", response.Code)
	}

	// The authenticated preview endpoint serves the current logo, including
	// on draft Projects, and is never publicly cacheable. Anonymous access
	// is rejected and the public draft URL stays hidden.
	anonymousPreview := get(adminLogoPath(otherID))
	if anonymousPreview.Code != 401 {
		t.Fatalf("anonymous administrator preview returned %d", anonymousPreview.Code)
	}
	preview := httptest.NewRequest("GET", adminLogoPath(otherID), nil)
	preview.Header.Set("Cookie", sessionCookie)
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, preview)
	if previewResponse.Code != 200 || previewResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("administrator preview returned %d with cache control %q", previewResponse.Code, previewResponse.Header().Get("Cache-Control"))
	}
	draftLogo, draftErr := logoService.Upload(ctx, seedActor, draftID, 1, projectlogos.UploadInput{ExpectedSize: int64(len(logoBytes)), Content: bytes.NewReader(logoBytes)})
	if draftErr != nil {
		t.Fatalf("seed draft logo: %v", draftErr)
	}
	if response := get(logoPath(draftID, "1")); response.Code != 404 {
		t.Fatalf("public draft logo returned %d, expected 404", response.Code)
	}
	draftPreview := httptest.NewRequest("GET", adminLogoPath(draftID), nil)
	draftPreview.Header.Set("Cookie", sessionCookie)
	draftPreviewResponse := httptest.NewRecorder()
	handler.ServeHTTP(draftPreviewResponse, draftPreview)
	if draftPreviewResponse.Code != 200 || draftPreviewResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("draft administrator preview returned %d with cache control %q", draftPreviewResponse.Code, draftPreviewResponse.Header().Get("Cache-Control"))
	}
	_ = draftLogo
}

func seedLogoHTTPSharedFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	statements := []string{
		"INSERT INTO people (id, display_name, normalized_name, student_id) VALUES ('018f0000-0000-7000-8000-000000000c20', 'HTTP Logo Student', 'http logo student', '7582931')",
		"INSERT INTO people (id, display_name, normalized_name, staff_id) VALUES ('018f0000-0000-7000-8000-000000000c21', 'HTTP Logo Advisor', 'http logo advisor', 'http-logo-advisor')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000c22', 'category', 'http_logo_category', '{\"en\":\"HTTP Logo Category\"}', 1)",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000c23', 'platform', 'http_logo_platform', '{\"en\":\"HTTP Logo Platform\"}', 1)",
		"INSERT INTO programs (id, key) VALUES ('018f0000-0000-7000-8000-000000000c30', 'http_logo_program')",
		"INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000c31', '018f0000-0000-7000-8000-000000000c30', 'HTTP Logo Program', 2020)",
		"INSERT INTO courses (id, key) VALUES ('018f0000-0000-7000-8000-000000000c32', 'http_logo_course')",
		"INSERT INTO course_versions (id, course_id, label, code, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000c33', '018f0000-0000-7000-8000-000000000c32', 'HTTP Logo Course', 'HLG499', 2020)",
		"INSERT INTO course_program_versions (course_version_id, program_version_id) VALUES ('018f0000-0000-7000-8000-000000000c33', '018f0000-0000-7000-8000-000000000c31')",
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed logo HTTP shared fixture: %v", err)
		}
	}
}

func seedLogoHTTPProject(t *testing.T, ctx context.Context, pool *pgxpool.Pool, rawID, status string) uuid.UUID {
	t.Helper()
	projectID := uuid.MustParse(rawID)
	if _, err := pool.Exec(ctx, "INSERT INTO projects (id, title, abstract, academic_year, semester, program_version_id, course_version_id) VALUES ($1, 'HTTP Logo Project', 'A complete abstract for logo boundary coverage.', 2026, 'first', '018f0000-0000-7000-8000-000000000c31', '018f0000-0000-7000-8000-000000000c33')", projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO project_participations (project_id, person_id, role, position) VALUES ($1, '018f0000-0000-7000-8000-000000000c20', 'student', 0), ($1, '018f0000-0000-7000-8000-000000000c21', 'advisor', 0)", projectID); err != nil {
		t.Fatalf("seed participations: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO project_taxonomy_values (project_id, taxonomy_value_id, dimension, position) VALUES ($1, '018f0000-0000-7000-8000-000000000c22', 'category', 0), ($1, '018f0000-0000-7000-8000-000000000c23', 'platform', 0)", projectID); err != nil {
		t.Fatalf("seed taxonomy: %v", err)
	}
	switch status {
	case "published":
		if _, err := pool.Exec(ctx, "UPDATE projects SET status='published', published_at=now() WHERE id=$1", projectID); err != nil {
			t.Fatalf("publish project: %v", err)
		}
	case "deleted":
		if _, err := pool.Exec(ctx, "UPDATE projects SET status='published', published_at=now() WHERE id=$1", projectID); err != nil {
			t.Fatalf("publish project before delete: %v", err)
		}
		if _, err := pool.Exec(ctx, "UPDATE projects SET status='deleted', deleted_at=now() WHERE id=$1", projectID); err != nil {
			t.Fatalf("delete project: %v", err)
		}
	}
	return projectID
}

func createLogoHTTPTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	adminConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	adminConfig.Database = "postgres"
	adminConnection, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	databaseName := "ause_logo_http_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := adminConnection.Exec(ctx, "CREATE DATABASE "+databaseName); err != nil {
		adminConnection.Close(ctx)
		t.Fatalf("create temporary database: %v", err)
	}
	adminConnection.Close(ctx)

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse pool configuration: %v", err)
	}
	poolConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open temporary database: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		t.Fatalf("set migration dialect: %v", err)
	}
	sqlDatabase := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, sqlDatabase, "../../../migrations"); err != nil {
		sqlDatabase.Close()
		pool.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	sqlDatabase.Close()

	t.Cleanup(func() {
		pool.Close()
		connection, connectErr := pgx.ConnectConfig(ctx, adminConfig)
		if connectErr != nil {
			return
		}
		defer connection.Close(ctx)
		_, _ = connection.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseName+" WITH (FORCE)")
	})
	return pool
}

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
