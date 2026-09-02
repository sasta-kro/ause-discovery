package httpserver

import (
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	api "ause-discovery.local/backend/generated/api"
	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/projects"
	"github.com/google/uuid"
)

func TestClientIPIgnoresForwardingFromUntrustedPeer(t *testing.T) {
	request := httptest.NewRequest("GET", "http://example.test/", nil)
	request.RemoteAddr = "203.0.113.10:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.20")

	address := clientIP(request, config.Config{TrustedProxyCIDRs: []string{"10.0.0.0/8"}})
	if address != netip.MustParseAddr("203.0.113.10") {
		t.Fatalf("clientIP returned %s", address)
	}
}

func TestClientIPSelectsFirstUntrustedAddressFromRight(t *testing.T) {
	request := httptest.NewRequest("GET", "http://example.test/", nil)
	request.RemoteAddr = "10.0.0.5:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.99, 198.51.100.20, 10.0.0.3")

	address := clientIP(request, config.Config{TrustedProxyCIDRs: []string{"10.0.0.0/8"}})
	if address != netip.MustParseAddr("198.51.100.20") {
		t.Fatalf("clientIP returned %s", address)
	}
}

func TestTrustedOriginUsesForwardedProtocolOnlyFromTrustedPeer(t *testing.T) {
	configuration := config.Config{TrustedProxyCIDRs: []string{"10.0.0.0/8"}}
	trustedRequest := httptest.NewRequest("POST", "http://example.edu/ause-discovery/api/v1/admin/projects", nil)
	trustedRequest.RemoteAddr = "10.0.0.5:4321"
	trustedRequest.Host = "example.edu"
	trustedRequest.Header.Set("Origin", "https://example.edu")
	trustedRequest.Header.Set("X-Forwarded-Proto", "https")
	if !trustedOrigin(trustedRequest, configuration) {
		t.Fatal("trustedOrigin rejected the HTTPS origin from a trusted proxy")
	}

	untrustedRequest := trustedRequest.Clone(trustedRequest.Context())
	untrustedRequest.RemoteAddr = "203.0.113.10:4321"
	if trustedOrigin(untrustedRequest, configuration) {
		t.Fatal("trustedOrigin accepted forwarded protocol from an untrusted peer")
	}
}

func TestAdminProjectResponseIncludesStoredAggregateState(t *testing.T) {
	semester := "first"
	project := projects.Project{ID: uuid.New(), Semester: &semester, ExtensionMetadata: map[string]any{"source": "test"}}
	program := &api.CatalogReference{Id: uuid.New(), Key: "computer_science", Label: "Computer Science"}
	participations := []api.Participation{{Role: "student", SortOrder: 0}}
	taxonomy := []api.TaxonomyValue{{Id: uuid.New(), Dimension: "category", Key: "software_application", Labels: map[string]string{"en": "Software / Application"}}}

	response := adminProjectResponse(project, program, nil, nil, participations, taxonomy, nil)
	if response.Semester == nil || *response.Semester != "first" {
		t.Fatalf("administrator response omitted semester: %+v", response.Semester)
	}
	if response.Program == nil || response.Program.Key != "computer_science" {
		t.Fatalf("administrator response omitted Program: %+v", response.Program)
	}
	if len(response.Participations) != 1 || len(response.Taxonomy) != 1 {
		t.Fatalf("administrator response omitted aggregate children")
	}
	if response.ExtensionMetadata == nil || (*response.ExtensionMetadata)["source"] != "test" {
		t.Fatalf("administrator response omitted extension metadata")
	}
}

func TestArtifactResponseExposesPublicURLsOnlyForActiveContent(t *testing.T) {
	artifactID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	projectID := uuid.MustParse("018f0000-0000-7000-8000-000000000502")
	value := artifacts.Artifact{ID: artifactID, ProjectID: projectID, ArtifactType: "report", DisplayName: "Report", OriginalFilename: "report.pdf", MIMEType: "application/pdf", Extension: "pdf", ByteCount: 42, Status: "active", Revision: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}

	active := artifactResponse(value, "/ause-discovery/")
	if active.ViewUrl == nil || active.DownloadUrl == nil {
		t.Fatalf("active PDF URLs were view=%v download=%v", active.ViewUrl, active.DownloadUrl)
	}
	value.Status = "deleted"
	deleted := artifactResponse(value, "/ause-discovery/")
	if deleted.ViewUrl != nil || deleted.DownloadUrl != nil {
		t.Fatalf("deleted Artifact exposed public URLs: %#v", deleted)
	}
}

func TestArtifactContentDispositionUsesSafeFallbackAndUTF8Filename(t *testing.T) {
	disposition := artifactContentDisposition("attachment", "résumé \"final\".pdf")
	expected := `attachment; filename="r_sum_ _final_.pdf"; filename*=UTF-8''r%C3%A9sum%C3%A9%20%22final%22.pdf`
	if disposition != expected {
		t.Fatalf("content disposition was %q, expected %q", disposition, expected)
	}
}
