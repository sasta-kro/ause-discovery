package main

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"ause-discovery.local/backend/internal/platform/config"
	searchservice "ause-discovery.local/backend/internal/search"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewSearchReconcilerUsesRuntimeConfiguration(t *testing.T) {
	configuration := config.Config{
		MeilisearchURL:    "http://search.test:7700",
		MeilisearchAPIKey: "search-key",
		MeilisearchIndex:  "project-search",
	}
	pool := &pgxpool.Pool{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	reconciler := newSearchReconciler(pool, configuration, logger)

	if reconciler.Pool != pool || reconciler.IndexUID != "project-search" || reconciler.Logger != logger {
		t.Fatalf("unexpected reconciler configuration: %+v", reconciler)
	}
	if reconciler.WorkerID == "" {
		t.Fatal("expected a unique worker identifier")
	}
	client, ok := reconciler.Index.(searchservice.MeilisearchClient)
	if !ok {
		t.Fatalf("expected Meilisearch client, got %T", reconciler.Index)
	}
	if client.BaseURL != "http://search.test:7700" || client.APIKey != "search-key" || client.TaskTimeout != 10*time.Second {
		t.Fatalf("unexpected search client configuration: %+v", client)
	}
}
