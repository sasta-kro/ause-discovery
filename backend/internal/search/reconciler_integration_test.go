package search

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type recordingIndex struct {
	ensured []string
	upserts map[string][]Document
	deletes map[string][]uuid.UUID
	swaps   [][2]string
	failure error
}

func (index *recordingIndex) Health(context.Context) error { return index.failure }
func (index *recordingIndex) EnsureIndex(_ context.Context, uid string) error {
	index.ensured = append(index.ensured, uid)
	return index.failure
}
func (index *recordingIndex) UpsertDocuments(_ context.Context, uid string, documents []Document) error {
	if index.failure != nil {
		return index.failure
	}
	if index.upserts == nil {
		index.upserts = map[string][]Document{}
	}
	for _, document := range documents {
		replaced := false
		for position := range index.upserts[uid] {
			if index.upserts[uid][position].ID == document.ID {
				index.upserts[uid][position] = document
				replaced = true
				break
			}
		}
		if !replaced {
			index.upserts[uid] = append(index.upserts[uid], document)
		}
	}
	return nil
}
func (index *recordingIndex) DeleteDocument(_ context.Context, uid string, documentID uuid.UUID) error {
	if index.failure != nil {
		return index.failure
	}
	if index.deletes == nil {
		index.deletes = map[string][]uuid.UUID{}
	}
	index.deletes[uid] = append(index.deletes[uid], documentID)
	documents := index.upserts[uid]
	for position := range documents {
		if documents[position].ID == documentID {
			index.upserts[uid] = append(documents[:position], documents[position+1:]...)
			break
		}
	}
	return nil
}
func (index *recordingIndex) Search(context.Context, string, IndexQuery) (IndexResult, error) {
	return IndexResult{}, index.failure
}
func (index *recordingIndex) Stats(_ context.Context, uid string) (IndexStats, error) {
	return IndexStats{NumberOfDocuments: len(index.upserts[uid])}, index.failure
}
func (index *recordingIndex) SwapIndexes(_ context.Context, first, second string) error {
	index.swaps = append(index.swaps, [2]string{first, second})
	return index.failure
}

func TestReconcileOnceSynchronizesCurrentDesiredState(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createSearchTestDatabase(t, ctx, databaseURL)
	projectID := seedSearchProject(t, ctx, pool)
	if _, err := pool.Exec(ctx, "INSERT INTO project_search_sync (project_id, desired_revision, desired_action, state) VALUES ($1, 7, 'upsert', 'pending')", projectID); err != nil {
		t.Fatalf("seed search sync: %v", err)
	}
	index := &recordingIndex{}
	reconciler := Reconciler{Pool: pool, Index: index, IndexUID: "projects", WorkerID: "test-worker", BatchSize: 10, LeaseDuration: time.Minute}

	processed, err := reconciler.ReconcileOnce(ctx)
	if err != nil || processed != 1 {
		t.Fatalf("ReconcileOnce processed %d and returned %v", processed, err)
	}
	if len(index.upserts["projects"]) != 1 || index.upserts["projects"][0].ID != projectID {
		t.Fatalf("reconciler upserts were %#v", index.upserts)
	}
	assertSyncRow(t, ctx, pool, projectID, "synced", 7, 7)

	if _, err := pool.Exec(ctx, "UPDATE projects SET status='draft', published_at=NULL, revision=8 WHERE id=$1", projectID); err != nil {
		t.Fatalf("make Project private: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE project_search_sync SET desired_revision=8, desired_action='remove', state='pending', updated_at=now() WHERE project_id=$1", projectID); err != nil {
		t.Fatalf("queue removal: %v", err)
	}
	if _, err := reconciler.ReconcileOnce(ctx); err != nil {
		t.Fatalf("removal ReconcileOnce returned an error: %v", err)
	}
	if len(index.deletes["projects"]) != 1 || index.deletes["projects"][0] != projectID {
		t.Fatalf("reconciler deletes were %#v", index.deletes)
	}
	assertSyncRow(t, ctx, pool, projectID, "synced", 8, 8)
}

func TestReconcileOnceRecordsBoundedRetryState(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createSearchTestDatabase(t, ctx, databaseURL)
	projectID := seedSearchProject(t, ctx, pool)
	if _, err := pool.Exec(ctx, "INSERT INTO project_search_sync (project_id, desired_revision, desired_action, state) VALUES ($1, 7, 'upsert', 'pending')", projectID); err != nil {
		t.Fatalf("seed search sync: %v", err)
	}
	reconciler := Reconciler{Pool: pool, Index: &recordingIndex{failure: errors.New("index unavailable")}, WorkerID: "test-worker", BatchSize: 10, LeaseDuration: time.Minute}

	processed, err := reconciler.ReconcileOnce(ctx)
	if processed != 1 || err == nil {
		t.Fatalf("failed ReconcileOnce processed %d and returned %v", processed, err)
	}
	var state string
	var detail string
	var nextAttempt time.Time
	if err := pool.QueryRow(ctx, "SELECT state, error_detail, next_attempt_at FROM project_search_sync WHERE project_id=$1", projectID).Scan(&state, &detail, &nextAttempt); err != nil {
		t.Fatalf("read failed sync state: %v", err)
	}
	if state != "failed" || detail != "index unavailable" || !nextAttempt.After(time.Now()) {
		t.Fatalf("retry state was state=%q detail=%q next=%s", state, detail, nextAttempt)
	}
}

func TestFullRebuildPopulatesPhysicalIndexAndSwapsAtomically(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createSearchTestDatabase(t, ctx, databaseURL)
	projectID := seedSearchProject(t, ctx, pool)
	actorID := uuid.MustParse("018f0000-0000-7000-8000-000000000601")
	if _, err := pool.Exec(ctx, "INSERT INTO project_search_sync (project_id, desired_revision, desired_action, state) VALUES ($1, 7, 'upsert', 'pending')", projectID); err != nil {
		t.Fatalf("seed search sync: %v", err)
	}
	index := &recordingIndex{}
	service := Service{Pool: pool, Index: index, IndexUID: "projects"}
	operation, err := service.CreateRebuild(ctx, actorID)
	if err != nil {
		t.Fatalf("CreateRebuild returned an error: %v", err)
	}
	reconciler := Reconciler{Pool: pool, Index: index, IndexUID: "projects", WorkerID: "test-worker", BatchSize: 10, LeaseDuration: time.Minute}
	processed, err := reconciler.ProcessRebuildOnce(ctx)
	if err != nil || !processed {
		t.Fatalf("ProcessRebuildOnce processed %t and returned %v", processed, err)
	}
	completed, err := service.GetRebuild(ctx, operation.ID)
	if err != nil {
		t.Fatalf("GetRebuild returned an error: %v", err)
	}
	if completed.State != "completed" || completed.TotalProjects != 1 || completed.ProcessedProjects != 1 {
		t.Fatalf("completed rebuild was %#v", completed)
	}
	if len(index.upserts[operation.PhysicalIndexName]) == 0 || len(index.swaps) != 1 || index.swaps[0] != [2]string{"projects", operation.PhysicalIndexName} {
		t.Fatalf("rebuild index state was upserts=%#v swaps=%#v", index.upserts, index.swaps)
	}
	assertSyncRow(t, ctx, pool, projectID, "synced", 7, 7)
}

func assertSyncRow(t *testing.T, ctx context.Context, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID uuid.UUID, expectedState string, expectedDesired, expectedIndexed int64) {
	t.Helper()
	var state string
	var desired int64
	var indexed int64
	if err := pool.QueryRow(ctx, "SELECT state, desired_revision, indexed_revision FROM project_search_sync WHERE project_id=$1", projectID).Scan(&state, &desired, &indexed); err != nil {
		t.Fatalf("read search sync row: %v", err)
	}
	if state != expectedState || desired != expectedDesired || indexed != expectedIndexed {
		t.Fatalf("search sync row was state=%q desired=%d indexed=%d", state, desired, indexed)
	}
}
