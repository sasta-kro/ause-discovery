package artifacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// s3Stub is a minimal S3-compatible endpoint for exercising the B2 backend
// without network access. It implements the path-style requests the client
// issues: PUT, HEAD, ranged and full GET, and DELETE.
type s3Stub struct {
	mutex         sync.Mutex
	objects       map[string][]byte
	forcedStatus  int
	rangeRequests []string
	listRequests  []string
	listHeaders   []string
	listDelay     chan struct{}
	listArrival   chan struct{}
	listBody      string
	malformedList bool
}

func newS3Stub() *s3Stub {
	return &s3Stub{objects: map[string][]byte{}}
}

func (stub *s3Stub) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Query().Get("list-type") == "2" {
		stub.serveList(writer, request)
		return
	}
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	if stub.forcedStatus != 0 {
		writer.WriteHeader(stub.forcedStatus)
		return
	}
	key := strings.TrimPrefix(request.URL.Path, "/test-bucket/")
	if key == "" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	switch request.Method {
	case http.MethodPut:
		content, err := io.ReadAll(request.Body)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		stub.objects[key] = content
		writer.WriteHeader(http.StatusOK)
	case http.MethodHead:
		content, ok := stub.objects[key]
		if !ok {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		writer.WriteHeader(http.StatusOK)
	case http.MethodGet:
		content, ok := stub.objects[key]
		if !ok {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if rangeHeader := request.Header.Get("Range"); rangeHeader != "" {
			stub.rangeRequests = append(stub.rangeRequests, rangeHeader)
			start := 0
			if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start); err != nil || start < 0 || start > len(content) {
				writer.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(content)-1, len(content)))
			writer.WriteHeader(http.StatusPartialContent)
			_, _ = writer.Write(content[start:])
			return
		}
		writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(content)
	case http.MethodDelete:
		delete(stub.objects, key)
		writer.WriteHeader(http.StatusNoContent)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// serveList answers one S3 ListObjectsV2 request. The delayed variant holds
// the response until released so tests can exercise leaders, waiters, and
// cancellation deterministically.
func (stub *s3Stub) serveList(writer http.ResponseWriter, request *http.Request) {
	// Announce arrival before any delay so tests know the leader request
	// has started, then gate the response when a delay is configured.
	if arrival := stub.listArrival; arrival != nil {
		select {
		case arrival <- struct{}{}:
		default:
		}
	}
	stub.mutex.Lock()
	if stub.listDelay != nil {
		stub.mutex.Unlock()
		select {
		case <-stub.listDelay:
		case <-request.Context().Done():
			return
		}
		stub.mutex.Lock()
	}
	stub.listRequests = append(stub.listRequests, request.URL.RawQuery)
	stub.listHeaders = append(stub.listHeaders, request.Header.Get("Authorization"))
	forced := stub.forcedStatus
	body := stub.listBody
	malformed := stub.malformedList
	stub.mutex.Unlock()
	if forced != 0 {
		writer.WriteHeader(forced)
		return
	}
	writer.Header().Set("Content-Type", "application/xml")
	writer.WriteHeader(http.StatusOK)
	if malformed {
		_, _ = writer.Write([]byte(`<?xml version="1.0"?><ListBucketResult><Name>broken`))
		return
	}
	if body == "" {
		body = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>test-bucket</Name><IsTruncated>false</IsTruncated><MaxKeys>1</MaxKeys><Prefix>v1/</Prefix></ListBucketResult>`
	}
	_, _ = writer.Write([]byte(body))
}

func (stub *s3Stub) forceStatus(status int) {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	stub.forcedStatus = status
}

func (stub *s3Stub) recordedListRequests() []string {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	return append([]string(nil), stub.listRequests...)
}

func (stub *s3Stub) delayLists(release chan struct{}) {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	stub.listDelay = release
}

// signalListArrival registers a channel that receives once for every
// ListObjectsV2 request the stub accepts, before any configured delay.
func (stub *s3Stub) signalListArrival(arrival chan struct{}) {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	stub.listArrival = arrival
}

func (stub *s3Stub) forceMalformedList() {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	stub.malformedList = true
}

func (stub *s3Stub) recordedListHeaders() []string {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	return append([]string(nil), stub.listHeaders...)
}

func (stub *s3Stub) storedContent(key string) ([]byte, bool) {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	content, ok := stub.objects[key]
	return content, ok
}

func (stub *s3Stub) recordedRanges() []string {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	return append([]string(nil), stub.rangeRequests...)
}

func newTestB2Storage(t *testing.T, stub *s3Stub) *B2Storage {
	t.Helper()
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	storage, err := NewB2Storage(server.URL, "test-bucket", "key-id", "application-key", 1024)
	if err != nil {
		t.Fatalf("NewB2Storage returned an error: %v", err)
	}
	return storage
}

func TestB2PutOpenExistsAndRemoveNew(t *testing.T) {
	stub := newS3Stub()
	storage := newTestB2Storage(t, stub)
	content := []byte("%PDF-1.7\nartifact content")

	stored, err := storage.Put(context.Background(), bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Put returned an error: %v", err)
	}
	if !strings.HasPrefix(stored.StorageKey, "v1/") {
		t.Fatalf("Put returned unexpected storage key %q", stored.StorageKey)
	}
	if stored.ByteCount != int64(len(content)) {
		t.Fatalf("Put stored %d bytes, expected %d", stored.ByteCount, len(content))
	}
	expectedDigest := sha256.Sum256(content)
	if stored.SHA256 != expectedDigest {
		t.Fatalf("Put returned digest %x, expected %x", stored.SHA256, expectedDigest)
	}
	uploaded, ok := stub.storedContent(stored.StorageKey)
	if !ok || !bytes.Equal(uploaded, content) {
		t.Fatalf("stub stored %q for key %q", uploaded, stored.StorageKey)
	}

	if !storage.Exists(context.Background(), stored.StorageKey) {
		t.Fatal("Exists returned false for stored content")
	}
	if storage.Exists(context.Background(), "v1/00/00/missing") {
		t.Fatal("Exists returned true for missing content")
	}

	reader, size, err := storage.Open(context.Background(), stored.StorageKey)
	if err != nil {
		t.Fatalf("Open returned an error: %v", err)
	}
	defer reader.Close()
	if size != int64(len(content)) {
		t.Fatalf("Open returned size %d, expected %d", size, len(content))
	}
	opened, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(opened, content) {
		t.Fatalf("full read returned %q with error %v", opened, err)
	}

	if err := storage.RemoveNew(context.Background(), stored.StorageKey); err != nil {
		t.Fatalf("RemoveNew returned an error: %v", err)
	}
	if storage.Exists(context.Background(), stored.StorageKey) {
		t.Fatal("content still exists after RemoveNew")
	}
	if err := storage.RemoveNew(context.Background(), stored.StorageKey); err != nil {
		t.Fatalf("RemoveNew of already-removed content returned %v, expected tolerance", err)
	}
}

func TestB2RangedReaderSeekIssuesRangedRequests(t *testing.T) {
	stub := newS3Stub()
	storage := newTestB2Storage(t, stub)
	content := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	stored, err := storage.Put(context.Background(), bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Put returned an error: %v", err)
	}

	reader, size, err := storage.Open(context.Background(), stored.StorageKey)
	if err != nil {
		t.Fatalf("Open returned an error: %v", err)
	}
	defer reader.Close()
	if size != int64(len(content)) {
		t.Fatalf("Open returned size %d, expected %d", size, len(content))
	}

	offset, err := reader.Seek(10, io.SeekStart)
	if err != nil || offset != 10 {
		t.Fatalf("Seek returned offset %d and error %v", offset, err)
	}
	window := make([]byte, 6)
	if _, err := io.ReadFull(reader, window); err != nil {
		t.Fatalf("ranged read returned an error: %v", err)
	}
	if string(window) != "KLMNOP" {
		t.Fatalf("ranged read returned %q, expected KLMNOP", window)
	}

	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek to start returned an error: %v", err)
	}
	full, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(full, content) {
		t.Fatalf("full read after seek returned %d bytes with error %v", len(full), err)
	}

	ranges := stub.recordedRanges()
	if len(ranges) != 2 || ranges[0] != "bytes=10-" || ranges[1] != "bytes=0-" {
		t.Fatalf("stub recorded ranges %v, expected one ranged request then one from zero", ranges)
	}
}

func TestB2OpenClassifiesUpstreamErrors(t *testing.T) {
	stub := newS3Stub()
	storage := newTestB2Storage(t, stub)

	if _, _, err := storage.Open(context.Background(), "v1/00/00/missing"); !errors.Is(err, ErrContentUnavailable) {
		t.Fatalf("missing Open returned %v, expected content unavailable", err)
	}

	stub.forceStatus(http.StatusInternalServerError)
	if _, _, err := storage.Open(context.Background(), "v1/00/00/anything"); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("upstream failure Open returned %v, expected storage unavailable", err)
	}
}

func TestB2PutRejectsInvalidSizes(t *testing.T) {
	storage := newTestB2Storage(t, newS3Stub())
	storage.MaxBytes = 4

	if _, err := storage.Put(context.Background(), bytes.NewReader(nil), 0); !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("empty Put returned %v, expected empty content error", err)
	}
	if _, err := storage.Put(context.Background(), bytes.NewReader([]byte("12345")), 5); !errors.Is(err, ErrContentTooLarge) {
		t.Fatalf("large Put returned %v, expected content too large error", err)
	}
	if _, err := storage.Put(context.Background(), bytes.NewReader([]byte("123")), 4); !errors.Is(err, ErrSizeMismatch) {
		t.Fatalf("mismatched Put returned %v, expected size mismatch error", err)
	}
}

func TestB2RejectsUnsafeKeys(t *testing.T) {
	storage := newTestB2Storage(t, newS3Stub())
	for _, key := range []string{"", "/absolute", "v1/../secret", "v1//double", "v1/.."} {
		if _, _, err := storage.Open(context.Background(), key); !errors.Is(err, ErrUnsafeStoragePath) {
			t.Fatalf("Open of unsafe key %q returned %v, expected unsafe path error", key, err)
		}
	}
}

func TestB2RegionFromEndpoint(t *testing.T) {
	cases := map[string]string{
		"s3.us-west-004.backblazeb2.com":              "us-west-004",
		"https://s3.eu-central-010.backblazeb2.com":   "eu-central-010",
		"https://s3.us-west-004.backblazeb2.com/path": "us-west-004",
		"127.0.0.1:8080": "127",
		"":               "us-west-004",
	}
	for endpoint, expected := range cases {
		if region := b2RegionFromEndpoint(endpoint); region != expected {
			t.Fatalf("region for %q was %q, expected %q", endpoint, region, expected)
		}
	}
}
