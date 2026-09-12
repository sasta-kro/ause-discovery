package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/platform/config"
)

func TestHealthSeparatesLivenessFromReadiness(t *testing.T) {
	for _, ready := range []bool{true, false} {
		mux := http.NewServeMux()
		HealthHandler{CheckReadiness: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("missing dependency deadline")
			}
			if !ready {
				return errors.New("private dependency details")
			}
			return nil
		}}.Register(mux, "/ause-discovery/")
		for _, endpoint := range []string{"live", "ready"} {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest("GET", "/ause-discovery/health/"+endpoint, nil))
			want := http.StatusOK
			if endpoint == "ready" && !ready {
				want = http.StatusServiceUnavailable
			}
			if response.Code != want || strings.Contains(response.Body.String(), "private") {
				t.Fatalf("%s returned %d: %s", endpoint, response.Code, response.Body.String())
			}
		}
	}
}

func TestControllerImplementsVersionedHealthContract(t *testing.T) {
	for _, ready := range []bool{true, false} {
		controller := Controller{Readiness: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("missing dependency deadline")
			}
			if !ready {
				return errors.New("private dependency details")
			}
			return nil
		}}

		liveResponse := httptest.NewRecorder()
		controller.GetLiveHealth(liveResponse, httptest.NewRequest("GET", "/api/v1/health/live", nil))
		if liveResponse.Code != http.StatusOK || !strings.Contains(liveResponse.Body.String(), `"status":"live"`) {
			t.Fatalf("liveness returned %d: %s", liveResponse.Code, liveResponse.Body.String())
		}

		readyResponse := httptest.NewRecorder()
		controller.GetReadyHealth(readyResponse, httptest.NewRequest("GET", "/api/v1/health/ready", nil))
		want := http.StatusOK
		if !ready {
			want = http.StatusServiceUnavailable
		}
		if readyResponse.Code != want || strings.Contains(readyResponse.Body.String(), "private") {
			t.Fatalf("readiness returned %d: %s", readyResponse.Code, readyResponse.Body.String())
		}
	}
}

// countingReadyProvider counts readiness probes so tests can prove both
// endpoint forms share one provider instance and its cache.
type countingReadyProvider struct {
	artifacts.LocalStorage
	probes int
	fails  bool
}

func (provider *countingReadyProvider) Name() string { return artifacts.BackendLocal }

func (provider *countingReadyProvider) CheckReady(ctx context.Context) error {
	provider.probes++
	if provider.fails {
		return errors.New("provider secret detail")
	}
	return provider.LocalStorage.CheckReady(ctx)
}

func TestReadinessCompositionIncludesTheStorageProvider(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLogoHTTPTestDatabase(t, ctx, databaseURL)
	configuration := config.Config{
		PublicBasePath:      "/ause-discovery/",
		ImportTemporaryRoot: t.TempDir(),
		SessionIdleTTL:      time.Minute,
		SessionAbsoluteTTL:  time.Minute,
	}
	provider := &countingReadyProvider{LocalStorage: artifacts.LocalStorage{Root: t.TempDir()}}
	set := artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{artifacts.BackendLocal: provider}}
	check := func(ctx context.Context) error { return CheckReadiness(ctx, pool, configuration, set) }

	if err := check(ctx); err != nil {
		t.Fatalf("healthy composition returned an error: %v", err)
	}
	if provider.probes != 1 {
		t.Fatalf("composition probed the provider %d times, expected 1", provider.probes)
	}

	provider.fails = true
	err := check(ctx)
	if err == nil {
		t.Fatal("failing provider still reported ready")
	}

	// Both endpoint forms return 503 with no provider detail while both
	// liveness forms stay 200. The fake provider has no cache, so every
	// readiness call reaching the provider proves the composition wires it
	// through both endpoint forms; provider-owned caching is proven in the
	// artifacts readiness tests.
	mux := http.NewServeMux()
	HealthHandler{CheckReadiness: check}.Register(mux, "/ause-discovery/")
	controller := Controller{Readiness: check}

	unversioned := httptest.NewRecorder()
	mux.ServeHTTP(unversioned, httptest.NewRequest("GET", "/ause-discovery/health/ready", nil))
	if unversioned.Code != http.StatusServiceUnavailable || strings.Contains(unversioned.Body.String(), "secret") {
		t.Fatalf("unversioned readiness returned %d: %s", unversioned.Code, unversioned.Body.String())
	}
	versioned := httptest.NewRecorder()
	controller.GetReadyHealth(versioned, httptest.NewRequest("GET", "/ause-discovery/api/v1/health/ready", nil))
	if versioned.Code != http.StatusServiceUnavailable || strings.Contains(versioned.Body.String(), "secret") {
		t.Fatalf("versioned readiness returned %d: %s", versioned.Code, versioned.Body.String())
	}
	if probes := provider.probes; probes != 4 {
		t.Fatalf("shared provider probed %d times across the failing check and both endpoint forms, expected 4", probes)
	}

	for name, request := range map[string]*http.Request{
		"unversioned": httptest.NewRequest("GET", "/ause-discovery/health/live", nil),
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s liveness returned %d during storage failure", name, response.Code)
		}
	}
	liveResponse := httptest.NewRecorder()
	controller.GetLiveHealth(liveResponse, httptest.NewRequest("GET", "/ause-discovery/api/v1/health/live", nil))
	if liveResponse.Code != http.StatusOK {
		t.Fatalf("versioned liveness returned %d during storage failure", liveResponse.Code)
	}
}

// b2ListEndpoint is a minimal S3-compatible ListObjectsV2 endpoint for
// proving the real B2 provider wiring without network access. It counts
// signed list requests and answers with an empty healthy result.
type b2ListEndpoint struct {
	requests int
}

func (endpoint *b2ListEndpoint) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Query().Get("list-type") == "2" {
		if strings.HasPrefix(request.Header.Get("Authorization"), "AWS4-HMAC-SHA256") {
			endpoint.requests++
		}
		writer.Header().Set("Content-Type", "application/xml")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>test-bucket</Name><IsTruncated>false</IsTruncated><MaxKeys>1</MaxKeys><Prefix>v1/</Prefix></ListBucketResult>`))
		return
	}
	writer.WriteHeader(http.StatusNotFound)
}

// TestBothReadinessEndpointFormsShareOneB2Request proves the real wiring:
// one constructed B2Storage instance serves the unversioned HealthHandler
// route and the versioned NewAPIHandler route, and both endpoint forms
// inside the success cache window result in exactly one remote request.
func TestBothReadinessEndpointFormsShareOneB2Request(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLogoHTTPTestDatabase(t, ctx, databaseURL)
	endpoint := &b2ListEndpoint{}
	server := httptest.NewServer(endpoint)
	t.Cleanup(server.Close)
	storage, storageErr := artifacts.NewB2Storage(server.URL, "test-bucket", "key-id", "application-key", 1024)
	if storageErr != nil {
		t.Fatalf("NewB2Storage returned an error: %v", storageErr)
	}
	set := artifacts.StorageSet{DefaultName: artifacts.BackendB2, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: artifacts.LocalStorage{Root: t.TempDir()},
		artifacts.BackendB2:    storage,
	}}
	configuration := config.Config{
		PublicBasePath:      "/ause-discovery/",
		ImportTemporaryRoot: t.TempDir(),
		SessionIdleTTL:      time.Minute,
		SessionAbsoluteTTL:  time.Minute,
	}
	check := func(ctx context.Context) error { return CheckReadiness(ctx, pool, configuration, set) }

	// The unversioned health route wired the way cmd/api wires it.
	mux := http.NewServeMux()
	HealthHandler{CheckReadiness: check}.Register(mux, configuration.PublicBasePath)
	unversioned := httptest.NewRecorder()
	mux.ServeHTTP(unversioned, httptest.NewRequest("GET", "/ause-discovery/health/ready", nil))
	if unversioned.Code != http.StatusOK {
		t.Fatalf("unversioned readiness returned %d: %s", unversioned.Code, unversioned.Body.String())
	}

	// The versioned route through the real generated handler.
	apiHandler := NewAPIHandler(pool, configuration, set)
	versioned := httptest.NewRecorder()
	apiHandler.ServeHTTP(versioned, httptest.NewRequest("GET", "/ause-discovery/api/v1/health/ready", nil))
	if versioned.Code != http.StatusOK {
		t.Fatalf("versioned readiness returned %d: %s", versioned.Code, versioned.Body.String())
	}

	if endpoint.requests != 1 {
		t.Fatalf("both endpoint forms issued %d signed list requests, expected 1 within the success cache window", endpoint.requests)
	}
}
