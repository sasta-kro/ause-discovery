package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type IndexQuery struct {
	Query   string
	Filters []string
	Sort    []string
	Offset  int
	Limit   int
	Facets  []string
}

type IndexResult struct {
	Hits              []Document
	Total             int
	FacetDistribution map[string]map[string]int
}

type IndexStats struct {
	NumberOfDocuments int `json:"numberOfDocuments"`
}

type Index interface {
	Health(context.Context) error
	EnsureIndex(context.Context, string) error
	UpsertDocuments(context.Context, string, []Document) error
	DeleteDocument(context.Context, string, uuid.UUID) error
	Search(context.Context, string, IndexQuery) (IndexResult, error)
	Stats(context.Context, string) (IndexStats, error)
	SwapIndexes(context.Context, string, string) error
}

type MeilisearchClient struct {
	BaseURL     string
	APIKey      string
	HTTPClient  *http.Client
	TaskTimeout time.Duration
}

// maxFacetValues is the explicit source-controlled faceting distribution
// bound. The engine default of 100 silently truncated the People facet, whose
// values are Person UUIDs, so every participating Person below this bound
// stays reachable in the contextual facet distribution.
const maxFacetValues = 1000

func (client MeilisearchClient) Health(ctx context.Context) error {
	response, err := client.request(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return client.responseError(response)
	}
	return nil
}

func (client MeilisearchClient) EnsureIndex(ctx context.Context, indexUID string) error {
	response, err := client.request(ctx, http.MethodGet, "/indexes/"+url.PathEscape(indexUID), nil)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusOK {
		response.Body.Close()
	} else if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		response, err = client.request(ctx, http.MethodPost, "/indexes", map[string]any{"uid": indexUID, "primaryKey": "id"})
		if err != nil {
			return err
		}
		if err := client.waitForAcceptedTask(ctx, response); err != nil {
			return err
		}
	} else {
		err := client.responseError(response)
		response.Body.Close()
		return err
	}
	// The query-time sort rule leads so an explicit user selection is
	// authoritative. Every lexical rule, including exactness, precedes the
	// academic-newest custom rules that order placeholder searches and break
	// relevance ties. The custom rules end with the unique Project ID so any
	// order they fully determine is stable across offset pages. Relevance
	// mode sends no query-time sort; see ResolveOrder.
	//
	// The faceting settings are explicit: the distribution bound raises the
	// engine's silent 100-value cap that truncated the People facet, and the
	// Person ID facets sort by count so a future truncation above the bound
	// keeps the most frequent matching People instead of an accidental UUID
	// lexicographic subset. All other facets keep alphabetical order.
	settings := map[string]any{
		"searchableAttributes": []string{"student_ids", "reference_code", "title", "title_aliases", "student_names", "advisor_names", "co_advisor_names", "committee_names", "taxonomy_labels", "taxonomy_keys", "program.label", "major.label", "course.label", "abstract"},
		"filterableAttributes": []string{"reference_code", "academic_year", "semester", "program_key", "major_key", "course_key", "person_ids", "student_ids", "advisor_person_ids", "category_keys", "platform_keys", "domain_keys", "topic_keys", "technology_keys", "artifact_types", "has_artifacts", "has_report", "has_slides", "has_source_code", "has_dataset"},
		"sortableAttributes":   []string{"academic_year", "semester_order", "title_sort", "id", "published_at", "updated_at"},
		"rankingRules":         []string{"sort", "words", "typo", "proximity", "attribute", "exactness", "academic_year:desc", "semester_order:desc", "title_sort:asc", "id:asc"},
		"typoTolerance":        map[string]any{"disableOnAttributes": []string{"student_ids", "reference_code"}},
		"faceting": map[string]any{
			"maxValuesPerFacet": maxFacetValues,
			"sortFacetValuesBy": map[string]any{
				"*":                   "alpha",
				"person_ids":          "count",
				"advisor_person_ids": "count",
			},
		},
	}
	response, err = client.request(ctx, http.MethodPatch, "/indexes/"+url.PathEscape(indexUID)+"/settings", settings)
	if err != nil {
		return err
	}
	return client.waitForAcceptedTask(ctx, response)
}

func (client MeilisearchClient) UpsertDocuments(ctx context.Context, indexUID string, documents []Document) error {
	if len(documents) == 0 {
		return nil
	}
	response, err := client.request(ctx, http.MethodPost, "/indexes/"+url.PathEscape(indexUID)+"/documents?primaryKey=id", documents)
	if err != nil {
		return err
	}
	return client.waitForAcceptedTask(ctx, response)
}

// taskError marks a completed-but-failed engine task and keeps the engine
// error code branchable through errors.As.
type taskError struct {
	code    string
	message string
}

func (err *taskError) Error() string { return "Meilisearch task " + err.code + ": " + err.message }

// taskErrorCode reports the engine error code of a failed task, or an empty
// string for other errors.
func taskErrorCode(err error) string {
	var taskErr *taskError
	if errors.As(err, &taskErr) {
		return taskErr.code
	}
	return ""
}

// DeleteIndex removes one index and waits for the accepted deletion task so
// callers can rely on the index actually being gone. A missing index is a
// controlled success whether the engine answers 404 directly or enqueues a
// deletion task that fails with index_not_found. It exists for disposable
// test indexes and operator recovery; the application's own lifecycle never
// deletes the logical index.
func (client MeilisearchClient) DeleteIndex(ctx context.Context, indexUID string) error {
	response, err := client.request(ctx, http.MethodDelete, "/indexes/"+url.PathEscape(indexUID), nil)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusNotFound {
		defer response.Body.Close()
		return nil
	}
	err = client.waitForAcceptedTask(ctx, response)
	if code := taskErrorCode(err); code == "index_not_found" {
		return nil
	}
	return err
}

func (client MeilisearchClient) DeleteDocument(ctx context.Context, indexUID string, documentID uuid.UUID) error {
	response, err := client.request(ctx, http.MethodDelete, "/indexes/"+url.PathEscape(indexUID)+"/documents/"+url.PathEscape(documentID.String()), nil)
	if err != nil {
		return err
	}
	return client.waitForAcceptedTask(ctx, response)
}

func (client MeilisearchClient) Search(ctx context.Context, indexUID string, query IndexQuery) (IndexResult, error) {
	payload := map[string]any{"q": query.Query, "offset": query.Offset, "limit": query.Limit, "facets": query.Facets}
	if len(query.Filters) > 0 {
		payload["filter"] = query.Filters
	}
	if len(query.Sort) > 0 {
		payload["sort"] = query.Sort
	}
	response, err := client.request(ctx, http.MethodPost, "/indexes/"+url.PathEscape(indexUID)+"/search", payload)
	if err != nil {
		return IndexResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return IndexResult{}, client.responseError(response)
	}
	var result struct {
		Hits               []Document                `json:"hits"`
		EstimatedTotalHits int                       `json:"estimatedTotalHits"`
		FacetDistribution  map[string]map[string]int `json:"facetDistribution"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return IndexResult{}, fmt.Errorf("decode Meilisearch result: %w", err)
	}
	return IndexResult{Hits: result.Hits, Total: result.EstimatedTotalHits, FacetDistribution: result.FacetDistribution}, nil
}

func (client MeilisearchClient) Stats(ctx context.Context, indexUID string) (IndexStats, error) {
	response, err := client.request(ctx, http.MethodGet, "/indexes/"+url.PathEscape(indexUID)+"/stats", nil)
	if err != nil {
		return IndexStats{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return IndexStats{}, client.responseError(response)
	}
	var stats IndexStats
	if err := json.NewDecoder(response.Body).Decode(&stats); err != nil {
		return IndexStats{}, fmt.Errorf("decode Meilisearch stats: %w", err)
	}
	return stats, nil
}

func (client MeilisearchClient) SwapIndexes(ctx context.Context, firstIndexUID, secondIndexUID string) error {
	response, err := client.request(ctx, http.MethodPost, "/swap-indexes", []map[string]any{{"indexes": []string{firstIndexUID, secondIndexUID}}})
	if err != nil {
		return err
	}
	return client.waitForAcceptedTask(ctx, response)
}

func (client MeilisearchClient) waitForAcceptedTask(ctx context.Context, response *http.Response) error {
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return client.responseError(response)
	}
	var accepted struct {
		TaskUID *int64 `json:"taskUid"`
	}
	if err := json.NewDecoder(response.Body).Decode(&accepted); err != nil {
		return fmt.Errorf("decode Meilisearch task: %w", err)
	}
	if accepted.TaskUID == nil || *accepted.TaskUID < 0 {
		return errors.New("Meilisearch returned an invalid task identifier")
	}
	timeout := client.TaskTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return errors.New("Meilisearch task timed out")
		}
		taskResponse, err := client.request(ctx, http.MethodGet, "/tasks/"+strconv.FormatInt(*accepted.TaskUID, 10), nil)
		if err != nil {
			return err
		}
		if taskResponse.StatusCode != http.StatusOK {
			err := client.responseError(taskResponse)
			taskResponse.Body.Close()
			return err
		}
		var task struct {
			Status string `json:"status"`
			Error  *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		err = json.NewDecoder(taskResponse.Body).Decode(&task)
		taskResponse.Body.Close()
		if err != nil {
			return fmt.Errorf("decode Meilisearch task status: %w", err)
		}
		switch task.Status {
		case "succeeded":
			return nil
		case "failed", "canceled":
			if task.Error != nil {
				return &taskError{code: task.Error.Code, message: task.Error.Message}
			}
			return fmt.Errorf("Meilisearch task ended with status %s", task.Status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func (client MeilisearchClient) request(ctx context.Context, method, path string, payload any) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode Meilisearch request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(client.BaseURL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if client.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+client.APIKey)
	}
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Meilisearch request failed: %w", err)
	}
	return response, nil
}

func (client MeilisearchClient) responseError(response *http.Response) error {
	content, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("Meilisearch returned %s: %s", response.Status, strings.TrimSpace(string(content)))
}
