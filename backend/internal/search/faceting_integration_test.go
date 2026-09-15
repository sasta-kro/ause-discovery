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

// TestPinnedMeilisearchReturnsCompletePersonFacet proves the actual pinned
// Meilisearch engine behavior for the explicit faceting settings: a
// distribution of more than 100 distinct Person UUID values returns every
// seeded value below the configured bound, the person facets sort by count,
// and a target that alphabetical truncation would have dropped stays present
// with its separate People and Advisor counts. The test requires
// AUSE_TEST_MEILISEARCH_URL and fails against the engine's default
// 100-value alphabetical cap.
func TestPinnedMeilisearchReturnsCompletePersonFacet(t *testing.T) {
	meilisearchURL := os.Getenv("AUSE_TEST_MEILISEARCH_URL")
	if meilisearchURL == "" {
		t.Fatal("AUSE_TEST_MEILISEARCH_URL is required for the pinned Meilisearch faceting test")
	}
	ctx := context.Background()
	indexUID := "faceting_test_" + strings.ReplaceAll(uuid.NewString()[:13], "-", "")
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

	// One hundred twenty Person UUIDs, plus one target UUID that sorts
	// alphabetically after every other value so the previous default
	// alphabetical 100-value cap would always have dropped it.
	personIDs := make([]uuid.UUID, 0, 121)
	for index := 0; index < 120; index += 1 {
		personIDs = append(personIDs, uuid.MustParse(fmt.Sprintf("018f0000-0000-7000-8000-%012d", index)))
	}
	target := uuid.MustParse("ffffffff-ffff-7000-8000-00000000ffff")
	personIDs = append(personIDs, target)

	// One document per Person, then two extra documents repeating the target
	// as advisor and committee participation for People count 3, and a
	// separate advisor projection for Advisor count 2. Five advisors keep
	// the narrower facet meaningfully populated.
	documents := make([]Document, 0, 128)
	for index, person := range personIDs {
		document := Document{ID: uuid.MustParse(fmt.Sprintf("018f0000-0000-7000-8000-%012d", 900+index)), Title: fmt.Sprintf("Facet corpus %d", index), PersonIDs: []string{person.String()}}
		if index < 5 {
			document.AdvisorPersonIDs = []string{person.String()}
		}
		documents = append(documents, document)
	}
	documents = append(documents,
		Document{ID: uuid.MustParse("018f0000-0000-7000-8000-000000000a01"), Title: "Target repeat one", PersonIDs: []string{target.String()}, AdvisorPersonIDs: []string{target.String()}},
		Document{ID: uuid.MustParse("018f0000-0000-7000-8000-000000000a02"), Title: "Target repeat two", PersonIDs: []string{target.String()}, AdvisorPersonIDs: []string{target.String()}},
	)
	if err := client.UpsertDocuments(ctx, indexUID, documents); err != nil {
		t.Fatalf("seed documents: %v", err)
	}

	result, err := client.Search(ctx, indexUID, IndexQuery{Limit: 1, Facets: []string{"person_ids", "advisor_person_ids"}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	people, ok := result.FacetDistribution["person_ids"]
	if !ok {
		t.Fatal("person_ids facet distribution missing")
	}
	if len(people) <= 100 {
		t.Fatalf("person facet returned %d values, expected more than the previous 100-value cap", len(people))
	}
	for _, person := range personIDs {
		if _, present := people[person.String()]; !present {
			t.Fatalf("person %s missing from a %d-value distribution below the bound", person, len(people))
		}
	}
	if got := people[target.String()]; got != 3 {
		t.Fatalf("target People count was %d, expected 3", got)
	}

	advisors, ok := result.FacetDistribution["advisor_person_ids"]
	if !ok {
		t.Fatal("advisor_person_ids facet distribution missing")
	}
	if len(advisors) != 6 {
		t.Fatalf("advisor facet returned %d values, expected 6", len(advisors))
	}
	if got := advisors[target.String()]; got != 2 {
		t.Fatalf("target Advisor count was %d, expected 2", got)
	}
}
