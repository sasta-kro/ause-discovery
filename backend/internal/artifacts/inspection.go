package artifacts

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
)

type UploadInspection struct {
	ByteCount int64
	SHA256    [sha256.Size]byte
}

func InspectUpload(ctx context.Context, input UploadInput, maximumBytes int64) (UploadInspection, error) {
	metadata, err := validateUploadMetadata(input.ArtifactType, input.DisplayName, input.OriginalFilename)
	if err != nil {
		return UploadInspection{}, err
	}
	if input.ExpectedSize == 0 {
		return UploadInspection{}, ErrEmptyContent
	}
	if maximumBytes <= 0 {
		return UploadInspection{}, errors.New("artifact storage limit must be positive")
	}
	if input.ExpectedSize > maximumBytes {
		return UploadInspection{}, ErrContentTooLarge
	}
	hasher := sha256.New()
	prefix := &prefixWriter{limit: 512}
	byteCount, err := io.Copy(io.MultiWriter(hasher, prefix), io.LimitReader(&contextReader{ctx: ctx, reader: input.Content}, maximumBytes+1))
	if err != nil {
		return UploadInspection{}, err
	}
	if byteCount == 0 {
		return UploadInspection{}, ErrEmptyContent
	}
	if byteCount > maximumBytes {
		return UploadInspection{}, ErrContentTooLarge
	}
	if input.ExpectedSize >= 0 && byteCount != input.ExpectedSize {
		return UploadInspection{}, ErrSizeMismatch
	}
	if err := validateDetectedContent(metadata.Extension, http.DetectContentType(prefix.content), prefix.content); err != nil {
		return UploadInspection{}, err
	}
	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return UploadInspection{ByteCount: byteCount, SHA256: digest}, nil
}
