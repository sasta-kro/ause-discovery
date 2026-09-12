package httpserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/projectlinks"
	"github.com/google/uuid"
)

func TestPublicProjectResponseIncludesOrderedRepositoryLinks(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLogoHTTPTestDatabase(t, ctx, databaseURL)
	seedLogoHTTPSharedFixture(t, ctx, pool)
	linkedID := seedLogoHTTPProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000e40", "published")
	plainID := seedLogoHTTPProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000e41", "published")

	checked := time.Date(2026, 9, 11, 7, 32, 27, 0, time.UTC)
	links := projectlinks.Service{Pool: pool}
	actorID := uuid.MustParse("018f0000-0000-7000-8000-000000000e01")
	if _, err := pool.Exec(ctx, "INSERT INTO application_users (id, username, status) VALUES ($1, 'links-http-admin', 'active')", actorID); err != nil {
		t.Fatalf("seed links actor: %v", err)
	}
	if _, err := links.Replace(ctx, actorID, linkedID, 1, []projectlinks.Input{
		{URL: "https://github.com/example/one", IsPrimary: true, Availability: projectlinks.AvailabilityAccessible, CheckedAt: checked},
		{URL: "https://github.com/example/two", IsPrimary: false, Availability: projectlinks.AvailabilityNotAccessible, CheckedAt: checked},
	}); err != nil {
		t.Fatalf("seed repository links: %v", err)
	}

	configuration := config.Config{PublicBasePath: "/ause-discovery/", SessionIdleTTL: time.Minute, SessionAbsoluteTTL: time.Minute}
	handler := NewAPIHandler(pool, configuration, artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: artifacts.LocalStorage{Root: t.TempDir()},
	}})
	get := func(projectID uuid.UUID) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/ause-discovery/api/v1/projects/"+projectID.String(), nil))
		return response
	}

	linked := get(linkedID)
	if linked.Code != 200 {
		t.Fatalf("linked Project returned %d with body %s", linked.Code, linked.Body.String())
	}
	var linkedBody struct {
		RepositoryLinks []struct {
			URL          string `json:"url"`
			Primary      bool   `json:"primary"`
			Availability string `json:"availability"`
		} `json:"repository_links"`
	}
	if err := json.Unmarshal(linked.Body.Bytes(), &linkedBody); err != nil {
		t.Fatalf("decode linked Project: %v", err)
	}
	if len(linkedBody.RepositoryLinks) != 2 || linkedBody.RepositoryLinks[0].URL != "https://github.com/example/one" || !linkedBody.RepositoryLinks[0].Primary || linkedBody.RepositoryLinks[1].Availability != "not_accessible" {
		t.Fatalf("linked Project carried %#v", linkedBody.RepositoryLinks)
	}

	plain := get(plainID)
	if plain.Code != 200 {
		t.Fatalf("plain Project returned %d", plain.Code)
	}
	if !strings.Contains(plain.Body.String(), `"repository_links":[]`) {
		t.Fatalf("plain Project omitted the always-present empty array: %s", plain.Body.String())
	}
}
