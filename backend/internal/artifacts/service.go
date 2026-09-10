package artifacts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"ause-discovery.local/backend/generated"
	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/platform/database"
	"ause-discovery.local/backend/internal/platform/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound             = errors.New("artifact not found")
	ErrRevisionConflict     = errors.New("artifact revision conflict")
	ErrProjectQuotaExceeded = errors.New("Project Artifact quota exceeded")
	ErrInvalidState         = errors.New("artifact operation is not permitted in the current state")
	ErrContentUnavailable   = errors.New("artifact content is unavailable")
)

type Service struct {
	Pool            *pgxpool.Pool
	Storage         StorageSet
	MaxProjectBytes int64
}

type UploadInput struct {
	ArtifactType     string
	DisplayName      string
	OriginalFilename string
	ExpectedSize     int64
	Content          io.Reader
}

type ReplaceInput struct {
	OriginalFilename string
	ExpectedSize     int64
	Content          io.Reader
}

type Artifact struct {
	ID               uuid.UUID
	ProjectID        uuid.UUID
	ArtifactType     string
	DisplayName      string
	OriginalFilename string
	StorageKey       string
	StorageBackend   string
	MIMEType         string
	Extension        string
	ByteCount        int64
	SHA256           [32]byte
	Status           string
	Revision         int64
	ActorID          *uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

type PublicContent struct {
	Artifact Artifact
	File     io.ReadSeekCloser
	Size     int64
}

func (service Service) Upload(ctx context.Context, actorID, projectID uuid.UUID, expectedProjectRevision int64, input UploadInput) (Artifact, error) {
	metadata, err := validateUploadMetadata(input.ArtifactType, input.DisplayName, input.OriginalFilename)
	if err != nil {
		return Artifact{}, err
	}
	stored, err := service.Storage.Default().Put(ctx, input.Content, input.ExpectedSize)
	if err != nil {
		return Artifact{}, err
	}
	if err := validateDetectedContent(metadata.Extension, stored.DetectedMIME, stored.Prefix); err != nil {
		_ = service.Storage.Default().RemoveNew(ctx, stored.StorageKey)
		return Artifact{}, err
	}

	artifactID := uuid.Must(uuid.NewV7())
	var result Artifact
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
		if err := service.checkQuota(ctx, transaction, projectID, stored.ByteCount); err != nil {
			return err
		}
		row := transaction.QueryRow(ctx, `
			INSERT INTO artifacts (id, project_id, type, display_name, original_filename, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, actor_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'active', $12)
			RETURNING id, project_id, type, display_name, original_filename, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, revision, actor_id, created_at, updated_at, deleted_at`,
			artifactID, projectID, metadata.ArtifactType, metadata.DisplayName, metadata.OriginalFilename, stored.StorageKey, service.Storage.DefaultName, stored.DetectedMIME, metadata.Extension, stored.ByteCount, stored.SHA256[:], actorID)
		result, err = scanArtifact(row)
		if err != nil {
			return err
		}
		updatedRevision, err := updateProjectAfterArtifactMutation(ctx, transaction, projectID, projectStatus)
		if err != nil {
			return err
		}
		if err := audit.AppendTx(ctx, transaction, artifactAuditEvent(actorID, "artifact.uploaded", result, map[string]any{"project_revision": updatedRevision})); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = service.Storage.Default().RemoveNew(ctx, stored.StorageKey)
		return Artifact{}, err
	}
	return result, nil
}

func (service Service) Update(ctx context.Context, actorID, artifactID uuid.UUID, expectedRevision int64, artifactType, displayName string) (Artifact, error) {
	var result Artifact
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		projectID, err := artifactProjectID(ctx, transaction, artifactID)
		if err != nil {
			return err
		}
		_, projectStatus, err := lockProject(ctx, transaction, projectID)
		if err != nil {
			return err
		}
		if projectStatus == "deleted" {
			return ErrInvalidState
		}
		current, err := lockArtifact(ctx, transaction, artifactID)
		if err != nil {
			return err
		}
		if current.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		metadata, err := validateUploadMetadata(artifactType, displayName, current.OriginalFilename)
		if err != nil {
			return err
		}
		result, err = scanArtifact(transaction.QueryRow(ctx, `UPDATE artifacts SET type=$2, display_name=$3, revision=revision+1, actor_id=$4, updated_at=now() WHERE id=$1 RETURNING id, project_id, type, display_name, original_filename, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, revision, actor_id, created_at, updated_at, deleted_at`, artifactID, metadata.ArtifactType, metadata.DisplayName, actorID))
		if err != nil {
			return err
		}
		projectRevision, err := updateProjectAfterArtifactMutation(ctx, transaction, projectID, projectStatus)
		if err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, artifactAuditEvent(actorID, "artifact.updated", result, map[string]any{"project_revision": projectRevision}))
	})
	return result, err
}

func (service Service) Delete(ctx context.Context, actorID, artifactID uuid.UUID, expectedRevision int64) (Artifact, error) {
	return service.transition(ctx, actorID, artifactID, expectedRevision, "delete")
}

func (service Service) Restore(ctx context.Context, actorID, artifactID uuid.UUID, expectedRevision int64) (Artifact, error) {
	return service.transition(ctx, actorID, artifactID, expectedRevision, "restore")
}

func (service Service) transition(ctx context.Context, actorID, artifactID uuid.UUID, expectedRevision int64, operation string) (Artifact, error) {
	var result Artifact
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		projectID, err := artifactProjectID(ctx, transaction, artifactID)
		if err != nil {
			return err
		}
		_, projectStatus, err := lockProject(ctx, transaction, projectID)
		if err != nil {
			return err
		}
		if projectStatus == "deleted" {
			return ErrInvalidState
		}
		current, err := lockArtifact(ctx, transaction, artifactID)
		if err != nil {
			return err
		}
		if current.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		if operation == "delete" && current.Status != "active" || operation == "restore" && current.Status != "deleted" {
			return ErrInvalidState
		}
		if operation == "restore" {
			if _, err := validateUploadMetadata(current.ArtifactType, current.DisplayName, current.OriginalFilename); err != nil {
				return err
			}
			backend, lookupErr := service.Storage.Lookup(current.StorageBackend)
			if lookupErr != nil {
				return ErrContentUnavailable
			}
			if !backend.Exists(ctx, current.StorageKey) {
				return ErrContentUnavailable
			}
			if err := service.checkQuota(ctx, transaction, projectID, current.ByteCount); err != nil {
				return err
			}
		}

		status := "deleted"
		deletedAtExpression := "now()"
		eventType := "artifact.deleted"
		if operation == "restore" {
			status = "active"
			deletedAtExpression = "NULL"
			eventType = "artifact.restored"
		}
		statement := fmt.Sprintf(`UPDATE artifacts SET status=$2, revision=revision+1, actor_id=$3, updated_at=now(), deleted_at=%s WHERE id=$1 RETURNING id, project_id, type, display_name, original_filename, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, revision, actor_id, created_at, updated_at, deleted_at`, deletedAtExpression)
		result, err = scanArtifact(transaction.QueryRow(ctx, statement, artifactID, status, actorID))
		if err != nil {
			return err
		}
		projectRevision, err := updateProjectAfterArtifactMutation(ctx, transaction, projectID, projectStatus)
		if err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, artifactAuditEvent(actorID, eventType, result, map[string]any{"project_revision": projectRevision}))
	})
	return result, err
}

func (service Service) Replace(ctx context.Context, actorID, artifactID uuid.UUID, expectedRevision int64, input ReplaceInput) (Artifact, error) {
	current, err := service.Get(ctx, artifactID)
	if err != nil {
		return Artifact{}, err
	}
	metadata, err := validateUploadMetadata(current.ArtifactType, current.DisplayName, input.OriginalFilename)
	if err != nil {
		return Artifact{}, err
	}
	stored, err := service.Storage.Default().Put(ctx, input.Content, input.ExpectedSize)
	if err != nil {
		return Artifact{}, err
	}
	if err := validateDetectedContent(metadata.Extension, stored.DetectedMIME, stored.Prefix); err != nil {
		_ = service.Storage.Default().RemoveNew(ctx, stored.StorageKey)
		return Artifact{}, err
	}

	replacementID := uuid.Must(uuid.NewV7())
	var result Artifact
	err = database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		projectID, err := artifactProjectID(ctx, transaction, artifactID)
		if err != nil {
			return err
		}
		_, projectStatus, err := lockProject(ctx, transaction, projectID)
		if err != nil {
			return err
		}
		if projectStatus == "deleted" {
			return ErrInvalidState
		}
		locked, err := lockArtifact(ctx, transaction, artifactID)
		if err != nil {
			return err
		}
		if locked.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		if locked.Status != "active" {
			return ErrInvalidState
		}
		lockedMetadata, err := validateUploadMetadata(locked.ArtifactType, locked.DisplayName, input.OriginalFilename)
		if err != nil {
			return err
		}
		if err := service.checkReplacementQuota(ctx, transaction, projectID, locked.ByteCount, stored.ByteCount); err != nil {
			return err
		}
		if _, err := transaction.Exec(ctx, `UPDATE artifacts SET status='deleted', revision=revision+1, actor_id=$2, updated_at=now(), deleted_at=now() WHERE id=$1`, artifactID, actorID); err != nil {
			return err
		}
		result, err = scanArtifact(transaction.QueryRow(ctx, `
			INSERT INTO artifacts (id, project_id, type, display_name, original_filename, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, actor_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'active', $12)
			RETURNING id, project_id, type, display_name, original_filename, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, revision, actor_id, created_at, updated_at, deleted_at`,
			replacementID, projectID, locked.ArtifactType, locked.DisplayName, lockedMetadata.OriginalFilename, stored.StorageKey, service.Storage.DefaultName, stored.DetectedMIME, lockedMetadata.Extension, stored.ByteCount, stored.SHA256[:], actorID))
		if err != nil {
			return err
		}
		projectRevision, err := updateProjectAfterArtifactMutation(ctx, transaction, projectID, projectStatus)
		if err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, artifactAuditEvent(actorID, "artifact.replaced", result, map[string]any{"project_revision": projectRevision, "replaced_artifact_id": artifactID.String(), "replacement_artifact_id": replacementID.String()}))
	})
	if err != nil {
		_ = service.Storage.Default().RemoveNew(ctx, stored.StorageKey)
		return Artifact{}, err
	}
	return result, nil
}

func (service Service) Get(ctx context.Context, artifactID uuid.UUID) (Artifact, error) {
	return scanArtifact(service.Pool.QueryRow(ctx, `SELECT id, project_id, type, display_name, original_filename, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, revision, actor_id, created_at, updated_at, deleted_at FROM artifacts WHERE id=$1`, artifactID))
}

func (service Service) CurrentRevision(ctx context.Context, artifactID uuid.UUID) (int64, error) {
	var revision int64
	err := service.Pool.QueryRow(ctx, "SELECT revision FROM artifacts WHERE id=$1", artifactID).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return revision, err
}

func (service Service) OpenPublic(ctx context.Context, artifactID uuid.UUID) (PublicContent, error) {
	artifact, err := scanArtifact(service.Pool.QueryRow(ctx, `
		SELECT artifacts.id, artifacts.project_id, artifacts.type, artifacts.display_name, artifacts.original_filename, artifacts.storage_key, artifacts.storage_backend, artifacts.mime_type, artifacts.extension, artifacts.byte_count, artifacts.sha256, artifacts.status, artifacts.revision, artifacts.actor_id, artifacts.created_at, artifacts.updated_at, artifacts.deleted_at
		FROM artifacts JOIN projects ON projects.id=artifacts.project_id
		WHERE artifacts.id=$1 AND artifacts.status='active' AND projects.status='published'`, artifactID))
	if err != nil {
		return PublicContent{}, err
	}
	backend, lookupErr := service.Storage.Lookup(artifact.StorageBackend)
	if lookupErr != nil {
		return PublicContent{}, ErrContentUnavailable
	}
	file, size, err := backend.Open(ctx, artifact.StorageKey)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrUnsafeStoragePath) || errors.Is(err, ErrContentUnavailable) {
		return PublicContent{}, ErrContentUnavailable
	}
	if err != nil {
		return PublicContent{}, err
	}
	return PublicContent{Artifact: artifact, File: file, Size: size}, nil
}

func (service Service) checkQuota(ctx context.Context, transaction pgx.Tx, projectID uuid.UUID, additionalBytes int64) error {
	return service.checkReplacementQuota(ctx, transaction, projectID, 0, additionalBytes)
}

func (service Service) checkReplacementQuota(ctx context.Context, transaction pgx.Tx, projectID uuid.UUID, replacedBytes, newBytes int64) error {
	if service.MaxProjectBytes <= 0 {
		return errors.New("Project Artifact quota must be positive")
	}
	var activeBytes int64
	if err := transaction.QueryRow(ctx, "SELECT COALESCE(sum(byte_count),0) FROM artifacts WHERE project_id=$1 AND status='active'", projectID).Scan(&activeBytes); err != nil {
		return err
	}
	if activeBytes-replacedBytes+newBytes > service.MaxProjectBytes {
		return ErrProjectQuotaExceeded
	}
	return nil
}

func artifactProjectID(ctx context.Context, transaction pgx.Tx, artifactID uuid.UUID) (uuid.UUID, error) {
	var projectID uuid.UUID
	err := transaction.QueryRow(ctx, "SELECT project_id FROM artifacts WHERE id=$1", artifactID).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return projectID, err
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

func lockArtifact(ctx context.Context, transaction pgx.Tx, artifactID uuid.UUID) (Artifact, error) {
	return scanArtifact(transaction.QueryRow(ctx, `SELECT id, project_id, type, display_name, original_filename, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, revision, actor_id, created_at, updated_at, deleted_at FROM artifacts WHERE id=$1 FOR UPDATE`, artifactID))
}

func updateProjectAfterArtifactMutation(ctx context.Context, transaction pgx.Tx, projectID uuid.UUID, projectStatus string) (int64, error) {
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

func artifactAuditEvent(actorID uuid.UUID, eventType string, artifact Artifact, metadata map[string]any) audit.Event {
	metadata["project_id"] = artifact.ProjectID.String()
	metadata["artifact_type"] = artifact.ArtifactType
	metadata["byte_count"] = artifact.ByteCount
	return audit.Event{ActorID: actorID, EventType: eventType, TargetType: "artifact", TargetID: artifact.ID, Metadata: metadata}
}

type rowScanner interface {
	Scan(destinations ...any) error
}

func scanArtifact(row rowScanner) (Artifact, error) {
	var result Artifact
	var digest []byte
	var actorID *uuid.UUID
	err := row.Scan(&result.ID, &result.ProjectID, &result.ArtifactType, &result.DisplayName, &result.OriginalFilename, &result.StorageKey, &result.StorageBackend, &result.MIMEType, &result.Extension, &result.ByteCount, &digest, &result.Status, &result.Revision, &actorID, &result.CreatedAt, &result.UpdatedAt, &result.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Artifact{}, ErrNotFound
	}
	if err != nil {
		return Artifact{}, err
	}
	if len(digest) != len(result.SHA256) {
		return Artifact{}, errors.New("stored Artifact digest has an invalid length")
	}
	copy(result.SHA256[:], digest)
	result.ActorID = actorID
	return result, nil
}
