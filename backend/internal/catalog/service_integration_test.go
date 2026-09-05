package catalog

import (
	"context"
	"os"
	"testing"
)

func TestListReturnsSynchronizedReferenceData(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, removeDatabase := createCatalogTestDatabase(t, ctx, databaseURL)
	t.Cleanup(removeDatabase)
	applyCatalogTestMigrations(t, ctx, pool)
	source, err := Load("../../../config/catalogs/academic.yaml", "../../../config/taxonomy/values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := (Synchronizer{DatabasePool: pool}).Sync(ctx, source); err != nil {
		t.Fatal(err)
	}
	values, err := (Service{Pool: pool}).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(values.Programs) == 0 || len(values.Courses) == 0 || len(values.Taxonomy) == 0 {
		t.Fatalf("incomplete catalogs: %+v", values)
	}
	for _, value := range values.Taxonomy {
		if value.Labels["en"] == "" {
			t.Fatalf("taxonomy lacks English label: %+v", value)
		}
	}
}
