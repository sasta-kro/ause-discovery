package projectpool

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testWorks(count int) []Work[int] {
	works := make([]Work[int], count)
	for index := range works {
		works[index] = Work[int]{ID: uuid.New(), Title: "Group", Body: index}
	}
	return works
}

func TestNormalizeDefaultsAndClamps(t *testing.T) {
	cases := map[int]int{0: DefaultWorkers, -3: DefaultWorkers, 1: 1, 4: 4, 8: MaxWorkers, 9: MaxWorkers}
	for input, expected := range cases {
		if normalized := Normalize(input); normalized != expected {
			t.Fatalf("Normalize(%d) returned %d, expected %d", input, normalized, expected)
		}
	}
}

func TestDispatchBoundsConcurrencyAndSerializesCallbacks(t *testing.T) {
	for _, workers := range []int{1, 4} {
		works := testWorks(16)
		var active, maxActive int64
		process := func(ctx context.Context, work Work[int]) Outcome[int] {
			current := atomic.AddInt64(&active, 1)
			for {
				observed := atomic.LoadInt64(&maxActive)
				if current <= observed || atomic.CompareAndSwapInt64(&maxActive, observed, current) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt64(&active, -1)
			return Outcome[int]{ID: work.ID}
		}
		// apply runs only inside the coordinator goroutine, so plain
		// mutation here is safe by design; the race detector validates it.
		seen := map[uuid.UUID]bool{}
		completedPositions := []int{}
		_, canceled := Dispatch(context.Background(), works, workers, process, func(outcome Outcome[int]) {
			seen[outcome.ID] = true
			completedPositions = append(completedPositions, outcome.Completed)
		})
		if canceled {
			t.Fatalf("workers=%d run reported cancellation", workers)
		}
		if len(seen) != len(works) {
			t.Fatalf("workers=%d produced %d completion callbacks, expected %d", workers, len(seen), len(works))
		}
		for position, value := range completedPositions {
			if value != position+1 {
				t.Fatalf("completion positions were %v, expected 1..%d", completedPositions, len(works))
			}
		}
		if observed := atomic.LoadInt64(&maxActive); observed > int64(workers) {
			t.Fatalf("workers=%d observed %d concurrent groups", workers, observed)
		}
		if workers > 1 && atomic.LoadInt64(&maxActive) < 2 {
			t.Fatalf("workers=%d never overlapped two groups", workers)
		}
		if workers == 1 && atomic.LoadInt64(&maxActive) != 1 {
			t.Fatalf("workers=1 observed %d concurrent groups", maxActive)
		}
	}
}

func TestDispatchFatalOutcomeStopsFurtherAssignments(t *testing.T) {
	works := testWorks(16)
	fatalID := works[0].ID
	release := make(chan struct{})
	process := func(ctx context.Context, work Work[int]) Outcome[int] {
		if work.ID == fatalID {
			return Outcome[int]{ID: work.ID, Failed: true, Fatal: true, FirstError: "storage unavailable"}
		}
		<-release
		return Outcome[int]{ID: work.ID}
	}
	processed := map[uuid.UUID]bool{}
	firstError, canceled := Dispatch(context.Background(), works, 4, process, func(outcome Outcome[int]) {
		processed[outcome.ID] = true
		if outcome.Fatal {
			close(release)
		}
	})
	if !canceled {
		t.Fatal("fatal outcome did not report cancellation")
	}
	if firstError != "storage unavailable" {
		t.Fatalf("first error was %q", firstError)
	}
	if !processed[fatalID] {
		t.Fatal("fatal group never completed")
	}
	if len(processed) != 4 {
		t.Fatalf("dispatch processed %d groups, expected exactly the 4 primed assignments and no replacements", len(processed))
	}
}

func TestDispatchParentCancellationStopsReplacementAssignment(t *testing.T) {
	works := testWorks(12)
	parentContext, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	runs := 0
	process := func(ctx context.Context, work Work[int]) Outcome[int] {
		runs++
		cancelParent()
		return Outcome[int]{ID: work.ID}
	}
	completed := 0
	_, canceled := Dispatch(parentContext, works, 1, process, func(outcome Outcome[int]) {
		completed++
	})
	if !canceled {
		t.Fatal("parent cancellation did not mark the run canceled")
	}
	if runs != 1 || completed != 1 {
		t.Fatalf("dispatch ran %d and completed %d groups, expected exactly the 1 primed assignment", runs, completed)
	}
}

func TestDispatchPreflightCanceledContextRunsNothing(t *testing.T) {
	works := testWorks(6)
	parentContext, cancelParent := context.WithCancel(context.Background())
	cancelParent()
	runs := 0
	process := func(ctx context.Context, work Work[int]) Outcome[int] {
		runs++
		return Outcome[int]{ID: work.ID}
	}
	completed := 0
	_, canceled := Dispatch(parentContext, works, 4, process, func(outcome Outcome[int]) {
		completed++
	})
	if !canceled {
		t.Fatal("pre-canceled context did not mark the run canceled")
	}
	if runs != 0 || completed != 0 {
		t.Fatalf("dispatch ran %d and completed %d groups on a pre-canceled context", runs, completed)
	}
}

func TestDispatchContextCancellationStopsWithoutLeakingGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	runContext, cancel := context.WithCancel(context.Background())
	works := testWorks(12)
	runs := int64(0)
	process := func(ctx context.Context, work Work[int]) Outcome[int] {
		if atomic.AddInt64(&runs, 1) == 1 {
			cancel()
		}
		time.Sleep(5 * time.Millisecond)
		return Outcome[int]{ID: work.ID}
	}
	_, canceled := Dispatch(runContext, works, 2, process, func(outcome Outcome[int]) {})
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
