package artifacts

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrationResult reports one artifact-storage migration run.
type MigrationResult struct {
	OnSourceBackend int
	OnTargetBackend int
	Planned         int
	Copied          int
	Verified        int
	Failed          []MigrationFailure
}

// MigrationFailure records one Artifact whose bytes could not be copied.
type MigrationFailure struct {
	ArtifactID uuid.UUID
	StorageKey string
	Err        error
}

// MigrateStorage copies Artifact bytes from the local filesystem backend to
// the configured default backend under each record's existing storage key
// and stamps its storage_backend marker after the copied digest matches the
// stored digest. Runs are idempotent: already-stamped rows are skipped by
// the work-list query and the guarded update makes concurrent runs safe. A
// digest mismatch aborts without stamping, because it means the source bytes
// and the record disagree. Per-row copy failures are collected and counted
// instead of aborting, so one unreadable file does not block the rest.
func MigrateStorage(ctx context.Context, pool *pgxpool.Pool, set StorageSet, apply bool) (MigrationResult, error) {
	target := set.Default()
	if target.Name() == BackendLocal {
		return MigrationResult{}, errors.New("artifact storage migration requires a non-local default backend")
	}
	source, err := set.Lookup(BackendLocal)
	if err != nil {
		return MigrationResult{}, err
	}

	result := MigrationResult{}
	backendRows, err := pool.Query(ctx, `SELECT storage_backend, count(*) FROM artifacts GROUP BY storage_backend`)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("count artifacts by backend: %w", err)
	}
	defer backendRows.Close()
	for backendRows.Next() {
		var name string
		var count int
		if err := backendRows.Scan(&name, &count); err != nil {
			return MigrationResult{}, fmt.Errorf("scan artifact backend counts: %w", err)
		}
		switch name {
		case BackendLocal:
			result.OnSourceBackend = count
		case target.Name():
			result.OnTargetBackend = count
		}
	}
	if err := backendRows.Err(); err != nil {
		return MigrationResult{}, fmt.Errorf("iterate artifact backend counts: %w", err)
	}

	rows, err := pool.Query(ctx, `SELECT id, storage_key, byte_count, sha256 FROM artifacts WHERE storage_backend=$1 ORDER BY created_at, id`, BackendLocal)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("select artifacts for migration: %w", err)
	}
	defer rows.Close()

	type workItem struct {
		id        uuid.UUID
		key       string
		byteCount int64
		digest    []byte
	}
	work := []workItem{}
	for rows.Next() {
		var item workItem
		if err := rows.Scan(&item.id, &item.key, &item.byteCount, &item.digest); err != nil {
			return MigrationResult{}, fmt.Errorf("scan artifact for migration: %w", err)
		}
		work = append(work, item)
	}
	if err := rows.Err(); err != nil {
		return MigrationResult{}, fmt.Errorf("iterate artifacts for migration: %w", err)
	}
	result.Planned = len(work)

	for _, item := range work {
		if !apply {
			continue
		}
		reader, _, err := source.Open(ctx, item.key)
		if err != nil {
			result.Failed = append(result.Failed, MigrationFailure{ArtifactID: item.id, StorageKey: item.key, Err: fmt.Errorf("open source: %w", err)})
			continue
		}
		stored, putErr := target.PutAt(ctx, item.key, reader, item.byteCount)
		_ = reader.Close()
		if putErr != nil {
			result.Failed = append(result.Failed, MigrationFailure{ArtifactID: item.id, StorageKey: item.key, Err: fmt.Errorf("copy to %s: %w", target.Name(), putErr)})
			continue
		}
		if stored.ByteCount != item.byteCount {
			result.Failed = append(result.Failed, MigrationFailure{ArtifactID: item.id, StorageKey: item.key, Err: fmt.Errorf("size mismatch: copied %d bytes, record holds %d", stored.ByteCount, item.byteCount)})
			continue
		}
		if string(stored.SHA256[:]) != string(item.digest) {
			return result, fmt.Errorf("digest mismatch for artifact %s (%s): copied bytes do not match the stored digest, no marker updated", item.id, item.key)
		}
		tag, err := pool.Exec(ctx, `UPDATE artifacts SET storage_backend=$2 WHERE id=$1 AND storage_backend=$3`, item.id, target.Name(), BackendLocal)
		if err != nil {
			result.Failed = append(result.Failed, MigrationFailure{ArtifactID: item.id, StorageKey: item.key, Err: fmt.Errorf("stamp storage backend: %w", err)})
			continue
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		result.Copied++
		result.Verified++
	}
	return result, nil
}
