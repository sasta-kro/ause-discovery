package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthHandler struct {
	DatabasePool *pgxpool.Pool
}

func (handler HealthHandler) Register(mux *http.ServeMux, publicBasePath string) {
	mux.HandleFunc("GET "+publicBasePath+"health/live", handler.live)
	mux.HandleFunc("GET "+publicBasePath+"health/ready", handler.ready)
}

func (handler HealthHandler) live(responseWriter http.ResponseWriter, _ *http.Request) {
	writeHealthResponse(responseWriter, http.StatusOK, "live")
}

func (handler HealthHandler) ready(responseWriter http.ResponseWriter, request *http.Request) {
	if handler.DatabasePool == nil {
		writeHealthResponse(responseWriter, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	deadline, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	if err := handler.DatabasePool.Ping(deadline); err != nil {
		writeHealthResponse(responseWriter, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	writeHealthResponse(responseWriter, http.StatusOK, "ready")
}

func writeHealthResponse(responseWriter http.ResponseWriter, status int, state string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(map[string]string{"status": state})
}
