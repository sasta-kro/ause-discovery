package search

import (
	"reflect"
	"testing"
)

func TestBuildFiltersUsesORWithinFacetsAndANDAcrossFacets(t *testing.T) {
	year := 2026
	hasReport := true
	query := Query{AcademicYear: &year, CategoryKeys: []string{"ai", "web"}, PlatformKeys: []string{"browser"}, HasReport: &hasReport}

	filters := buildFilters(query)
	expected := []string{`academic_year = 2026`, `category_keys IN ["ai","web"]`, `platform_keys IN ["browser"]`, `has_report = true`}
	if !reflect.DeepEqual(filters, expected) {
		t.Fatalf("filters were %#v, expected %#v", filters, expected)
	}
}

func TestSearchCursorRoundTripsAndRejectsDifferentQuery(t *testing.T) {
	query := Query{Text: "archive", CategoryKeys: []string{"web"}, Limit: 20}
	cursor, err := encodeCursor(query, 20)
	if err != nil {
		t.Fatalf("encodeCursor returned an error: %v", err)
	}
	offset, err := decodeCursor(query, cursor)
	if err != nil || offset != 20 {
		t.Fatalf("decodeCursor returned offset %d and error %v", offset, err)
	}
	changed := query
	changed.Text = "different"
	if _, err := decodeCursor(changed, cursor); err != ErrInvalidCursor {
		t.Fatalf("changed-query cursor returned %v, expected invalid cursor", err)
	}
}

func TestMapAndLabelSearchFacets(t *testing.T) {
	personID := "018f0000-0000-7000-8000-000000000701"
	facets := mapFacets(map[string]map[string]int{
		"semester":           {"first": 9},
		"person_ids":         {personID: 4},
		"advisor_person_ids": {personID: 2},
	})
	facets.People = applyFacetLabels(facets.People, map[string]string{personID: "Alex Advisor"})
	facets.Advisors = applyFacetLabels(facets.Advisors, map[string]string{personID: "Alex Advisor"})

	if len(facets.Semesters) != 1 || facets.Semesters[0].Key != "first" || facets.Semesters[0].Count != 9 {
		t.Fatalf("Semester facets were %#v", facets.Semesters)
	}
	if len(facets.People) != 1 || facets.People[0].Label == nil || *facets.People[0].Label != "Alex Advisor" {
		t.Fatalf("People facets were %#v", facets.People)
	}
	if len(facets.Advisors) != 1 || facets.Advisors[0].Label == nil || *facets.Advisors[0].Label != "Alex Advisor" || facets.Advisors[0].Count != 2 {
		t.Fatalf("Advisor facets were %#v", facets.Advisors)
	}
}

func TestTopicTaxonomyDoesNotContributeSearchTerms(t *testing.T) {
	document := Document{}
	appendDocumentTaxonomy(&document, TaxonomyValue{Dimension: "topic", Key: "computer_vision", Labels: map[string]string{"en": "Computer Vision"}})
	appendDocumentTaxonomy(&document, TaxonomyValue{Dimension: "technology", Key: "flutter", Labels: map[string]string{"en": "Flutter"}})

	if !reflect.DeepEqual(document.TopicKeys, []string{"computer_vision"}) {
		t.Fatalf("Topic filter keys were %#v", document.TopicKeys)
	}
	if !reflect.DeepEqual(document.TaxonomyKeys, []string{"flutter"}) || !reflect.DeepEqual(document.TaxonomyLabels, []string{"Flutter"}) {
		t.Fatalf("searchable taxonomy was keys %#v labels %#v", document.TaxonomyKeys, document.TaxonomyLabels)
	}
}
