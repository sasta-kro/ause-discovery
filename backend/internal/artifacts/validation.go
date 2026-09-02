package artifacts

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrArtifactTypeMismatch = errors.New("artifact type does not permit the file extension")
	ErrContentTypeMismatch  = errors.New("artifact content does not match the file extension")
	ErrUnsafeContentType    = errors.New("artifact content type is unsafe")
)

type uploadMetadata struct {
	ArtifactType     string
	DisplayName      string
	OriginalFilename string
	Extension        string
}

var allowedExtensions = map[string]map[string]bool{
	"report":      {"pdf": true},
	"slides":      {"pdf": true, "ppt": true, "pptx": true},
	"source_code": {"zip": true, "tar.gz": true, "tgz": true},
	"proposal":    {"pdf": true, "docx": true},
	"poster":      {"pdf": true, "png": true, "jpg": true, "jpeg": true},
	"dataset":     {"zip": true, "csv": true, "tsv": true, "json": true, "xlsx": true},
	"demo_video":  {"mp4": true, "webm": true},
	"other":       {"pdf": true, "docx": true, "pptx": true, "zip": true, "csv": true, "xlsx": true, "json": true},
}

func validateUploadMetadata(artifactType, displayName, originalFilename string) (uploadMetadata, error) {
	artifactType = strings.TrimSpace(artifactType)
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || utf8.RuneCountInString(displayName) > 300 {
		return uploadMetadata{}, errors.New("artifact display name must contain between 1 and 300 characters")
	}

	sanitizedFilename := sanitizeFilename(originalFilename)
	if sanitizedFilename == "" || utf8.RuneCountInString(sanitizedFilename) > 512 {
		return uploadMetadata{}, errors.New("artifact filename must contain between 1 and 512 characters")
	}
	extension := normalizedExtension(sanitizedFilename)
	allowed, knownType := allowedExtensions[artifactType]
	if !knownType || !allowed[extension] {
		return uploadMetadata{}, ErrArtifactTypeMismatch
	}
	return uploadMetadata{ArtifactType: artifactType, DisplayName: displayName, OriginalFilename: sanitizedFilename, Extension: extension}, nil
}

func validateDetectedContent(extension, detectedMIME string, prefix []byte) error {
	normalizedMIME := strings.ToLower(strings.TrimSpace(strings.Split(detectedMIME, ";")[0]))
	trimmedPrefix := bytes.TrimSpace(prefix)
	lowerPrefix := bytes.ToLower(trimmedPrefix)
	if bytes.HasPrefix(prefix, []byte{0x7f, 'E', 'L', 'F'}) || bytes.HasPrefix(prefix, []byte{'M', 'Z'}) || bytes.HasPrefix(lowerPrefix, []byte("<html")) || bytes.HasPrefix(lowerPrefix, []byte("<!doctype html")) || bytes.HasPrefix(lowerPrefix, []byte("<svg")) {
		return ErrUnsafeContentType
	}
	switch normalizedMIME {
	case "text/html", "image/svg+xml", "application/javascript", "text/javascript", "application/x-httpd-php", "application/x-msdownload", "application/x-executable", "application/x-sharedlib":
		return ErrUnsafeContentType
	}

	matches := false
	switch extension {
	case "pdf":
		matches = normalizedMIME == "application/pdf" && bytes.HasPrefix(prefix, []byte("%PDF-"))
	case "png":
		matches = normalizedMIME == "image/png" && bytes.HasPrefix(prefix, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	case "jpg", "jpeg":
		matches = normalizedMIME == "image/jpeg" && bytes.HasPrefix(prefix, []byte{0xff, 0xd8, 0xff})
	case "zip", "docx", "pptx", "xlsx":
		matches = normalizedMIME == "application/zip" && hasZIPSignature(prefix)
	case "tar.gz", "tgz":
		matches = (normalizedMIME == "application/gzip" || normalizedMIME == "application/x-gzip") && bytes.HasPrefix(prefix, []byte{0x1f, 0x8b})
	case "ppt":
		matches = normalizedMIME == "application/octet-stream" && bytes.HasPrefix(prefix, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	case "csv", "tsv", "json":
		matches = (normalizedMIME == "text/plain" || normalizedMIME == "application/json" || normalizedMIME == "application/octet-stream") && !bytes.Contains(prefix, []byte{0})
	case "mp4":
		matches = (normalizedMIME == "video/mp4" || normalizedMIME == "application/octet-stream") && len(prefix) >= 12 && string(prefix[4:8]) == "ftyp"
	case "webm":
		matches = (normalizedMIME == "video/webm" || normalizedMIME == "application/octet-stream") && bytes.HasPrefix(prefix, []byte{0x1a, 0x45, 0xdf, 0xa3})
	}
	if !matches {
		return fmt.Errorf("%w: extension %s detected as %s", ErrContentTypeMismatch, extension, normalizedMIME)
	}
	return nil
}

func sanitizeFilename(filename string) string {
	filename = strings.ReplaceAll(filename, "\\", "/")
	filename = filepath.Base(filename)
	filename = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return -1
		}
		return character
	}, filename)
	return strings.TrimSpace(filename)
}

func normalizedExtension(filename string) string {
	lowerFilename := strings.ToLower(filename)
	if strings.HasSuffix(lowerFilename, ".tar.gz") {
		return "tar.gz"
	}
	return strings.TrimPrefix(filepath.Ext(lowerFilename), ".")
}

func hasZIPSignature(prefix []byte) bool {
	return bytes.HasPrefix(prefix, []byte{'P', 'K', 0x03, 0x04}) || bytes.HasPrefix(prefix, []byte{'P', 'K', 0x05, 0x06}) || bytes.HasPrefix(prefix, []byte{'P', 'K', 0x07, 0x08})
}
