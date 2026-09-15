package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMeilisearchClientConfiguresMutatesAndQueriesIndex(t *testing.T) {
	var mutex sync.Mutex
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-key" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		mutex.Lock()
		requests = append(requests, request.Method+" "+request.URL.Path)
		mutex.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/health":
			_, _ = writer.Write([]byte(`{"status":"available"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/indexes/projects":
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"code":"index_not_found"}`))
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/tasks/"):
			_, _ = writer.Write([]byte(`{"status":"succeeded"}`))
		case request.Method == http.MethodPatch && request.URL.Path == "/indexes/projects/settings":
			var payload struct {
				FilterableAttributes []string `json:"filterableAttributes"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode settings payload: %v", err)
			}
			foundReferenceCode := false
			for _, attribute := range payload.FilterableAttributes {
				foundReferenceCode = foundReferenceCode || attribute == "reference_code"
			}
			if !foundReferenceCode {
				t.Error("reference_code must be filterable for exact identifier matching")
			}
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"taskUid":0}`))
		case request.Method == http.MethodPost && request.URL.Path == "/indexes/projects/search":
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode search payload: %v", err)
			}
			if payload["q"] != "archive" || payload["limit"] != float64(10) {
				t.Errorf("search payload was %#v", payload)
			}
			_, _ = writer.Write([]byte(`{"hits":[{"id":"018f0000-0000-7000-8000-000000000701","title":"Archive Project","title_sort":"archive project","title_aliases":[],"abstract":"Abstract","academic_year":2026,"semester":"first","semester_order":1,"program":{"id":"018f0000-0000-7000-8000-000000000702","key":"program","label":"Program"},"course":{"id":"018f0000-0000-7000-8000-000000000703","key":"course","label":"Course"},"program_key":"program","course_key":"course","people":[],"person_ids":[],"person_names":[],"student_ids":[],"student_names":[],"advisor_person_ids":[],"advisor_names":[],"co_advisor_names":[],"committee_names":[],"taxonomy":[],"taxonomy_keys":[],"taxonomy_labels":[],"categories":[],"platforms":[],"category_keys":[],"platform_keys":[],"domain_keys":[],"topic_keys":[],"technology_keys":[],"artifact_types":[],"artifact_count":0,"has_artifacts":false,"has_report":false,"has_slides":false,"has_source_code":false,"has_dataset":false,"published_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","revision":1}],"estimatedTotalHits":1,"facetDistribution":{"program_key":{"program":1}}}`))
		default:
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"taskUid":0}`))
		}
	}))
	defer server.Close()

	client := MeilisearchClient{BaseURL: server.URL, APIKey: "test-key", HTTPClient: server.Client(), TaskTimeout: time.Second}
	ctx := context.Background()
	if err := client.Health(ctx); err != nil {
		t.Fatalf("Health returned an error: %v", err)
	}
	if err := client.EnsureIndex(ctx, "projects"); err != nil {
		t.Fatalf("EnsureIndex returned an error: %v", err)
	}
	document := Document{ID: uuid.MustParse("018f0000-0000-7000-8000-000000000701"), Title: "Archive Project"}
	if err := client.UpsertDocuments(ctx, "projects", []Document{document}); err != nil {
		t.Fatalf("UpsertDocuments returned an error: %v", err)
	}
	if err := client.DeleteDocument(ctx, "projects", document.ID); err != nil {
		t.Fatalf("DeleteDocument returned an error: %v", err)
	}
	result, err := client.Search(ctx, "projects", IndexQuery{Query: "archive", Limit: 10, Facets: []string{"program_key"}})
	if err != nil {
		t.Fatalf("Search returned an error: %v", err)
	}
	if len(result.Hits) != 1 || result.Hits[0].Title != "Archive Project" || result.Total != 1 || result.FacetDistribution["program_key"]["program"] != 1 {
		t.Fatalf("Search returned invalid result: %#v", result)
	}

	expectedRequests := []string{"GET /health", "GET /indexes/projects", "POST /indexes", "GET /tasks/0", "PATCH /indexes/projects/settings", "GET /tasks/0", "POST /indexes/projects/documents", "GET /tasks/0", "DELETE /indexes/projects/documents/018f0000-0000-7000-8000-000000000701", "GET /tasks/0", "POST /indexes/projects/search"}
	if len(requests) != len(expectedRequests) {
		t.Fatalf("recorded requests were %#v", requests)
	}
	for index := range expectedRequests {
		if requests[index] != expectedRequests[index] {
			t.Fatalf("request %d was %q, expected %q", index, requests[index], expectedRequests[index])
		}
	}
}

func TestEnsureIndexDoesNotRecreateExistingIndex(t *testing.T) {
	createRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/indexes/projects":
			_, _ = writer.Write([]byte(`{"uid":"projects","primaryKey":"id"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/indexes":
			createRequests++
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"taskUid":1}`))
		case request.Method == http.MethodPatch && request.URL.Path == "/indexes/projects/settings":
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"taskUid":2}`))
		case request.Method == http.MethodGet && request.URL.Path == "/tasks/2":
			_, _ = writer.Write([]byte(`{"status":"succeeded"}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := MeilisearchClient{BaseURL: server.URL, HTTPClient: server.Client(), TaskTimeout: time.Second}
	if err := client.EnsureIndex(context.Background(), "projects"); err != nil {
		t.Fatalf("EnsureIndex returned an error: %v", err)
	}
	if createRequests != 0 {
		t.Fatalf("existing index was recreated %d times", createRequests)
	}
}

func TestEnsureIndexSendsExplicitFacetingSettings(t *testing.T) {
	var settingsPayload struct {
		SearchableAttributes []string `json:"searchableAttributes"`
		FilterableAttributes []string `json:"filterableAttributes"`
		SortableAttributes   []string `json:"sortableAttributes"`
		RankingRules         []string `json:"rankingRules"`
		TypoTolerance        struct {
			DisableOnAttributes []string `json:"disableOnAttributes"`
		} `json:"typoTolerance"`
		Faceting struct {
			MaxValuesPerFacet int            `json:"maxValuesPerFacet"`
			SortFacetValuesBy map[string]string `json:"sortFacetValuesBy"`
		} `json:"faceting"`
	}
	var mutex sync.Mutex
	settingsRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/indexes/projects":
			_, _ = writer.Write([]byte(`{"uid":"projects","primaryKey":"id"}`))
		case request.Method == http.MethodPatch && request.URL.Path == "/indexes/projects/settings":
			mutex.Lock()
			settingsRequests++
			mutex.Unlock()
			if err := json.NewDecoder(request.Body).Decode(&settingsPayload); err != nil {
				t.Errorf("decode settings payload: %v", err)
			}
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"taskUid":7}`))
		case request.Method == http.MethodGet && request.URL.Path == "/tasks/7":
			_, _ = writer.Write([]byte(`{"status":"succeeded"}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := MeilisearchClient{BaseURL: server.URL, HTTPClient: server.Client(), TaskTimeout: time.Second}
	if err := client.EnsureIndex(context.Background(), "projects"); err != nil {
		t.Fatalf("EnsureIndex returned an error: %v", err)
	}
	if settingsRequests != 1 {
		t.Fatalf("settings patched %d times for an existing index", settingsRequests)
	}
	// The nested faceting contract is asserted structurally: the named bound
	// replaces the engine's silent 100-value cap, Person ID facets truncate
	// by count, and everything else stays alphabetical.
	if settingsPayload.Faceting.MaxValuesPerFacet != maxFacetValues {
		t.Fatalf("maxValuesPerFacet was %d, expected %d", settingsPayload.Faceting.MaxValuesPerFacet, maxFacetValues)
	}
	if got := settingsPayload.Faceting.SortFacetValuesBy["*"]; got != "alpha" {
		t.Fatalf("default facet ordering was %q", got)
	}
	if got := settingsPayload.Faceting.SortFacetValuesBy["person_ids"]; got != "count" {
		t.Fatalf("person_ids facet ordering was %q", got)
	}
	if got := settingsPayload.Faceting.SortFacetValuesBy["advisor_person_ids"]; got != "count" {
		t.Fatalf("advisor_person_ids facet ordering was %q", got)
	}
	// The accepted settings around faceting remain unchanged.
	if len(settingsPayload.SearchableAttributes) == 0 || len(settingsPayload.FilterableAttributes) == 0 || len(settingsPayload.SortableAttributes) == 0 {
		t.Fatal("searchable, filterable, or sortable attributes were dropped")
	}
	if strings.Join(settingsPayload.RankingRules, ",") != "sort,words,typo,proximity,attribute,exactness,academic_year:desc,semester_order:desc,title_sort:asc,id:asc" {
		t.Fatalf("ranking rules were %v", settingsPayload.RankingRules)
	}
	if strings.Join(settingsPayload.TypoTolerance.DisableOnAttributes, ",") != "student_ids,reference_code" {
		t.Fatalf("typo tolerance attributes were %v", settingsPayload.TypoTolerance.DisableOnAttributes)
	}
}

func TestDeleteIndexWaitsForTaskAndTreatsMissingAsSuccess(t *testing.T) {
	var mutex sync.Mutex
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		requests = append(requests, request.Method+" "+request.URL.Path)
		mutex.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodDelete && request.URL.Path == "/indexes/waited":
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"taskUid": 41}`))
		case request.Method == http.MethodDelete && request.URL.Path == "/indexes/absent":
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"code":"index_not_found"}`))
		case request.Method == http.MethodDelete && request.URL.Path == "/indexes/enqueued-missing":
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"taskUid": 42}`))
		case request.Method == http.MethodGet && request.URL.Path == "/tasks/41":
			_, _ = writer.Write([]byte(`{"status":"succeeded"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/tasks/42":
			_, _ = writer.Write([]byte(`{"status":"failed","error":{"code":"index_not_found","message":"Index not found."}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := MeilisearchClient{BaseURL: server.URL, APIKey: "test-key", TaskTimeout: 2 * time.Second}

	if err := client.DeleteIndex(context.Background(), "waited"); err != nil {
		t.Fatalf("DeleteIndex for an accepted deletion returned an error: %v", err)
	}
	if err := client.DeleteIndex(context.Background(), "absent"); err != nil {
		t.Fatalf("DeleteIndex for a missing index returned an error: %v", err)
	}
	// The engine accepts a missing-index deletion as a task that then fails
	// with index_not_found: the end state is still a controlled success.
	if err := client.DeleteIndex(context.Background(), "enqueued-missing"); err != nil {
		t.Fatalf("DeleteIndex for an enqueued missing index returned an error: %v", err)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if fmt.Sprint(requests) != fmt.Sprint([]string{"DELETE /indexes/waited", "GET /tasks/41", "DELETE /indexes/absent", "DELETE /indexes/enqueued-missing", "GET /tasks/42"}) {
		t.Fatalf("requests were %v", requests)
	}
}
