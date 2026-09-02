package artifacts

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrEmptyContent      = errors.New("artifact content is empty")
	ErrContentTooLarge   = errors.New("artifact content exceeds the configured limit")
	ErrSizeMismatch      = errors.New("artifact content size does not match the declared size")
	ErrUnsafeStoragePath = errors.New("artifact storage path is unsafe")
)

type Storage struct {
	Root     string
	MaxBytes int64
}

type StoredContent struct {
	StorageKey   string
	ByteCount    int64
	SHA256       [sha256.Size]byte
	DetectedMIME string
	Prefix       []byte
}

func (storage Storage) Put(ctx context.Context, reader io.Reader, expectedSize int64) (StoredContent, error) {
	if expectedSize == 0 {
		return StoredContent{}, ErrEmptyContent
	}
	if storage.MaxBytes <= 0 {
		return StoredContent{}, errors.New("artifact storage limit must be positive")
	}
	if expectedSize > storage.MaxBytes {
		return StoredContent{}, ErrContentTooLarge
	}
	if err := storage.ensureRoot(); err != nil {
		return StoredContent{}, err
	}

	contentID := uuid.NewString()
	storageKey := filepath.ToSlash(filepath.Join("v1", contentID[0:2], contentID[2:4], contentID))
	finalPath, err := storage.resolve(storageKey, false)
	if err != nil {
		return StoredContent{}, err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o750); err != nil {
		return StoredContent{}, fmt.Errorf("create artifact directory: %w", err)
	}
	if err := storage.rejectSymlinks(filepath.Dir(finalPath)); err != nil {
		return StoredContent{}, err
	}

	temporaryDirectory := filepath.Join(storage.Root, ".tmp")
	temporaryFile, err := os.CreateTemp(temporaryDirectory, contentID+"-*.part")
	if err != nil {
		return StoredContent{}, fmt.Errorf("create artifact temporary file: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	removeTemporary := true
	defer func() {
		_ = temporaryFile.Close()
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
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
	if err := temporaryFile.Sync(); err != nil {
		return StoredContent{}, fmt.Errorf("sync artifact content: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return StoredContent{}, fmt.Errorf("close artifact content: %w", err)
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return StoredContent{}, fmt.Errorf("finalize artifact content: %w", err)
	}
	removeTemporary = false

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

func (storage Storage) Open(storageKey string) (*os.File, int64, error) {
	path, err := storage.resolve(storageKey, true)
	if err != nil {
		return nil, 0, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	information, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	if !information.Mode().IsRegular() {
		file.Close()
		return nil, 0, ErrUnsafeStoragePath
	}
	return file, information.Size(), nil
}

func (storage Storage) Exists(storageKey string) bool {
	file, _, err := storage.Open(storageKey)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

func (storage Storage) removeNew(storageKey string) error {
	path, err := storage.resolve(storageKey, true)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (storage Storage) ensureRoot() error {
	if !filepath.IsAbs(storage.Root) {
		return ErrUnsafeStoragePath
	}
	if err := os.MkdirAll(storage.Root, 0o750); err != nil {
		return fmt.Errorf("create artifact root: %w", err)
	}
	if err := storage.rejectSymlinks(storage.Root); err != nil {
		return err
	}
	temporaryDirectory := filepath.Join(storage.Root, ".tmp")
	if err := os.MkdirAll(temporaryDirectory, 0o750); err != nil {
		return fmt.Errorf("create artifact temporary directory: %w", err)
	}
	return storage.rejectSymlinks(temporaryDirectory)
}

func (storage Storage) resolve(storageKey string, requireExisting bool) (string, error) {
	if storageKey == "" || filepath.IsAbs(storageKey) || filepath.Clean(storageKey) != storageKey || strings.HasPrefix(storageKey, ".."+string(filepath.Separator)) {
		return "", ErrUnsafeStoragePath
	}
	root, err := filepath.Abs(storage.Root)
	if err != nil {
		return "", ErrUnsafeStoragePath
	}
	path := filepath.Join(root, storageKey)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrUnsafeStoragePath
	}
	checkPath := path
	if !requireExisting {
		checkPath = filepath.Dir(path)
	}
	if err := storage.rejectSymlinks(checkPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}

func (storage Storage) rejectSymlinks(path string) error {
	root, err := filepath.Abs(storage.Root)
	if err != nil {
		return ErrUnsafeStoragePath
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ErrUnsafeStoragePath
	}
	current := root
	paths := []string{root}
	if relative != "." {
		for _, component := range strings.Split(relative, string(filepath.Separator)) {
			current = filepath.Join(current, component)
			paths = append(paths, current)
		}
	}
	for _, candidate := range paths {
		information, statErr := os.Lstat(candidate)
		if statErr != nil {
			return statErr
		}
		if information.Mode()&os.ModeSymlink != 0 {
			return ErrUnsafeStoragePath
		}
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-reader.ctx.Done():
		return 0, reader.ctx.Err()
	default:
		return reader.reader.Read(buffer)
	}
}

type prefixWriter struct {
	content []byte
	limit   int
}

func (writer *prefixWriter) Write(content []byte) (int, error) {
	remaining := writer.limit - len(writer.content)
	if remaining > 0 {
		if len(content) < remaining {
			remaining = len(content)
		}
		writer.content = append(writer.content, content[:remaining]...)
	}
	return len(content), nil
}
