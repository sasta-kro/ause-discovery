package artifacts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalStorageCheckReadyProbesAndCleansUp(t *testing.T) {
	root := t.TempDir()
	storage := LocalStorage{Root: root, MaxBytes: 1024}
	if err := storage.CheckReady(context.Background()); err != nil {
		t.Fatalf("CheckReady returned an error: %v", err)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatalf("read probe root: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("readiness probe left %d entries behind", len(entries))
	}
	// A fresh deployment without any upload yet stays ready: the write
	// path's on-demand temporary directory must not be required.
	if err := storage.CheckReady(context.Background()); err != nil {
		t.Fatalf("second CheckReady returned an error: %v", err)
	}
}

func TestLocalStorageCheckReadyRejectsUnusableRoots(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"missing root": func(t *testing.T) string {
			return filepath.Join(t.TempDir(), "absent")
		},
		"root is a file": func(t *testing.T) string {
			path := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
				t.Fatalf("write file: %v", err)
			}
			return path
		},
		"root without write permission": func(t *testing.T) string {
			if os.Geteuid() == 0 {
				t.Skip("root ignores directory permissions")
			}
			root := t.TempDir()
			if err := os.Chmod(root, 0o500); err != nil {
				t.Fatalf("restrict root: %v", err)
			}
			t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
			return root
		},
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			storage := LocalStorage{Root: prepare(t), MaxBytes: 1024}
			if err := storage.CheckReady(context.Background()); !errors.Is(err, ErrStorageUnavailable) {
				t.Fatalf("CheckReady returned %v, expected unavailable", err)
			}
		})
	}
	storage := LocalStorage{Root: "relative/root", MaxBytes: 1024}
	if err := storage.CheckReady(context.Background()); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("relative root returned %v, expected unavailable", err)
	}
}

func TestLocalStorageCheckReadyHonorsCanceledContext(t *testing.T) {
	root := t.TempDir()
	storage := LocalStorage{Root: root, MaxBytes: 1024}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := storage.CheckReady(canceled); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("canceled CheckReady returned %v, expected unavailable", err)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatalf("read probe root: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("canceled check performed probe work, leaving %d entries", len(entries))
	}
}

func TestB2CheckReadyIssuesOneBoundedListRequest(t *testing.T) {
	stub := newS3Stub()
	stub.objects["v1/aa/bb/existing"] = []byte("object")
	storage := newTestB2Storage(t, stub)
	if err := storage.CheckReady(context.Background()); err != nil {
		t.Fatalf("CheckReady returned an error: %v", err)
	}
	requests := stub.recordedListRequests()
	if len(requests) != 1 {
		t.Fatalf("readiness issued %d requests, expected 1", len(requests))
	}
	query := requests[0]
	for _, required := range []string{"list-type=2", "max-keys=1", "prefix=v1%2F"} {
		if !strings.Contains(query, required) {
			t.Fatalf("readiness query %q omitted %q", query, required)
		}
	}
	// An empty bucket is equally healthy: response contents are irrelevant.
	empty := newS3Stub()
	if err := newTestB2Storage(t, empty).CheckReady(context.Background()); err != nil {
		t.Fatalf("CheckReady against an empty bucket returned an error: %v", err)
	}
}

func TestB2CheckReadyClassifiesEveryFailureAsUnavailable(t *testing.T) {
	cases := map[string]func(t *testing.T, stub *s3Stub){
		"authentication failure": func(t *testing.T, stub *s3Stub) { stub.forceStatus(http.StatusForbidden) },
		"missing bucket":         func(t *testing.T, stub *s3Stub) { stub.forceStatus(http.StatusNotFound) },
		"server failure":         func(t *testing.T, stub *s3Stub) { stub.forceStatus(http.StatusInternalServerError) },
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			stub := newS3Stub()
			prepare(t, stub)
			if err := newTestB2Storage(t, stub).CheckReady(context.Background()); !errors.Is(err, ErrStorageUnavailable) {
				t.Fatalf("CheckReady returned %v, expected unavailable", err)
			}
		})
	}

	// Network failure: the endpoint is gone before the request.
	stub := newS3Stub()
	server := httptest.NewServer(stub)
	serverURL := server.URL
	server.Close()
	offline, offlineErr := NewB2Storage(serverURL, "test-bucket", "key-id", "application-key", 1024)
	if offlineErr != nil {
		t.Fatalf("NewB2Storage returned an error: %v", offlineErr)
	}
	if err := offline.CheckReady(context.Background()); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("offline CheckReady returned %v, expected unavailable", err)
	}
}

func TestB2CheckReadyHonorsDeadlineAndCancellation(t *testing.T) {
	stub := newS3Stub()
	release := make(chan struct{})
	stub.delayLists(release)
	storage := newTestB2Storage(t, stub)

	deadlineContext, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := storage.CheckReady(deadlineContext); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("deadline CheckReady returned %v, expected unavailable", err)
	}

	canceled, cancelCanceled := context.WithCancel(context.Background())
	cancelCanceled()
	if err := storage.CheckReady(canceled); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("canceled CheckReady returned %v, expected unavailable", err)
	}
	// Drain any delayed handler still holding a request slot.
	close(release)
}

type manualClock struct {
	current time.Time
}

func (clock *manualClock) Now() time.Time { return clock.current }

func (clock *manualClock) advance(duration time.Duration) {
	clock.current = clock.current.Add(duration)
}

func TestB2CheckReadyCachesSuccessAndFailure(t *testing.T) {
	stub := newS3Stub()
	storage := newTestB2Storage(t, stub)
	clock := &manualClock{current: time.Now()}
	storage.readiness.now = clock.Now

	for index := 0; index < 3; index++ {
		if err := storage.CheckReady(context.Background()); err != nil {
			t.Fatalf("CheckReady %d returned an error: %v", index+1, err)
		}
	}
	if requests := stub.recordedListRequests(); len(requests) != 1 {
		t.Fatalf("cached window issued %d requests, expected 1", len(requests))
	}

	// Expiring the success window starts one new request.
	clock.advance(readinessSuccessTTL)
	if err := storage.CheckReady(context.Background()); err != nil {
		t.Fatalf("expired-window CheckReady returned an error: %v", err)
	}
	if requests := stub.recordedListRequests(); len(requests) != 2 {
		t.Fatalf("after expiry issued %d requests, expected 2", len(requests))
	}

	// Failures cache for the shorter window and recover promptly. The
	// terminal 403 keeps the SDK from retrying, so request counts stay
	// deterministic.
	stub.forceStatus(http.StatusForbidden)
	clock.advance(readinessSuccessTTL)
	if err := storage.CheckReady(context.Background()); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("failing CheckReady returned %v, expected unavailable", err)
	}
	for index := 0; index < 2; index++ {
		if err := storage.CheckReady(context.Background()); !errors.Is(err, ErrStorageUnavailable) {
			t.Fatalf("cached failure check %d returned %v", index+1, err)
		}
	}
	if requests := stub.recordedListRequests(); len(requests) != 3 {
		t.Fatalf("failure window issued %d requests, expected 3", len(requests))
	}

	stub.forceStatus(0)
	clock.advance(readinessFailureTTL)
	if err := storage.CheckReady(context.Background()); err != nil {
		t.Fatalf("recovered CheckReady returned an error: %v", err)
	}
	if requests := stub.recordedListRequests(); len(requests) != 4 {
		t.Fatalf("recovery issued %d requests, expected 4", len(requests))
	}
}

func TestB2CheckReadyCoalescesConcurrentCalls(t *testing.T) {
	stub := newS3Stub()
	release := make(chan struct{})
	stub.delayLists(release)
	storage := newTestB2Storage(t, stub)

	const waiters = 4
	results := make(chan error, waiters+1)
	leaderDone := make(chan struct{})
	go func() {
		results <- storage.CheckReady(context.Background())
		close(leaderDone)
	}()
	// Let the leader reach the blocked request before starting waiters.
	deadline := time.Now().Add(time.Second)
	for len(stub.recordedListRequests()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	var waiterGroup sync.WaitGroup
	for index := 0; index < waiters; index++ {
		waiterGroup.Add(1)
		go func() {
			defer waiterGroup.Done()
			results <- storage.CheckReady(context.Background())
		}()
	}
	close(release)
	waiterGroup.Wait()
	<-leaderDone
	for index := 0; index < waiters+1; index++ {
		if err := <-results; err != nil {
			t.Fatalf("coalesced check returned %v", err)
		}
	}
	if requests := stub.recordedListRequests(); len(requests) != 1 {
		t.Fatalf("concurrent checks issued %d requests, expected 1", len(requests))
	}
}

func TestB2CheckReadyCanceledWaiterDoesNotDisturbLeader(t *testing.T) {
	stub := newS3Stub()
	release := make(chan struct{})
	stub.delayLists(release)
	storage := newTestB2Storage(t, stub)

	leaderResult := make(chan error, 1)
	go func() { leaderResult <- storage.CheckReady(context.Background()) }()
	deadline := time.Now().Add(time.Second)
	for len(stub.recordedListRequests()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	waiterContext, cancel := context.WithCancel(context.Background())
	waiterResult := make(chan error, 1)
	go func() { waiterResult <- storage.CheckReady(waiterContext) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-waiterResult; !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("canceled waiter returned %v, expected unavailable", err)
	}

	close(release)
	if err := <-leaderResult; err != nil {
		t.Fatalf("leader returned %v after its waiter canceled", err)
	}
	if requests := stub.recordedListRequests(); len(requests) != 1 {
		t.Fatalf("leader and waiter issued %d requests, expected 1", len(requests))
	}
}

func TestStorageSetCheckReadyChecksOnlyTheDefaultProvider(t *testing.T) {
	root := t.TempDir()
	local := LocalStorage{Root: root, MaxBytes: 1024}
	failing := &failingReadyBackend{LocalStorage: local}
	set := StorageSet{DefaultName: BackendLocal, Backends: map[string]Backend{
		BackendLocal: local,
		BackendB2:    failing,
	}}
	if err := set.CheckReady(context.Background()); err != nil {
		t.Fatalf("secondary provider failure made the default provider unready: %v", err)
	}

	failingDefault := StorageSet{DefaultName: BackendB2, Backends: map[string]Backend{
		BackendLocal: local,
		BackendB2:    failing,
	}}
	if err := failingDefault.CheckReady(context.Background()); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("failing default provider returned %v, expected unavailable", err)
	}

	missing := StorageSet{DefaultName: BackendB2, Backends: map[string]Backend{BackendLocal: local}}
	if err := missing.CheckReady(context.Background()); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("unconfigured default provider returned %v, expected unavailable", err)
	}
}

type failingReadyBackend struct {
	LocalStorage
	calls int64
}

func (backend *failingReadyBackend) Name() string { return BackendB2 }

func (backend *failingReadyBackend) CheckReady(ctx context.Context) error {
	atomic.AddInt64(&backend.calls, 1)
	return ErrStorageUnavailable
}
