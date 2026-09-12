package artifacts

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
)

// B2Storage stores Artifact bytes in a private Backblaze B2 bucket through
// its S3-compatible API (AD-011). Visitors never contact the bucket: the
// API proxies every read. The shared readiness probe coalesces and caches
// bucket checks so unauthenticated readiness traffic cannot open one B2
// request per call.
type B2Storage struct {
	Bucket    string
	Client    *s3.Client
	MaxBytes  int64
	readiness *readinessProbe
}

func NewB2Storage(endpoint, bucket, keyID, applicationKey string, maxBytes int64) (*B2Storage, error) {
	endpoint = strings.TrimSpace(endpoint)
	bucket = strings.TrimSpace(bucket)
	keyID = strings.TrimSpace(keyID)
	applicationKey = strings.TrimSpace(applicationKey)
	if endpoint == "" || bucket == "" || keyID == "" || applicationKey == "" {
		return nil, errors.New("B2 artifact storage requires endpoint, bucket, key ID, and application key")
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	awsConfig := aws.Config{
		Region:      b2RegionFromEndpoint(endpoint),
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(keyID, applicationKey, "")),
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	storage := &B2Storage{Bucket: bucket, Client: client, MaxBytes: maxBytes}
	storage.readiness = newReadinessProbe(storage.listReady)
	return storage, nil
}

// b2RegionFromEndpoint derives the SigV4 signing region from a B2 S3
// endpoint host, for example s3.us-west-004.backblazeb2.com becomes
// us-west-004.
func b2RegionFromEndpoint(endpoint string) string {
	host := endpoint
	if index := strings.Index(host, "://"); index >= 0 {
		host = host[index+3:]
	}
	if index := strings.Index(host, "/"); index >= 0 {
		host = host[:index]
	}
	host = strings.TrimPrefix(host, "s3.")
	if index := strings.Index(host, "."); index >= 0 {
		host = host[:index]
	}
	if host == "" {
		return "us-west-004"
	}
	return host
}

func (storage B2Storage) Name() string { return BackendB2 }

func (storage B2Storage) Put(ctx context.Context, reader io.Reader, expectedSize int64) (StoredContent, error) {
	contentID := uuid.NewString()
	storageKey := "v1/" + contentID[0:2] + "/" + contentID[2:4] + "/" + contentID
	return storage.PutAt(ctx, storageKey, reader, expectedSize)
}

func (storage B2Storage) PutAt(ctx context.Context, storageKey string, reader io.Reader, expectedSize int64) (StoredContent, error) {
	if expectedSize == 0 {
		return StoredContent{}, ErrEmptyContent
	}
	if storage.MaxBytes <= 0 {
		return StoredContent{}, errors.New("artifact storage limit must be positive")
	}
	if expectedSize > storage.MaxBytes {
		return StoredContent{}, ErrContentTooLarge
	}
	if err := validateB2StorageKey(storageKey); err != nil {
		return StoredContent{}, err
	}

	// Stage to a temporary file so digest, detected MIME, size checks, and
	// the upload length are computed from the exact bytes, mirroring the
	// local backend's guarantees before anything reaches the bucket.
	temporaryFile, err := os.CreateTemp("", "ause-b2-*")
	if err != nil {
		return StoredContent{}, fmt.Errorf("create artifact temporary file: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer func() {
		_ = temporaryFile.Close()
		_ = os.Remove(temporaryPath)
	}()

	hasher := sha256.New()
	prefix := &prefixWriter{limit: 512}
	limitedReader := io.LimitReader(&contextReader{ctx: ctx, reader: reader}, storage.MaxBytes+1)
	byteCount, err := io.Copy(io.MultiWriter(temporaryFile, hasher, prefix), limitedReader)
	if err != nil {
		return StoredContent{}, fmt.Errorf("store artifact content: %w", err)
	}
	if byteCount == 0 {
		return StoredContent{}, ErrEmptyContent
	}
	if byteCount > storage.MaxBytes {
		return StoredContent{}, ErrContentTooLarge
	}
	if expectedSize >= 0 && byteCount != expectedSize {
		return StoredContent{}, ErrSizeMismatch
	}
	if _, err := temporaryFile.Seek(0, io.SeekStart); err != nil {
		return StoredContent{}, fmt.Errorf("rewind staged artifact content: %w", err)
	}

	_, err = storage.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(storage.Bucket),
		Key:           aws.String(storageKey),
		Body:          temporaryFile,
		ContentLength: aws.Int64(byteCount),
	})
	if err != nil {
		return StoredContent{}, classifyB2Error(err)
	}

	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return StoredContent{
		StorageKey:   storageKey,
		ByteCount:    byteCount,
		SHA256:       digest,
		DetectedMIME: http.DetectContentType(prefix.content),
		Prefix:       append([]byte(nil), prefix.content...),
	}, nil
}

func (storage B2Storage) Open(ctx context.Context, storageKey string) (io.ReadSeekCloser, int64, error) {
	if err := validateB2StorageKey(storageKey); err != nil {
		return nil, 0, err
	}
	output, err := storage.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(storage.Bucket),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		return nil, 0, classifyB2Error(err)
	}
	if output.ContentLength == nil {
		return nil, 0, fmt.Errorf("%w: missing content length for %q", ErrStorageUnavailable, storageKey)
	}
	size := *output.ContentLength
	reader := &rangedReader{ctx: ctx, client: storage.Client, bucket: storage.Bucket, key: storageKey, size: size}
	return reader, size, nil
}

func (storage B2Storage) Exists(ctx context.Context, storageKey string) bool {
	_, err := storage.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(storage.Bucket),
		Key:    aws.String(storageKey),
	})
	return err == nil
}

// CheckReady proves the configured endpoint, credentials, bucket access, and
// bucket-scoped list capability with one read-only ListObjectsV2 request
// bounded to the application-owned v1/ key namespace. An empty bucket is
// healthy; response contents are irrelevant. The probe is coalesced and
// briefly cached by the shared readiness controller.
func (storage *B2Storage) CheckReady(ctx context.Context) error {
	// NewB2Storage is the supported construction path and always wires the
	// controller. Anything else is a programming error, reported as a
	// controlled unavailability rather than a panic.
	if storage == nil || storage.readiness == nil {
		return fmt.Errorf("%w: b2 readiness controller is not initialized", ErrStorageUnavailable)
	}
	return storage.readiness.check(ctx)
}

// listReady issues the single readiness list request against the bucket.
func (storage B2Storage) listReady(ctx context.Context) error {
	_, err := storage.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(storage.Bucket),
		MaxKeys: aws.Int32(1),
		Prefix:  aws.String("v1/"),
	})
	if err != nil {
		return classifyB2ReadinessError(err)
	}
	return nil
}

// classifyB2ReadinessError maps every readiness request failure to the
// storage-unavailable sentinel. Unlike content reads, a missing bucket or
// missing-object response is an availability failure here, never a healthy
// or content-absent signal.
func classifyB2ReadinessError(err error) error {
	if err == nil {
		return nil
	}
	var responseError *awshttp.ResponseError
	if errors.As(err, &responseError) {
		var apiError smithy.APIError
		code := ""
		if errors.As(err, &apiError) {
			code = apiError.ErrorCode()
		}
		return fmt.Errorf("%w: b2 readiness: %s: status %d", ErrStorageUnavailable, code, responseError.HTTPStatusCode())
	}
	return fmt.Errorf("%w: b2 readiness: %v", ErrStorageUnavailable, err)
}

// readinessProbe bounds readiness traffic to the remote provider: at most
// one request in flight for the sharing instance, concurrent callers reuse
// the in-flight result, and results are cached briefly. Successful results
// cache for readinessSuccessTTL and failures for readinessFailureTTL so B2
// recovery is observed promptly. The controller holds no credentials,
// object names, or response bodies.
type readinessProbe struct {
	mu         sync.Mutex
	leader     *readinessCall
	cachedErr  error
	cachedAt   time.Time
	now        func() time.Time
	successTTL time.Duration
	failureTTL time.Duration
	request    func(context.Context) error
	// onWaiter, when set by tests, is invoked once a caller has joined the
	// in-flight call and is about to wait, giving deterministic
	// synchronization without sleeps.
	onWaiter func()
}

type readinessCall struct {
	done chan struct{}
	err  error
}

const (
	readinessSuccessTTL = 30 * time.Second
	readinessFailureTTL = 5 * time.Second
)

// newReadinessProbe builds a controller around one request function. A nil
// request reports unavailable so an unwired controller can never look
// healthy.
func newReadinessProbe(request func(context.Context) error) *readinessProbe {
	return &readinessProbe{
		now:        time.Now,
		successTTL: readinessSuccessTTL,
		failureTTL: readinessFailureTTL,
		request:    request,
	}
}

// check returns the current provider verdict, starting or joining at most
// one underlying request. Waiters honor their own context cancellation
// without disturbing the leader.
func (probe *readinessProbe) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: readiness canceled: %v", ErrStorageUnavailable, err)
	}
	probe.mu.Lock()
	if !probe.cachedAt.IsZero() {
		ttl := probe.failureTTL
		if probe.cachedErr == nil {
			ttl = probe.successTTL
		}
		if probe.now().Sub(probe.cachedAt) < ttl {
			cached := probe.cachedErr
			probe.mu.Unlock()
			return cached
		}
	}
	if call := probe.leader; call != nil {
		waiterHook := probe.onWaiter
		probe.mu.Unlock()
		if waiterHook != nil {
			waiterHook()
		}
		select {
		case <-call.done:
			return call.err
		case <-ctx.Done():
			return fmt.Errorf("%w: readiness waiter canceled: %v", ErrStorageUnavailable, ctx.Err())
		}
	}
	call := &readinessCall{done: make(chan struct{})}
	probe.leader = call
	probe.mu.Unlock()

	// The leader's own context bounds the underlying request; the caller of
	// CheckReady supplies the readiness deadline.
	err := error(nil)
	if probe.request == nil {
		err = fmt.Errorf("%w: readiness request is not wired", ErrStorageUnavailable)
	} else {
		err = probe.request(ctx)
	}

	probe.mu.Lock()
	call.err = err
	probe.leader = nil
	probe.cachedErr = err
	probe.cachedAt = probe.now()
	probe.mu.Unlock()
	close(call.done)
	return err
}

func (storage B2Storage) RemoveNew(ctx context.Context, storageKey string) error {
	if err := validateB2StorageKey(storageKey); err != nil {
		return err
	}
	_, err := storage.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(storage.Bucket),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		if errors.Is(classifyB2Error(err), ErrContentUnavailable) {
			return nil
		}
		return classifyB2Error(err)
	}
	return nil
}

// rangedReader is an io.ReadSeekCloser over a B2 object. Seeks close the
// current stream and re-open a ranged GET from the new offset on the next
// read, so http.ServeContent's seek-per-range behavior costs one ranged
// request per served range and one sequential request per full download.
type rangedReader struct {
	ctx    context.Context
	client *s3.Client
	bucket string
	key    string
	size   int64
	offset int64
	body   io.ReadCloser
}

func (reader *rangedReader) Read(buffer []byte) (int, error) {
	if reader.offset >= reader.size {
		return 0, io.EOF
	}
	if reader.body == nil {
		output, err := reader.client.GetObject(reader.ctx, &s3.GetObjectInput{
			Bucket: aws.String(reader.bucket),
			Key:    aws.String(reader.key),
			Range:  aws.String(fmt.Sprintf("bytes=%d-", reader.offset)),
		})
		if err != nil {
			return 0, classifyB2Error(err)
		}
		reader.body = output.Body
	}
	return reader.body.Read(buffer)
}

func (reader *rangedReader) Seek(offset int64, whence int) (int64, error) {
	var absolute int64
	switch whence {
	case io.SeekStart:
		absolute = offset
	case io.SeekCurrent:
		absolute = reader.offset + offset
	case io.SeekEnd:
		absolute = reader.size + offset
	default:
		return 0, fmt.Errorf("seek with invalid whence %d", whence)
	}
	if absolute < 0 {
		return 0, errors.New("seek before start of artifact content")
	}
	if absolute != reader.offset && reader.body != nil {
		_ = reader.body.Close()
		reader.body = nil
	}
	reader.offset = absolute
	return absolute, nil
}

func (reader *rangedReader) Close() error {
	if reader.body == nil {
		return nil
	}
	err := reader.body.Close()
	reader.body = nil
	reader.offset = reader.size
	return err
}

// classifyB2Error maps upstream S3 failures onto the artifact service's
// error surface: missing objects are an availability fact about one
// Artifact, while every other upstream failure is a storage outage.
func classifyB2Error(err error) error {
	if err == nil {
		return nil
	}
	var responseError *awshttp.ResponseError
	if errors.As(err, &responseError) {
		var apiError smithy.APIError
		code := ""
		if errors.As(err, &apiError) {
			code = apiError.ErrorCode()
		}
		if code == "NoSuchKey" || code == "NotFound" {
			return ErrContentUnavailable
		}
		return fmt.Errorf("%w: %s: status %d", ErrStorageUnavailable, code, responseError.HTTPStatusCode())
	}
	return fmt.Errorf("%w: %v", ErrStorageUnavailable, err)
}

func validateB2StorageKey(storageKey string) error {
	if storageKey == "" || strings.HasPrefix(storageKey, "/") {
		return ErrUnsafeStoragePath
	}
	for _, segment := range strings.Split(storageKey, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return ErrUnsafeStoragePath
		}
	}
	return nil
}
