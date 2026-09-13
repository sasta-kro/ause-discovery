package search

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestPinnedMeilisearchOrdersAcademically proves the actual pinned
// Meilisearch engine behavior for the ordering contract: placeholder
// searches default to academic newest, relevance keeps lexical ranking
// ahead of chronology, ties fall back to academic newest, and explicit
// oldest and title orders are authoritative and stable across pages. The
// test requires AUSE_TEST_MEILISEARCH_URL and uses a unique disposable
// index it removes afterward.
func TestPinnedMeilisearchOrdersAcademically(t *testing.T) {
	meilisearchURL := os.Getenv("AUSE_TEST_MEILISEARCH_URL")
	if meilisearchURL == "" {
		t.Fatal("AUSE_TEST_MEILISEARCH_URL is required for the pinned Meilisearch ordering test")
	}
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createSearchTestDatabase(t, ctx, databaseURL)
	indexUID := "ordering_test_" + strings.ReplaceAll(uuid.NewString()[:13], "-", "")
	client := MeilisearchClient{BaseURL: meilisearchURL, APIKey: os.Getenv("AUSE_TEST_MEILISEARCH_KEY"), TaskTimeout: 10 * time.Second}
	if err := client.DeleteIndex(ctx, indexUID); err != nil {
		t.Fatalf("pre-clean index: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.DeleteIndex(cleanupContext, indexUID); err != nil {
			t.Errorf("clean up disposable index %s: %v", indexUID, err)
		}
	})
	if err := client.EnsureIndex(ctx, indexUID); err != nil {
		t.Fatalf("ensure index: %v", err)
	}

	documents := []Document{
		orderingDocument("018f0000-0000-7000-8000-000000000101", 2016, 1, "Alpha archiver", ""),
		orderingDocument("018f0000-0000-7000-8000-000000000102", 2020, 1, "Beta beacon", ""),
		orderingDocument("018f0000-0000-7000-8000-000000000103", 2020, 2, "Caution beacon", ""),
		orderingDocument("018f0000-0000-7000-8000-000000000104", 2020, 3, "Delta beacon", ""),
		// Two equally relevant documents differ only in academic period.
		orderingDocument("018f0000-0000-7000-8000-000000000105", 2018, 1, "Identical twin controller", "greenhouse greenhouse monitoring"),
		orderingDocument("018f0000-0000-7000-8000-000000000106", 2024, 2, "Identical twin controller", "greenhouse greenhouse monitoring"),
		// A strong older textual match against a weak newer one.
		orderingDocument("018f0000-0000-7000-8000-000000000107", 2019, 1, "Greenhouse guardian", "greenhouse control"),
		orderingDocument("018f0000-0000-7000-8000-000000000108", 2025, 1, "Modern ledger", "mentions greenhouse once"),
	}
	if err := client.UpsertDocuments(ctx, indexUID, documents); err != nil {
		t.Fatalf("seed documents: %v", err)
	}
	service := Service{Pool: pool, Index: client, IndexUID: indexUID}
	titles := func(result Result) []string {
		collected := make([]string, 0, len(result.Items))
		for _, item := range result.Items {
			collected = append(collected, item.Title)
		}
		return collected
	}
	ids := func(result Result) []string {
		collected := make([]string, 0, len(result.Items))
		for _, item := range result.Items {
			collected = append(collected, item.ID.String())
		}
		return collected
	}

	// Placeholder search orders by academic newest: 2025 first, then equal
	// 2024, ... and within 2020 Summer (semester_order 3), then Second, then
	// First.
	placeholder, err := service.Search(ctx, Query{Limit: 20})
	if err != nil {
		t.Fatalf("placeholder search: %v", err)
	}
	got := titles(placeholder)
	wantHead := []string{"Modern ledger", "Identical twin controller", "Delta beacon", "Caution beacon", "Beta beacon"}
	for index, want := range wantHead {
		if got[index] != want {
			t.Fatalf("placeholder order was %v, expected head %v", got, wantHead)
		}
	}

	// Relevance keeps lexical ranking ahead of chronology: the stronger
	// older textual match ranks above the weaker newer one, and equally
	// relevant documents fall back to academic newest.
	relevant, err := service.Search(ctx, Query{Text: "greenhouse", Limit: 20})
	if err != nil {
		t.Fatalf("relevance search: %v", err)
	}
	got = titles(relevant)
	// The title match (words bucket) beats the abstract-only match; between
	// the two identical titles, 2024 precedes 2018.
	if got[0] != "Greenhouse guardian" || got[1] != "Identical twin controller" || got[2] != "Identical twin controller" {
		t.Fatalf("relevance order was %v", got)
	}
	if relevant.Items[1].AcademicYear != 2024 || relevant.Items[2].AcademicYear != 2018 {
		t.Fatalf("equal-relevance tie did not use academic newest: %v", got)
	}

	// An explicit academic sort is authoritative even for a nonempty text
	// query: the older document with the strong lexical match must not
	// outrank the newer document when the user asks for newest.
	explicitNewest, err := service.Search(ctx, Query{Text: "greenhouse", Sort: "newest", Limit: 20})
	if err != nil {
		t.Fatalf("explicit newest text search: %v", err)
	}
	got = titles(explicitNewest)
	if got[0] != "Modern ledger" || got[1] != "Identical twin controller" {
		t.Fatalf("explicit newest did not override lexical relevance: %v", got)
	}
	explicitOldest, err := service.Search(ctx, Query{Text: "greenhouse", Sort: "oldest", Limit: 20})
	if err != nil {
		t.Fatalf("explicit oldest text search: %v", err)
	}
	got = titles(explicitOldest)
	if got[0] != "Identical twin controller" || explicitOldest.Items[0].AcademicYear != 2018 || got[len(got)-1] != "Modern ledger" {
		t.Fatalf("explicit oldest did not reverse chronology for a text query: %v", got)
	}

	// Explicit oldest reverses academic chronology.
	oldest, err := service.Search(ctx, Query{Sort: "oldest", Limit: 20})
	if err != nil {
		t.Fatalf("oldest search: %v", err)
	}
	if oldestResult := titles(oldest); oldestResult[0] != "Alpha archiver" || oldestResult[len(oldestResult)-1] != "Modern ledger" {
		t.Fatalf("oldest order was %v", oldestResult)
	}

	// Explicit title uses normalized title order deterministically.
	byTitle, err := service.Search(ctx, Query{Sort: "title", Limit: 20})
	if err != nil {
		t.Fatalf("title search: %v", err)
	}
	got = titles(byTitle)
	for index := 1; index < len(got); index++ {
		if strings.ToLower(got[index-1]) > strings.ToLower(got[index]) {
			t.Fatalf("title order was not ascending at %d: %v", index, got)
		}
	}

	// Two bounded pages carry no duplicate Project IDs.
	firstPage, err := service.Search(ctx, Query{Limit: 3})
	if err != nil || firstPage.NextCursor == nil {
		t.Fatalf("first page returned error %v and cursor %v", err, firstPage.NextCursor)
	}
	secondPage, err := service.Search(ctx, Query{Limit: 3, Cursor: *firstPage.NextCursor})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	seen := map[string]bool{}
	for _, id := range append(append([]string{}, ids(firstPage)...), ids(secondPage)...) {
		if seen[id] {
			t.Fatalf("Project %s appeared on both pages", id)
		}
		seen[id] = true
	}
	if len(seen) != 6 {
		t.Fatalf("two pages covered %d distinct Projects, expected 6", len(seen))
	}
}

func orderingDocument(identifier string, year int, semesterOrder int, title, abstract string) Document {
	parsed := uuid.MustParse(identifier)
	semester := "first"
	switch semesterOrder {
	case 2:
		semester = "second"
	case 3:
		semester = "summer"
	}
	return Document{
		ID: parsed, Title: title, TitleSort: strings.ToLower(title), Abstract: abstract,
		AcademicYear: year, Semester: semester, SemesterOrder: semesterOrder,
		Program:     CatalogReference{ID: parsed, Key: fmt.Sprintf("program_%s", identifier[len(identifier)-4:]), Label: "Ordering Program"},
		Course:      CatalogReference{ID: parsed, Key: fmt.Sprintf("course_%s", identifier[len(identifier)-4:]), Label: "Ordering Course"},
		PublishedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}
}
