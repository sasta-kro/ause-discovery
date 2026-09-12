package projectlogos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"time"

	"ause-discovery.local/backend/generated"
	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/platform/database"
	"ause-discovery.local/backend/internal/platform/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound           = errors.New("project logo not found")
	ErrRevisionConflict   = errors.New("project logo revision conflict")
	ErrInvalidState       = errors.New("project logo operation is not permitted in the current state")
	ErrContentUnavailable = errors.New("project logo content is unavailable")
	ErrStorageUnavailable = errors.New("project logo storage is unavailable")
	ErrEmptyContent       = errors.New("project logo content is empty")
	ErrContentTooLarge    = errors.New("project logo content exceeds the configured limit")
	ErrSizeMismatch       = errors.New("project logo content size does not match the declared size")
	ErrInvalidContentType = errors.New("project logo content is not a valid PNG image")
	ErrInvalidDimensions  = errors.New("project logo dimensions are invalid or exceed the configured limit")
	ErrCorruptContent     = errors.New("project logo content is corrupt or truncated")
)

const (
	MIMEType           = "image/png"
	Extension          = "png"
	MaxBytes     int64 = 2 * 1024 * 1024
	MaxDimension       = 1600
)

// Service owns Project Logo metadata. A Project Logo is Project-owned
// presentation metadata stored through the shared storage set: it is not an
// Artifact, never enters Artifact quotas, facets, or Project File lists,
// and at most one row per Project is active.
type Service struct {
	Pool    *pgxpool.Pool
	Storage artifacts.StorageSet
}

type Logo struct {
	ID             uuid.UUID
	ProjectID      uuid.UUID
	StorageKey     string
	StorageBackend string
	MIMEType       string
	Extension      string
	ByteCount      int64
	SHA256         [sha256.Size]byte
	Status         string
	Revision       int64
	ActorID        *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type UploadInput struct {
	ExpectedSize int64
	Content      io.Reader
}

type ValidatedContent struct {
	Content   []byte
	ByteCount int64
	SHA256    [sha256.Size]byte
	Width     int
	Height    int
}

// Validate reads and validates one complete logo image in memory. The 2 MiB
// limit makes whole-image buffering the simplest correct validation.
func Validate(reader io.Reader, expectedSize int64) (ValidatedContent, error) {
	content, err := io.ReadAll(io.LimitReader(reader, MaxBytes+1))
	if err != nil {
		return ValidatedContent{}, fmt.Errorf("read project logo content: %w", err)
	}
	if len(content) == 0 {
		return ValidatedContent{}, ErrEmptyContent
	}
	if int64(len(content)) > MaxBytes {
		return ValidatedContent{}, ErrContentTooLarge
	}
	if expectedSize >= 0 && int64(len(content)) != expectedSize {
		return ValidatedContent{}, ErrSizeMismatch
	}
	if http.DetectContentType(content) != MIMEType {
		return ValidatedContent{}, ErrInvalidContentType
	}
	// The configuration decode runs first so dimensions are bounded before
	// the full decode allocates image memory.
	configuration, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil || configuration.Width <= 0 || configuration.Height <= 0 {
		return ValidatedContent{}, ErrInvalidDimensions
	}
	if configuration.Width > MaxDimension || configuration.Height > MaxDimension {
		return ValidatedContent{}, ErrInvalidDimensions
	}
	// A decodable header proves nothing about the image data: fully decode
	// the buffered PNG so truncated IDAT streams, corrupt checksums, missing
	// image data, and missing terminal chunks are rejected before storage.
	if _, err := png.Decode(bytes.NewReader(content)); err != nil {
		return ValidatedContent{}, ErrCorruptContent
	}
	return ValidatedContent{
		Content:   content,
		ByteCount: int64(len(content)),
		SHA256:    sha256.Sum256(content),
		Width:     configuration.Width,
		Height:    configuration.Height,
	}, nil
}

// Upload stores validated PNG bytes and makes them the Project's single
// active logo, soft-deleting any previous active row. The image is written
// before the transaction; a failed transaction removes only the new object.
func (service Service) Upload(ctx context.Context, actorID, projectID uuid.UUID, expectedProjectRevision int64, input UploadInput) (Logo, error) {
	validated, err := Validate(input.Content, input.ExpectedSize)
	if err != nil {
		return Logo{}, err
	}
	stored, err := service.Storage.Default().Put(ctx, bytes.NewReader(validated.Content), validated.ByteCount)
	if err != nil {
		if errors.Is(err, artifacts.ErrStorageUnavailable) {
			return Logo{}, ErrStorageUnavailable
		}
		return Logo{}, err
	}
	if stored.DetectedMIME != MIMEType {
		_ = service.Storage.Default().RemoveNew(ctx, stored.StorageKey)
		return Logo{}, ErrInvalidContentType
	}

	logoID := uuid.Must(uuid.NewV7())
	var result Logo
	err = database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		projectRevision, projectStatus, err := lockProject(ctx, transaction, projectID)
		if err != nil {
			return err
		}
		if projectRevision != expectedProjectRevision {
			return ErrRevisionConflict
		}
		if projectStatus == "deleted" {
			return ErrInvalidState
		}
		previous, previousErr := lockActiveLogo(ctx, transaction, projectID)
		if previousErr != nil {
			return previousErr
		}
		eventType := "project_logo.uploaded"
		metadata := map[string]any{"project_revision": projectRevision}
		if previous != nil {
			if _, err := transaction.Exec(ctx, `UPDATE project_logos SET status='deleted', revision=revision+1, actor_id=$2, updated_at=now(), deleted_at=now() WHERE id=$1`, previous.ID, actorID); err != nil {
				return err
			}
			eventType = "project_logo.replaced"
			metadata["replaced_logo_id"] = previous.ID.String()
		}
		// The revision is derived from every historical row for the Project,
		// not only the active row, so upload, replacement, removal, and
		// re-upload can never reuse an earlier public logo version and an
		// immutable URL can never serve different bytes. The Project row lock
		// serializes this read against every other logo mutation.
		var highestRevision int64
		if err := transaction.QueryRow(ctx, `SELECT coalesce(max(revision), 0) FROM project_logos WHERE project_id=$1`, projectID).Scan(&highestRevision); err != nil {
			return err
		}
		logoRevision := highestRevision + 1
		row := transaction.QueryRow(ctx, `
			INSERT INTO project_logos (id, project_id, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, revision, actor_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active', $9, $10)
			RETURNING id, project_id, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, revision, actor_id, created_at, updated_at, deleted_at`,
			logoID, projectID, stored.StorageKey, service.Storage.DefaultName, MIMEType, Extension, stored.ByteCount, stored.SHA256[:], logoRevision, actorID)
		result, err = scanLogo(row)
		if err != nil {
			return err
		}
		updatedRevision, err := advanceProjectAfterLogoMutation(ctx, transaction, projectID, projectStatus)
		if err != nil {
			return err
		}
		metadata["project_revision"] = updatedRevision
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: eventType, TargetType: "project_logo", TargetID: logoID, Metadata: metadata})
	})
	if err != nil {
		_ = service.Storage.Default().RemoveNew(ctx, stored.StorageKey)
		return Logo{}, err
	}
	return result, nil
}

// Remove soft-deletes the active logo. Bytes and rows are retained.
func (service Service) Remove(ctx context.Context, actorID, projectID uuid.UUID, expectedProjectRevision int64) error {
	return database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		projectRevision, projectStatus, err := lockProject(ctx, transaction, projectID)
		if err != nil {
			return err
		}
		if projectRevision != expectedProjectRevision {
			return ErrRevisionConflict
		}
		if projectStatus == "deleted" {
			return ErrInvalidState
		}
		active, activeErr := lockActiveLogo(ctx, transaction, projectID)
		if activeErr != nil {
			return activeErr
		}
		if _, err := transaction.Exec(ctx, `UPDATE project_logos SET status='deleted', revision=revision+1, actor_id=$2, updated_at=now(), deleted_at=now() WHERE id=$1`, active.ID, actorID); err != nil {
			return err
		}
		updatedRevision, err := advanceProjectAfterLogoMutation(ctx, transaction, projectID, projectStatus)
		if err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "project_logo.removed", TargetType: "project_logo", TargetID: active.ID, Metadata: map[string]any{"project_revision": updatedRevision}})
	})
}

// ActiveRevision reports the active logo revision for response building.
func (service Service) ActiveRevision(ctx context.Context, projectID uuid.UUID) (int64, bool, error) {
	var revision int64
	err := service.Pool.QueryRow(ctx, `SELECT revision FROM project_logos WHERE project_id=$1 AND status='active'`, projectID).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return revision, true, nil
}

// ActiveDigest reports the active logo digest for idempotent import skips.
func (service Service) ActiveDigest(ctx context.Context, projectID uuid.UUID) ([sha256.Size]byte, bool, error) {
	var digest []byte
	err := service.Pool.QueryRow(ctx, `SELECT sha256 FROM project_logos WHERE project_id=$1 AND status='active'`, projectID).Scan(&digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return [sha256.Size]byte{}, false, nil
	}
	if err != nil {
		return [sha256.Size]byte{}, false, err
	}
	var result [sha256.Size]byte
	copy(result[:], digest)
	return result, true, nil
}

type PublicContent struct {
	Logo Logo
	File io.ReadSeekCloser
	Size int64
}

// OpenActive returns the published Project's active logo content from the
// row-selected storage provider. The expected revision is part of the
// lookup: a missing or stale version resolves to ErrNotFound before any
// storage provider is opened, so stale public URLs can never trigger
// provider reads or serve replacement bytes.
func (service Service) OpenActive(ctx context.Context, projectID uuid.UUID, expectedRevision int64) (PublicContent, error) {
	logo, err := scanLogo(service.Pool.QueryRow(ctx, `
		SELECT project_logos.id, project_logos.project_id, project_logos.storage_key, project_logos.storage_backend, project_logos.mime_type, project_logos.extension, project_logos.byte_count, project_logos.sha256, project_logos.status, project_logos.revision, project_logos.actor_id, project_logos.created_at, project_logos.updated_at, project_logos.deleted_at
		FROM project_logos JOIN projects ON projects.id=project_logos.project_id
		WHERE project_logos.project_id=$1 AND project_logos.status='active' AND projects.status='published'`, projectID))
	if err != nil {
		return PublicContent{}, err
	}
	if logo.Revision != expectedRevision {
		return PublicContent{}, ErrNotFound
	}
	return openLogoContent(ctx, service.Storage, logo)
}

// OpenActiveForAdministration returns the current active logo content for
// administrator preview. Unlike the public path it accepts draft Projects,
// because the administrator already owns the record. Deleted Projects are
// excluded.
func (service Service) OpenActiveForAdministration(ctx context.Context, projectID uuid.UUID) (PublicContent, error) {
	logo, err := scanLogo(service.Pool.QueryRow(ctx, `
		SELECT project_logos.id, project_logos.project_id, project_logos.storage_key, project_logos.storage_backend, project_logos.mime_type, project_logos.extension, project_logos.byte_count, project_logos.sha256, project_logos.status, project_logos.revision, project_logos.actor_id, project_logos.created_at, project_logos.updated_at, project_logos.deleted_at
		FROM project_logos JOIN projects ON projects.id=project_logos.project_id
		WHERE project_logos.project_id=$1 AND project_logos.status='active' AND projects.status <> 'deleted'`, projectID))
	if err != nil {
		return PublicContent{}, err
	}
	return openLogoContent(ctx, service.Storage, logo)
}

func openLogoContent(ctx context.Context, set artifacts.StorageSet, logo Logo) (PublicContent, error) {
	backend, lookupErr := set.Lookup(logo.StorageBackend)
	if lookupErr != nil {
		return PublicContent{}, ErrContentUnavailable
	}
	file, size, err := backend.Open(ctx, logo.StorageKey)
	if err != nil {
		if errors.Is(err, artifacts.ErrStorageUnavailable) {
			return PublicContent{}, ErrStorageUnavailable
		}
		return PublicContent{}, ErrContentUnavailable
	}
	return PublicContent{Logo: logo, File: file, Size: size}, nil
}

func lockProject(ctx context.Context, transaction pgx.Tx, projectID uuid.UUID) (int64, string, error) {
	var revision int64
	var status string
	err := transaction.QueryRow(ctx, "SELECT revision, status FROM projects WHERE id=$1 FOR UPDATE", projectID).Scan(&revision, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", ErrNotFound
	}
	return revision, status, err
}

func lockActiveLogo(ctx context.Context, transaction pgx.Tx, projectID uuid.UUID) (*Logo, error) {
	logo, err := scanLogo(transaction.QueryRow(ctx, `SELECT id, project_id, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, revision, actor_id, created_at, updated_at, deleted_at FROM project_logos WHERE project_id=$1 AND status='active' FOR UPDATE`, projectID))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &logo, nil
}

func advanceProjectAfterLogoMutation(ctx context.Context, transaction pgx.Tx, projectID uuid.UUID, projectStatus string) (int64, error) {
	var revision int64
	if err := transaction.QueryRow(ctx, "UPDATE projects SET revision=revision+1, updated_at=now() WHERE id=$1 RETURNING revision", projectID).Scan(&revision); err != nil {
		return 0, err
	}
	desiredAction := "remove"
	if projectStatus == "published" {
		desiredAction = "upsert"
	}
	_, err := generated.New(transaction).UpsertProjectSearchSync(ctx, generated.UpsertProjectSearchSyncParams{ProjectID: identity.UUID(projectID), DesiredRevision: revision, DesiredAction: desiredAction})
	return revision, err
}

type rowScanner interface {
	Scan(destinations ...any) error
}

func scanLogo(row rowScanner) (Logo, error) {
	var result Logo
	var digest []byte
	var actorID *uuid.UUID
	err := row.Scan(&result.ID, &result.ProjectID, &result.StorageKey, &result.StorageBackend, &result.MIMEType, &result.Extension, &result.ByteCount, &digest, &result.Status, &result.Revision, &actorID, &result.CreatedAt, &result.UpdatedAt, &result.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Logo{}, ErrNotFound
	}
	if err != nil {
		return Logo{}, err
	}
	if len(digest) != len(result.SHA256) {
		return Logo{}, errors.New("stored Project Logo digest has an invalid length")
	}
	copy(result.SHA256[:], digest)
	result.ActorID = actorID
	return result, nil
}
