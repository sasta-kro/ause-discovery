package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
