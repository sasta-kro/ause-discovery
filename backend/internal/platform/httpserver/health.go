package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/platform/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthHandler struct {
	CheckReadiness func(context.Context) error
}

func (handler HealthHandler) Register(mux *http.ServeMux, publicBasePath string) {
	mux.HandleFunc("GET "+publicBasePath+"health/live", handler.live)
	mux.HandleFunc("GET "+publicBasePath+"health/ready", handler.ready)
}

func (handler HealthHandler) live(responseWriter http.ResponseWriter, _ *http.Request) {
	writeHealthResponse(responseWriter, http.StatusOK, "live")
}

func (handler HealthHandler) ready(responseWriter http.ResponseWriter, request *http.Request) {
	if handler.CheckReadiness == nil {
		writeHealthResponse(responseWriter, http.StatusServiceUnavailable, "not ready")
		return
	}

	deadline, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	if err := handler.CheckReadiness(deadline); err != nil {
		writeHealthResponse(responseWriter, http.StatusServiceUnavailable, "not ready")
		return
	}

	writeHealthResponse(responseWriter, http.StatusOK, "ready")
}

func CheckReadiness(ctx context.Context, pool *pgxpool.Pool, configuration config.Config) error {
	if err := database.CheckSchema(ctx, pool); err != nil {
		return err
	}
	return configuration.CheckStorageDirectories()
}

func (controller *Controller) GetLiveHealth(responseWriter http.ResponseWriter, _ *http.Request) {
	writeHealthResponse(responseWriter, http.StatusOK, "live")
}

func (controller *Controller) GetReadyHealth(responseWriter http.ResponseWriter, request *http.Request) {
	if controller.Readiness == nil {
		problem(responseWriter, request, http.StatusServiceUnavailable, "service_unavailable", "Service unavailable", "")
		return
	}

	deadline, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	if err := controller.Readiness(deadline); err != nil {
		problem(responseWriter, request, http.StatusServiceUnavailable, "service_unavailable", "Service unavailable", "")
		return
	}

	writeHealthResponse(responseWriter, http.StatusOK, "ready")
}

func writeHealthResponse(responseWriter http.ResponseWriter, status int, state string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(map[string]string{"status": state})
}
