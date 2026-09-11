package artifactimport

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testProjectGroups(count int) []projectWork {
	groups := make([]projectWork, count)
	for index := range groups {
		groups[index] = projectWork{ID: uuid.New(), Name: "Project"}
	}
	return groups
}

func TestNormalizeWorkersDefaultsAndClamps(t *testing.T) {
	cases := map[int]int{
		0:  DefaultWorkers,
		-3: DefaultWorkers,
		1:  1,
		4:  4,
		8:  MaxWorkers,
		9:  MaxWorkers,
	}
	for input, expected := range cases {
		if normalized := normalizeWorkers(input); normalized != expected {
			t.Fatalf("normalizeWorkers(%d) returned %d, expected %d", input, normalized, expected)
		}
	}
}

func TestGroupByProjectPreservesOrder(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	plan := []plannedUpload{
		{ProjectID: first, ProjectName: "first"},
		{ProjectID: second, ProjectName: "second"},
		{ProjectID: first, ProjectName: "first"},
		{ProjectID: second, ProjectName: "second"},
		{ProjectID: first, ProjectName: "first"},
	}
	groups := groupByProject(plan)
	if len(groups) != 2 {
		t.Fatalf("groupByProject produced %d groups, expected 2", len(groups))
	}
	if groups[0].ID != first || groups[1].ID != second {
		t.Fatalf("groupByProject dispatched in order %v %v", groups[0].ID, groups[1].ID)
	}
	if len(groups[0].Uploads) != 3 || len(groups[1].Uploads) != 2 {
		t.Fatalf("groupByProject split files %d and %d, expected 3 and 2", len(groups[0].Uploads), len(groups[1].Uploads))
	}
	for index, upload := range groups[0].Uploads {
		if upload.ProjectID != first || upload.ProjectName != "first" {
			t.Fatalf("first group upload %d lost its Project identity", index)
		}
	}
}

func TestDispatchBoundsConcurrencyAndSerializesCallbacks(t *testing.T) {
	for _, workers := range []int{1, 4} {
		groups := testProjectGroups(16)
		var active, maxActive int64
		process := func(ctx context.Context, work projectWork) projectOutcome {
			current := atomic.AddInt64(&active, 1)
			for {
				observed := atomic.LoadInt64(&maxActive)
				if current <= observed || atomic.CompareAndSwapInt64(&maxActive, observed, current) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt64(&active, -1)
			return projectOutcome{Progress: ProjectProgress{ProjectID: work.ID}}
		}
		// apply runs only inside the coordinator goroutine, so plain
		// mutation here is safe by design; the race detector validates it.
		seen := map[uuid.UUID]bool{}
		completedPositions := []int{}
		_, canceled := dispatchProjects(context.Background(), groups, workers, process, func(outcome projectOutcome) {
			seen[outcome.Progress.ProjectID] = true
			completedPositions = append(completedPositions, outcome.Progress.Completed)
		})
		if canceled {
			t.Fatalf("workers=%d run reported cancellation", workers)
		}
		if len(seen) != len(groups) {
			t.Fatalf("workers=%d produced %d completion callbacks, expected %d", workers, len(seen), len(groups))
		}
		for position, value := range completedPositions {
			if value != position+1 {
				t.Fatalf("completion positions were %v, expected 1..%d", completedPositions, len(groups))
			}
		}
		if observed := atomic.LoadInt64(&maxActive); observed > int64(workers) {
			t.Fatalf("workers=%d observed %d concurrent Projects", workers, observed)
		}
		if workers > 1 && atomic.LoadInt64(&maxActive) < 2 {
			t.Fatalf("workers=%d never overlapped two Projects", workers)
		}
		if workers == 1 && atomic.LoadInt64(&maxActive) != 1 {
			t.Fatalf("workers=1 observed %d concurrent Projects", maxActive)
		}
	}
}

func TestDispatchFatalOutcomeStopsFurtherProjects(t *testing.T) {
	groups := testProjectGroups(8)
	fatalID := groups[0].ID
	processed := map[uuid.UUID]bool{}
	process := func(ctx context.Context, work projectWork) projectOutcome {
		if work.ID == fatalID {
			return projectOutcome{Progress: ProjectProgress{ProjectID: work.ID}, Failed: true, Fatal: true, FirstError: "storage unavailable"}
		}
		// Normal Projects occupy their worker long enough that the fatal
		// outcome reaches the coordinator and stops dispatch before the
		// feeder can hand out every remaining group.
		time.Sleep(30 * time.Millisecond)
		return projectOutcome{Progress: ProjectProgress{ProjectID: work.ID}}
	}
	fatalSeen := false
	firstError, canceled := dispatchProjects(context.Background(), groups, 1, process, func(outcome projectOutcome) {
		processed[outcome.Progress.ProjectID] = true
		if outcome.Progress.ProjectID == fatalID {
			fatalSeen = true
		}
	})
	if !canceled {
		t.Fatal("fatal outcome did not report cancellation")
	}
	if firstError != "storage unavailable" {
		t.Fatalf("first error was %q", firstError)
	}
	if !fatalSeen {
		t.Fatal("fatal Project never completed")
	}
	if len(processed) >= len(groups) {
		t.Fatalf("dispatch ran all %d Projects despite the fatal outcome", len(groups))
	}
}

func TestDispatchContextCancellationStopsWithoutLeakingGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	contextContext, cancel := context.WithCancel(context.Background())
	groups := testProjectGroups(12)
	runs := int64(0)
	process := func(ctx context.Context, work projectWork) projectOutcome {
		if atomic.AddInt64(&runs, 1) == 1 {
			cancel()
		}
		time.Sleep(5 * time.Millisecond)
		return projectOutcome{Progress: ProjectProgress{ProjectID: work.ID}}
	}
	_, canceled := dispatchProjects(contextContext, groups, 2, process, func(outcome projectOutcome) {})
	if !canceled {
		t.Fatal("context cancellation did not report cancellation")
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if remaining := runtime.NumGoroutine(); remaining > before {
		t.Fatalf("dispatch leaked goroutines: %d before, %d after", before, remaining)
	}
}
