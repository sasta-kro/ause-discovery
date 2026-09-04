package httpserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	api "ause-discovery.local/backend/generated/api"
	"ause-discovery.local/backend/internal/auth"
	"ause-discovery.local/backend/internal/people"
	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/projects"
)

func TestAdminListsPageThroughCursorsAndRejectMalformedOnes(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createAuditHTTPTestDatabase(t, ctx, databaseURL)
	configuration := config.Config{PublicBasePath: "/ause-discovery/", SessionIdleTTL: 30 * time.Minute, SessionAbsoluteTTL: 12 * time.Hour}
	handler := NewAPIHandler(pool, configuration)

	authService := auth.Service{Pool: pool, SessionIdleTTL: configuration.SessionIdleTTL, SessionAbsoluteTTL: configuration.SessionAbsoluteTTL}
	adminID, err := authService.CreateUser(ctx, "list-http-admin", "List-Http-2026!x")
	if err != nil {
		t.Fatalf("create test administrator: %v", err)
	}
	sessionCookie := loginThroughHandler(t, handler, "list-http-admin", "List-Http-2026!x")

	peopleService := people.Service{Pool: pool}
	projectService := projects.Service{Pool: pool}
	for _, name := range []string{"List Person Alpha", "List Person Beta", "List Person Gamma"} {
		if _, err := peopleService.Create(ctx, adminID, people.Input{DisplayName: name}); err != nil {
			t.Fatalf("create Person %q: %v", name, err)
		}
	}
	for _, title := range []string{"Cursor Round Trip Report", "Unrelated Notebook", "Round Trip Slides"} {
		projectTitle := title
		if _, err := projectService.Create(ctx, adminID, projects.Input{Title: &projectTitle, ReferenceCode: nil}); err != nil {
			t.Fatalf("create Project %q: %v", title, err)
		}
	}

	get := func(path string) (int, []byte) {
		request := httptest.NewRequest("GET", path, nil)
		request.Header.Set("Cookie", sessionCookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response.Code, response.Body.Bytes()
	}

	status, body := get("/ause-discovery/api/v1/admin/people?limit=1")
	if status != 200 {
		t.Fatalf("first People page returned %d with body %s", status, body)
	}
	var peoplePage api.AdminPersonPage
	if err := json.Unmarshal(body, &peoplePage); err != nil {
		t.Fatalf("decode People page: %v", err)
	}
	if len(peoplePage.Items) != 1 || peoplePage.Page.NextCursor == nil {
		t.Fatalf("first People page held %d items with cursor %v", len(peoplePage.Items), peoplePage.Page.NextCursor)
	}
	firstPersonID := peoplePage.Items[0].Id.String()
	status, body = get("/ause-discovery/api/v1/admin/people?limit=1&cursor=" + *peoplePage.Page.NextCursor)
	if status != 200 {
		t.Fatalf("second People page returned %d with body %s", status, body)
	}
	if err := json.Unmarshal(body, &peoplePage); err != nil {
		t.Fatalf("decode second People page: %v", err)
	}
	if len(peoplePage.Items) != 1 || peoplePage.Page.NextCursor == nil {
		t.Fatalf("second People page held %d items with cursor %v", len(peoplePage.Items), peoplePage.Page.NextCursor)
	}
	if peoplePage.Items[0].Id.String() == firstPersonID {
		t.Fatalf("Person %s appeared on both pages", firstPersonID)
	}
	status, body = get("/ause-discovery/api/v1/admin/people?cursor=not-a-cursor")
	if status != 400 {
		t.Fatalf("malformed People cursor returned %d with body %s", status, body)
	}
	var problem api.Problem
	if err := json.Unmarshal(body, &problem); err != nil || problem.Code != "validation_error" {
		t.Fatalf("malformed People cursor produced %s", body)
	}

	status, body = get("/ause-discovery/api/v1/admin/projects?limit=2")
	if status != 200 {
		t.Fatalf("first Projects page returned %d with body %s", status, body)
	}
	var projectPage api.AdminProjectPage
	if err := json.Unmarshal(body, &projectPage); err != nil {
		t.Fatalf("decode Projects page: %v", err)
	}
	if len(projectPage.Items) != 2 || projectPage.Page.NextCursor == nil {
		t.Fatalf("first Projects page held %d items with cursor %v", len(projectPage.Items), projectPage.Page.NextCursor)
	}
	firstPageIDs := []string{projectPage.Items[0].Id.String(), projectPage.Items[1].Id.String()}
	status, body = get("/ause-discovery/api/v1/admin/projects?limit=2&cursor=" + *projectPage.Page.NextCursor)
	if status != 200 {
		t.Fatalf("second Projects page returned %d with body %s", status, body)
	}
	projectPage = api.AdminProjectPage{}
	if err := json.Unmarshal(body, &projectPage); err != nil {
		t.Fatalf("decode second Projects page: %v", err)
	}
	if len(projectPage.Items) != 1 || projectPage.Page.NextCursor != nil {
		t.Fatalf("final Projects page held %d items with cursor %v", len(projectPage.Items), projectPage.Page.NextCursor)
	}
	for _, id := range firstPageIDs {
		if projectPage.Items[0].Id.String() == id {
			t.Fatalf("Project %s appeared on both pages", id)
		}
	}

	status, body = get("/ause-discovery/api/v1/admin/projects?q=round+trip")
	if status != 200 {
		t.Fatalf("filtered Projects page returned %d with body %s", status, body)
	}
	projectPage = api.AdminProjectPage{}
	if err := json.Unmarshal(body, &projectPage); err != nil {
		t.Fatalf("decode filtered Projects page: %v", err)
	}
	if len(projectPage.Items) != 2 {
		t.Fatalf("filtered Projects page held %d items, expected the two round-trip Projects", len(projectPage.Items))
	}
	status, body = get("/ause-discovery/api/v1/admin/projects?cursor=also-not-a-cursor")
	if status != 400 {
		t.Fatalf("malformed Projects cursor returned %d with body %s", status, body)
	}
	if err := json.Unmarshal(body, &problem); err != nil || problem.Code != "validation_error" {
		t.Fatalf("malformed Projects cursor produced %s", body)
	}
}
