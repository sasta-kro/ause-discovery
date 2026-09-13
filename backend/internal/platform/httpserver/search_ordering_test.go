package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	api "ause-discovery.local/backend/generated/api"
	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/auth"
	"ause-discovery.local/backend/internal/catalog"
	importservice "ause-discovery.local/backend/internal/imports"
	"ause-discovery.local/backend/internal/people"
	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/projectlinks"
	"ause-discovery.local/backend/internal/projectlogos"
	"ause-discovery.local/backend/internal/projects"
	searchservice "ause-discovery.local/backend/internal/search"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// orderingIndex is a minimal search Index that records the sort list it
// received and serves a stable multi-page document sequence.
type orderingIndex struct {
	mutex     sync.Mutex
	sortLists [][]string
	hits      []searchservice.Document
}

func (index *orderingIndex) Health(ctx context.Context) error { return nil }
func (index *orderingIndex) EnsureIndex(ctx context.Context, uid string) error {
	return nil
}
func (index *orderingIndex) UpsertDocuments(ctx context.Context, uid string, documents []searchservice.Document) error {
	return nil
}
func (index *orderingIndex) DeleteDocument(ctx context.Context, uid string, documentID uuid.UUID) error {
	return nil
}
func (index *orderingIndex) Stats(ctx context.Context, uid string) (searchservice.IndexStats, error) {
	return searchservice.IndexStats{NumberOfDocuments: len(index.hits)}, nil
}
func (index *orderingIndex) SwapIndexes(ctx context.Context, uid, replacement string) error {
	return nil
}
func (index *orderingIndex) Search(ctx context.Context, uid string, query searchservice.IndexQuery) (searchservice.IndexResult, error) {
	index.mutex.Lock()
	defer index.mutex.Unlock()
	sortList := append([]string{}, query.Sort...)
	index.sortLists = append(index.sortLists, sortList)
	end := query.Offset + query.Limit
	if end > len(index.hits) {
		end = len(index.hits)
	}
	return searchservice.IndexResult{Hits: index.hits[query.Offset:end], Total: len(index.hits)}, nil
}

func (index *orderingIndex) lastSort() []string {
	index.mutex.Lock()
	defer index.mutex.Unlock()
	if len(index.sortLists) == 0 {
		return nil
	}
	return index.sortLists[len(index.sortLists)-1]
}

func orderingDocument(position int) searchservice.Document {
	identifier := fmt.Sprintf("018f0000-0000-7000-8000-%012d", position)
	return searchservice.Document{
		ID: uuid.MustParse(identifier), Title: fmt.Sprintf("Ordered Project %02d", position),
		AcademicYear: 2026, Semester: "first", SemesterOrder: 1,
		Program:     searchservice.CatalogReference{ID: uuid.MustParse("018f0000-0000-7000-8000-000000000f01"), Key: "ordering_program", Label: "Ordering Program"},
		Course:      searchservice.CatalogReference{ID: uuid.MustParse("018f0000-0000-7000-8000-000000000f02"), Key: "ordering_course", Label: "Ordering Course"},
		PublishedAt: time.Now(),
	}
}

func TestSearchEndpointResolvesAcademicOrdering(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLogoHTTPTestDatabase(t, ctx, databaseURL)
	index := &orderingIndex{}
	for position := 1; position <= 5; position++ {
		index.hits = append(index.hits, orderingDocument(position))
	}
	configuration := config.Config{PublicBasePath: "/ause-discovery/", ImportTemporaryRoot: t.TempDir(), SessionIdleTTL: time.Minute, SessionAbsoluteTTL: time.Minute}
	handler := newOrderingSearchHandler(pool, configuration, index)

	cases := []struct {
		query    string
		wantSort []string
	}{
		{query: "", wantSort: []string{"academic_year:desc", "semester_order:desc", "title_sort:asc", "id:asc"}},
		{query: "?q=vision", wantSort: nil},
		{query: "?q=vision&sort=relevance", wantSort: nil},
		{query: "?sort=newest", wantSort: []string{"academic_year:desc", "semester_order:desc", "title_sort:asc", "id:asc"}},
		{query: "?sort=oldest", wantSort: []string{"academic_year:asc", "semester_order:asc", "title_sort:asc", "id:asc"}},
		{query: "?sort=title", wantSort: []string{"title_sort:asc", "academic_year:desc", "semester_order:desc", "id:asc"}},
		{query: "?q=vision&sort=newest", wantSort: []string{"academic_year:desc", "semester_order:desc", "title_sort:asc", "id:asc"}},
	}
	for _, test := range cases {
		t.Run(test.query, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", "/ause-discovery/api/v1/search"+test.query, nil))
			if response.Code != 200 {
				t.Fatalf("search %q returned %d with body %s", test.query, response.Code, response.Body.String())
			}
			got := index.lastSort()
			if fmt.Sprint(got) != fmt.Sprint(test.wantSort) {
				t.Fatalf("search %q sent sort %v, expected %v", test.query, got, test.wantSort)
			}
		})
	}

	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest("GET", "/ause-discovery/api/v1/search?sort=recent", nil))
	if invalid.Code != 400 || !strings.Contains(invalid.Body.String(), "validation_error") {
		t.Fatalf("invalid sort returned %d with body %s", invalid.Code, invalid.Body.String())
	}

	// Two pages through the real cursor keep order and contain no duplicate
	// Project IDs.
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest("GET", "/ause-discovery/api/v1/search?limit=3", nil))
	if first.Code != 200 {
		t.Fatalf("first page returned %d", first.Code)
	}
	var firstPage struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Page struct {
			NextCursor *string `json:"next_cursor"`
		} `json:"page"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstPage); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(firstPage.Items) != 3 || firstPage.Page.NextCursor == nil {
		t.Fatalf("first page held %d items with cursor %v", len(firstPage.Items), firstPage.Page.NextCursor)
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest("GET", "/ause-discovery/api/v1/search?limit=3&cursor="+*firstPage.Page.NextCursor, nil))
	if second.Code != 200 {
		t.Fatalf("second page returned %d with body %s", second.Code, second.Body.String())
	}
	var secondPage struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondPage); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	seen := map[string]bool{}
	for _, item := range firstPage.Items {
		seen[item.ID] = true
	}
	for _, item := range secondPage.Items {
		if seen[item.ID] {
			t.Fatalf("Project %s appeared on both pages", item.ID)
		}
		seen[item.ID] = true
	}
	if len(seen) != 5 {
		t.Fatalf("two pages covered %d distinct Projects, expected 5", len(seen))
	}
}

// newOrderingSearchHandler mirrors NewAPIHandler's controller wiring with
// the fake search Index injected so the public search route can be driven
// through the real generated handler.
func newOrderingSearchHandler(pool *pgxpool.Pool, configuration config.Config, index searchservice.Index) http.Handler {
	controller := Controller{
		Audit:     audit.Service{Pool: pool},
		Auth:      auth.Service{Pool: pool, SessionIdleTTL: configuration.SessionIdleTTL, SessionAbsoluteTTL: configuration.SessionAbsoluteTTL},
		Catalogs:  catalog.Service{Pool: pool},
		Artifacts: artifacts.Service{Pool: pool, Storage: orderingStorageSet(), MaxProjectBytes: configuration.MaxProjectArtifactBytes},
		Imports:   importservice.Service{Pool: pool, TemporaryRoot: configuration.ImportTemporaryRoot},
		People:    people.Service{Pool: pool},
		Logos:     projectlogos.Service{Pool: pool, Storage: orderingStorageSet()},
		Links:     projectlinks.Service{Pool: pool},
		Projects:  projects.Service{Pool: pool},
		Search:    searchservice.Service{Pool: pool, Index: index, IndexUID: configuration.MeilisearchIndex},
		Config:    configuration,
		Readiness: func(ctx context.Context) error { return CheckReadiness(ctx, pool, configuration, orderingStorageSet()) },
	}
	router := chi.NewRouter()
	router.Use(requestID, securityHeaders)
	baseURL := strings.TrimSuffix(configuration.PublicBasePath, "/") + "/api/v1"
	return api.HandlerWithOptions(&controller, api.ChiServerOptions{
		BaseRouter: router,
		BaseURL:    baseURL,
		ErrorHandlerFunc: func(writer http.ResponseWriter, request *http.Request, err error) {
			problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "Invalid request parameters.")
		},
	})
}

func orderingStorageSet() artifacts.StorageSet {
	return artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: artifacts.LocalStorage{Root: os.TempDir(), MaxBytes: 1 << 20},
	}}
}
