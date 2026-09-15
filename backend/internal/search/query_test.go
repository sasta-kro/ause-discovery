package search

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
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

func TestResolveOrderCoversImplicitAndExplicitModes(t *testing.T) {
	cases := []struct {
		name     string
		sort     string
		text     string
		wantName string
		wantSort []string
	}{
		{name: "implicit empty", sort: "", text: "", wantName: OrderAcademicNewest, wantSort: academicNewestSort},
		{name: "implicit whitespace", sort: "", text: "   ", wantName: OrderAcademicNewest, wantSort: academicNewestSort},
		{name: "implicit text", sort: "", text: "vision", wantName: OrderRelevance, wantSort: nil},
		{name: "explicit relevance with text", sort: "relevance", text: "vision", wantName: OrderRelevance, wantSort: nil},
		{name: "explicit relevance without text", sort: "relevance", text: "", wantName: OrderAcademicNewest, wantSort: academicNewestSort},
		{name: "explicit newest", sort: "newest", text: "vision", wantName: OrderAcademicNewest, wantSort: academicNewestSort},
		{name: "explicit oldest", sort: "oldest", text: "vision", wantName: OrderAcademicOldest, wantSort: academicOldestSort},
		{name: "explicit title", sort: "title", text: "vision", wantName: OrderTitle, wantSort: titleSort},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			resolved := ResolveOrder(test.sort, test.text)
			if resolved.Name != test.wantName {
				t.Fatalf("resolved %q for sort %q text %q, expected %q", resolved.Name, test.sort, test.text, test.wantName)
			}
			if len(resolved.Sort) != len(test.wantSort) {
				t.Fatalf("resolved sort %v, expected %v", resolved.Sort, test.wantSort)
			}
			for index := range resolved.Sort {
				if resolved.Sort[index] != test.wantSort[index] {
					t.Fatalf("resolved sort %v, expected %v", resolved.Sort, test.wantSort)
				}
			}
		})
	}
}

func TestAcademicSortListsAreDeterministic(t *testing.T) {
	// Newest order: year descending, semester descending (Summer, Second,
	// First through semester_order), normalized title, then the unique id.
	if fmt.Sprint(academicNewestSort) != fmt.Sprint([]string{"academic_year:desc", "semester_order:desc", "title_sort:asc", "id:asc"}) {
		t.Fatalf("newest sort list was %v", academicNewestSort)
	}
	if fmt.Sprint(academicOldestSort) != fmt.Sprint([]string{"academic_year:asc", "semester_order:asc", "title_sort:asc", "id:asc"}) {
		t.Fatalf("oldest sort list was %v", academicOldestSort)
	}
	if fmt.Sprint(titleSort) != fmt.Sprint([]string{"title_sort:asc", "academic_year:desc", "semester_order:desc", "id:asc"}) {
		t.Fatalf("title sort list was %v", titleSort)
	}
	// Relevance mode never sends query-time sort.
	if resolved := ResolveOrder("", "vision"); resolved.Name == OrderRelevance && resolved.Sort != nil {
		t.Fatal("relevance mode sent a query-time sort")
	}
}

func TestSemesterOrderMatchesAcademicChronology(t *testing.T) {
	// Within one academic year, newest order is Summer, Second, First.
	if !(semesterOrder("summer") > semesterOrder("second") && semesterOrder("second") > semesterOrder("first")) {
		t.Fatal("semester order does not satisfy Summer, Second, First chronology")
	}
}

func TestCursorBindsToResolvedOrderingAndSchema(t *testing.T) {
	query := Query{Text: "vision", Sort: "", Limit: 20}
	cursor, err := encodeCursor(query, 20)
	if err != nil {
		t.Fatalf("encodeCursor returned an error: %v", err)
	}
	if _, err := decodeCursor(query, cursor); err != nil {
		t.Fatalf("identical query cursor rejected: %v", err)
	}

	// Semantically equivalent omitted and explicit relevance share the same
	// resolved ordering, so their cursors interchange.
	explicit := query
	explicit.Sort = "relevance"
	if _, err := decodeCursor(explicit, cursor); err != nil {
		t.Fatalf("explicit-relevance cursor rejected against implicit query: %v", err)
	}

	// A different resolved ordering invalidates the cursor.
	changed := query
	changed.Sort = "oldest"
	if _, err := decodeCursor(changed, cursor); err != ErrInvalidCursor {
		t.Fatal("changed-ordering cursor accepted")
	}
	changed = query
	changed.Sort = "title"
	if _, err := decodeCursor(changed, cursor); err != ErrInvalidCursor {
		t.Fatal("title-ordering cursor accepted")
	}

	// Text emptiness changes the resolved ordering for a relevance request.
	changed = query
	changed.Text = ""
	if _, err := decodeCursor(changed, cursor); err != ErrInvalidCursor {
		t.Fatal("empty-text cursor accepted against textual query")
	}
}

func TestCursorRejectsOlderSchemaVersion(t *testing.T) {
	query := Query{Text: "vision", Limit: 20}
	cursor, err := encodeCursor(query, 20)
	if err != nil {
		t.Fatalf("encodeCursor returned an error: %v", err)
	}
	if _, err := decodeCursor(query, cursor); err != nil {
		t.Fatalf("current-version cursor rejected: %v", err)
	}

	// Recreate a version-2 cursor exactly as the previous schema encoded it.
	// The settings change to schema version 3 must invalidate it through the
	// controlled invalid-cursor path rather than paging old positions.
	legacy := query
	legacy.Cursor = ""
	legacy.Text = strings.TrimSpace(legacy.Text)
	legacy.Sort = ResolveOrder(legacy.Sort, legacy.Text).Name
	encoded, err := json.Marshal(struct {
		SchemaVersion int   `json:"schema_version"`
		Query         Query `json:"query"`
	}{SchemaVersion: 2, Query: legacy})
	if err != nil {
		t.Fatalf("marshal legacy hash payload: %v", err)
	}
	digest := sha256.Sum256(encoded)
	legacyCursor, err := json.Marshal(cursorPayload{Offset: 20, QueryHash: hex.EncodeToString(digest[:8])})
	if err != nil {
		t.Fatalf("marshal legacy cursor: %v", err)
	}
	stale := base64.RawURLEncoding.EncodeToString(legacyCursor)
	if _, err := decodeCursor(query, stale); err != ErrInvalidCursor {
		t.Fatalf("version-2 cursor accepted under schema version %d: %v", SchemaVersion, err)
	}
}

func TestCursorAcceptsSemanticallyEquivalentWhitespace(t *testing.T) {
	padded := Query{Text: "  greenhouse  ", Limit: 20}
	cursor, err := encodeCursor(padded, 20)
	if err != nil {
		t.Fatalf("encodeCursor returned an error: %v", err)
	}
	trimmed := Query{Text: "greenhouse", Limit: 20}
	if _, err := decodeCursor(trimmed, cursor); err != nil {
		t.Fatalf("trimmed query rejected the padded-query cursor: %v", err)
	}
	if _, err := decodeCursor(padded, cursor); err != nil {
		t.Fatalf("padded query rejected its own cursor: %v", err)
	}
	different := Query{Text: "greenhouse kit", Limit: 20}
	if _, err := decodeCursor(different, cursor); err != ErrInvalidCursor {
		t.Fatal("genuinely different text accepted the cursor")
	}
}
