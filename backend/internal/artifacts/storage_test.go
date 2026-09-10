package artifacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoragePutOpenAndExists(t *testing.T) {
	root := t.TempDir()
	storage := LocalStorage{Root: root, MaxBytes: 1024}
	content := []byte("artifact content")

	stored, err := storage.Put(context.Background(), bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Put returned an error: %v", err)
	}
	if !strings.HasPrefix(stored.StorageKey, "v1/") || filepath.IsAbs(stored.StorageKey) {
		t.Fatalf("Put returned unsafe storage key %q", stored.StorageKey)
	}
	if stored.ByteCount != int64(len(content)) {
		t.Fatalf("Put stored %d bytes, expected %d", stored.ByteCount, int64(len(content)))
	}
	expectedDigest := sha256.Sum256(content)
	if stored.SHA256 != expectedDigest {
		t.Fatalf("Put returned digest %x, expected %x", stored.SHA256, expectedDigest)
	}
	if !storage.Exists(context.Background(), stored.StorageKey) {
		t.Fatal("Exists returned false for stored content")
	}

	file, size, err := storage.Open(context.Background(), stored.StorageKey)
	if err != nil {
		t.Fatalf("Open returned an error: %v", err)
	}
	defer file.Close()
	opened, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read opened content: %v", err)
	}
	if size != int64(len(content)) || !bytes.Equal(opened, content) {
		t.Fatalf("Open returned size %d and content %q", size, opened)
	}
}

func TestStoragePutRejectsInvalidSizes(t *testing.T) {
	storage := LocalStorage{Root: t.TempDir(), MaxBytes: 4}

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

func TestStorageRejectsTraversalAndSymlinks(t *testing.T) {
	root := t.TempDir()
	storage := LocalStorage{Root: root, MaxBytes: 1024}

	if _, _, err := storage.Open(context.Background(), "../outside"); !errors.Is(err, ErrUnsafeStoragePath) {
		t.Fatalf("traversal Open returned %v, expected unsafe path error", err)
	}

	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("content"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if _, _, err := storage.Open(context.Background(), "linked"); !errors.Is(err, ErrUnsafeStoragePath) {
		t.Fatalf("symlink Open returned %v, expected unsafe path error", err)
	}
}

func TestNewStorageSetRegistersAndValidatesBackends(t *testing.T) {
	local := LocalStorage{Root: t.TempDir(), MaxBytes: 128}
	localSet, err := NewStorageSet(StorageOptions{DefaultName: BackendLocal, LocalRoot: local.Root, MaxArtifactBytes: local.MaxBytes})
	if err != nil {
		t.Fatalf("local NewStorageSet returned an error: %v", err)
	}
	if localSet.Default().Name() != BackendLocal {
		t.Fatalf("default backend was %q, expected local", localSet.Default().Name())
	}
	if _, err := localSet.Lookup(BackendB2); !errors.Is(err, ErrUnknownStorageName) {
		t.Fatalf("Lookup of absent b2 returned %v, expected unknown name error", err)
	}
	if _, err := NewStorageSet(StorageOptions{DefaultName: BackendLocal, LocalRoot: local.Root}); err != nil {
		t.Fatalf("local NewStorageSet without b2 values returned an error: %v", err)
	}

	if _, err := NewStorageSet(StorageOptions{DefaultName: "s3", LocalRoot: local.Root}); err == nil {
		t.Fatal("unknown default backend was accepted")
	}
	if _, err := NewStorageSet(StorageOptions{DefaultName: BackendB2, LocalRoot: local.Root, B2Endpoint: "s3.us-west-004.backblazeb2.com"}); err == nil {
		t.Fatal("incomplete b2 configuration was accepted")
	}
}
