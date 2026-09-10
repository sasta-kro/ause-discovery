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

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
)

// B2Storage stores Artifact bytes in a private Backblaze B2 bucket through
// its S3-compatible API (AD-011). Visitors never contact the bucket: the
// API proxies every read.
type B2Storage struct {
	Bucket   string
	Client   *s3.Client
	MaxBytes int64
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
	return &B2Storage{Bucket: bucket, Client: client, MaxBytes: maxBytes}, nil
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
