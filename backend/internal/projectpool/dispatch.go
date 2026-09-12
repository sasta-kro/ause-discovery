// Package projectpool provides the bounded Project-level worker pool shared
// by the bulk importers. It is the smallest scheduling and serialization
// helper extracted from the accepted Project File importer: groups are
// dispatched concurrently with a fixed bound, files within one group run
// sequentially inside its worker, and a single coordinator goroutine
// serializes outcome handling so callbacks never overlap.
package projectpool

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultWorkers = 4
	MaxWorkers     = 8
)

// Normalize maps an unset or out-of-range worker count onto the bounded
// default.
func Normalize(count int) int {
	if count < 1 {
		return DefaultWorkers
	}
	if count > MaxWorkers {
		return MaxWorkers
	}
	return count
}

// Work is one dispatchable group carrying caller-defined payload.
type Work[T any] struct {
	ID    uuid.UUID
	Title string
	Body  T
}

// Outcome is the worker result for one group. Completed and Total are
// assigned by Dispatch in completion order.
type Outcome[T any] struct {
	ID         uuid.UUID
	Title      string
	Completed  int
	Total      int
	Failed     bool
	Fatal      bool
	FirstError string
	Duration   time.Duration
	Body       T
}

// Dispatch runs a fixed worker pool over groups. The single coordinator
// goroutine owns assignment: it primes the pool and replaces each completed
// assignment with the next group, re-checking run-context cancellation
// before every assignment, so neither a pre-canceled context nor a parent
// canceled mid-run can start further groups, and no assignment follows a
// fatal outcome reaching the coordinator. Cancellation and fatal outcomes
// stop assignment but keep collecting outcomes from already-running groups.
// Workers process one group at a time through process and deliver outcomes
// through a channel buffered for every group, so they can always deliver
// and exit without leaking. The coordinator assigns completion positions
// and folds results through apply, which therefore never runs concurrently.
// It returns the first failure text and whether the run stopped early.
func Dispatch[WorkPayload any, OutcomePayload any](ctx context.Context, works []Work[WorkPayload], workers int, process func(context.Context, Work[WorkPayload]) Outcome[OutcomePayload], apply func(Outcome[OutcomePayload])) (string, bool) {
	if len(works) == 0 {
		return "", false
	}
	if workers < 1 {
		workers = 1
	}
	runContext, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan Work[WorkPayload])
	outcomes := make(chan Outcome[OutcomePayload], len(works))
	var workersDone sync.WaitGroup
	for worker := 0; worker < workers && worker < len(works); worker++ {
		workersDone.Add(1)
		go func() {
			defer workersDone.Done()
			for work := range jobs {
				outcomes <- process(runContext, work)
			}
		}()
	}

	firstError := ""
	canceled := false
	completed := 0
	nextWork := 0
	jobsClosed := false
	stopAssigning := func() {
		if !jobsClosed {
			jobsClosed = true
			close(jobs)
		}
	}
	coordinatorDone := make(chan struct{})
	go func() {
		defer close(coordinatorDone)
		for nextWork < len(works) && nextWork < workers && runContext.Err() == nil {
			jobs <- works[nextWork]
			nextWork++
		}
		if nextWork == len(works) || runContext.Err() != nil {
			if runContext.Err() != nil {
				canceled = true
			}
			stopAssigning()
		}
		for outcome := range outcomes {
			completed++
			outcome.Completed = completed
			outcome.Total = len(works)
			if outcome.Failed && firstError == "" {
				firstError = outcome.FirstError
			}
			apply(outcome)
			if outcome.Fatal {
				canceled = true
				cancel()
				stopAssigning()
				continue
			}
			if runContext.Err() != nil {
				canceled = true
				stopAssigning()
				continue
			}
			if nextWork < len(works) && !jobsClosed {
				jobs <- works[nextWork]
				nextWork++
				if nextWork == len(works) {
					stopAssigning()
				}
			}
		}
	}()

	workersDone.Wait()
	close(outcomes)
	<-coordinatorDone
	if ctx.Err() != nil {
		canceled = true
	}
	return firstError, canceled
}
