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
