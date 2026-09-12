package httpserver

import (
	"testing"

	searchservice "ause-discovery.local/backend/internal/search"
	"github.com/google/uuid"
)

func TestSearchResponseRendersVersionedLogoURL(t *testing.T) {
	withLogo := uuid.MustParse("018f0000-0000-7000-8000-000000000701")
	withoutLogo := uuid.MustParse("018f0000-0000-7000-8000-000000000702")
	revision := int64(6)
	result := searchservice.Result{Limit: 20, Items: []searchservice.ResultItem{
		{Document: searchservice.Document{ID: withLogo, Title: "With Logo", LogoRevision: &revision}},
		{Document: searchservice.Document{ID: withoutLogo, Title: "Without Logo"}},
	}}
	response := searchResponse(result, "/ause-discovery/")
	if len(response.Items) != 2 {
		t.Fatalf("search response held %d items", len(response.Items))
	}
	if response.Items[0].LogoUrl == nil || *response.Items[0].LogoUrl != "/ause-discovery/api/v1/projects/"+withLogo.String()+"/logo?v=6" {
		t.Fatalf("first item carried logo URL %v, expected the base-path versioned URL", response.Items[0].LogoUrl)
	}
	if response.Items[1].LogoUrl != nil {
		t.Fatalf("second item carried logo URL %v without a logo revision", *response.Items[1].LogoUrl)
	}
}
