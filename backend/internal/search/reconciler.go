package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reconciler struct {
	Pool          *pgxpool.Pool
	Index         Index
	IndexUID      string
	WorkerID      string
	BatchSize     int
	LeaseDuration time.Duration
	PollInterval  time.Duration
	Logger        *slog.Logger
}

type claimedSync struct {
	ProjectID       uuid.UUID
	DesiredRevision int64
	DesiredAction   string
	AttemptCount    int
}

func (reconciler Reconciler) Run(ctx context.Context) {
	logger := reconciler.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if _, err := reconciler.Pool.Exec(ctx, `UPDATE search_rebuild_operations SET state='failed', error_detail='search_rebuild_interrupted: API process restarted', completed_at=now(), updated_at=now() WHERE state='processing'`); err != nil && ctx.Err() == nil {
		logger.Error("search rebuild recovery failed", "error", err)
	}
	interval := reconciler.PollInterval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	indexReady := false
	for {
		if !indexReady {
			if err := reconciler.Index.EnsureIndex(ctx, reconciler.indexUID()); err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Error("search index initialization failed", "error", err)
			} else {
				indexReady = true
			}
		}
		if indexReady {
			if _, err := reconciler.ProcessRebuildOnce(ctx); err != nil && ctx.Err() == nil {
				logger.Error("search rebuild failed", "error", err)
			}
			if _, err := reconciler.ReconcileOnce(ctx); err != nil && ctx.Err() == nil {
				logger.Error("search reconciliation batch failed", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (reconciler Reconciler) ReconcileOnce(ctx context.Context) (int, error) {
	claims, err := reconciler.claimSync(ctx)
	if err != nil {
		return 0, err
	}
	errorsFound := []error{}
	for _, claim := range claims {
		operationErr := reconciler.applyClaim(ctx, claim)
		if operationErr == nil {
			if err := reconciler.markSucceeded(ctx, claim); err != nil {
				errorsFound = append(errorsFound, err)
			}
			continue
		}
		if err := reconciler.markFailed(ctx, claim, operationErr); err != nil {
			errorsFound = append(errorsFound, errors.Join(operationErr, err))
		} else {
			errorsFound = append(errorsFound, operationErr)
		}
	}
	return len(claims), errors.Join(errorsFound...)
}

func (reconciler Reconciler) claimSync(ctx context.Context) ([]claimedSync, error) {
	batchSize := reconciler.BatchSize
	if batchSize <= 0 || batchSize > 100 {
		batchSize = 20
	}
	leaseDuration := reconciler.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	workerID := reconciler.workerID()
	rows, err := reconciler.Pool.Query(ctx, `
		WITH claimable AS (
			SELECT project_id FROM project_search_sync
			WHERE ((state IN ('pending','failed') AND (next_attempt_at IS NULL OR next_attempt_at <= now())) OR (state='processing' AND lease_expires_at <= now()))
			ORDER BY updated_at, project_id LIMIT $1 FOR UPDATE SKIP LOCKED
		)
		UPDATE project_search_sync AS sync
		SET state='processing', attempt_count=attempt_count+1, lease_owner=$2, lease_expires_at=$3, last_attempt_at=now(), next_attempt_at=NULL, error_detail=NULL, updated_at=now()
		FROM claimable WHERE sync.project_id=claimable.project_id
		RETURNING sync.project_id, sync.desired_revision, sync.desired_action, sync.attempt_count`, batchSize, workerID, time.Now().Add(leaseDuration))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	claims := []claimedSync{}
	for rows.Next() {
		var claim claimedSync
		if err := rows.Scan(&claim.ProjectID, &claim.DesiredRevision, &claim.DesiredAction, &claim.AttemptCount); err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}
	return claims, rows.Err()
}

func (reconciler Reconciler) applyClaim(ctx context.Context, claim claimedSync) error {
	if claim.DesiredAction == "remove" {
		return reconciler.Index.DeleteDocument(ctx, reconciler.indexUID(), claim.ProjectID)
	}
	document, err := BuildProjectDocument(ctx, reconciler.Pool, claim.ProjectID)
	if errors.Is(err, ErrProjectNotPublished) {
		return reconciler.Index.DeleteDocument(ctx, reconciler.indexUID(), claim.ProjectID)
	}
	if err != nil {
		return err
	}
	if document.Revision != claim.DesiredRevision {
		return errors.New("Project revision changed before search projection")
	}
	return reconciler.Index.UpsertDocuments(ctx, reconciler.indexUID(), []Document{document})
}

func (reconciler Reconciler) markSucceeded(ctx context.Context, claim claimedSync) error {
	_, err := reconciler.Pool.Exec(ctx, `UPDATE project_search_sync SET indexed_revision=desired_revision, state='synced', lease_owner=NULL, lease_expires_at=NULL, error_detail=NULL, synchronized_at=now(), updated_at=now() WHERE project_id=$1 AND desired_revision=$2 AND state='processing' AND lease_owner=$3`, claim.ProjectID, claim.DesiredRevision, reconciler.workerID())
	return err
}

func (reconciler Reconciler) markFailed(ctx context.Context, claim claimedSync, operationErr error) error {
	detail := truncateUTF8(operationErr.Error(), 4096)
	nextAttempt := time.Now().Add(retryDelay(claim.ProjectID, claim.AttemptCount))
	_, err := reconciler.Pool.Exec(ctx, `UPDATE project_search_sync SET state='failed', lease_owner=NULL, lease_expires_at=NULL, next_attempt_at=$4, error_detail=$5, updated_at=now() WHERE project_id=$1 AND desired_revision=$2 AND state='processing' AND lease_owner=$3`, claim.ProjectID, claim.DesiredRevision, reconciler.workerID(), nextAttempt, detail)
	return err
}

func (reconciler Reconciler) ProcessRebuildOnce(ctx context.Context) (bool, error) {
	operation, found, err := reconciler.claimRebuild(ctx)
	if err != nil || !found {
		return found, err
	}
	if err := reconciler.executeRebuild(ctx, operation); err != nil {
		detail := "search_rebuild_failed: " + truncateUTF8(err.Error(), 4000)
		_, updateErr := reconciler.Pool.Exec(ctx, `UPDATE search_rebuild_operations SET state='failed', error_detail=$2, completed_at=now(), updated_at=now() WHERE id=$1 AND state='processing'`, operation.ID, detail)
		return true, errors.Join(err, updateErr)
	}
	return true, nil
}

func (reconciler Reconciler) claimRebuild(ctx context.Context) (RebuildOperation, bool, error) {
	var operation RebuildOperation
	found := false
	err := database.InTransaction(ctx, reconciler.Pool, func(transaction pgx.Tx) error {
		row := transaction.QueryRow(ctx, `SELECT id, state, physical_index_name, initiated_by, created_at, started_at, completed_at, total_count, processed_count, failed_count, error_detail FROM search_rebuild_operations WHERE state='pending' ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED`)
		pending, err := scanRebuildOperation(row)
		if errors.Is(err, ErrRebuildNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var total int
		if err := transaction.QueryRow(ctx, "SELECT count(*) FROM projects WHERE status='published'").Scan(&total); err != nil {
			return err
		}
		operation, err = scanRebuildOperation(transaction.QueryRow(ctx, `UPDATE search_rebuild_operations SET state='processing', started_at=now(), total_count=$2, updated_at=now() WHERE id=$1 RETURNING id, state, physical_index_name, initiated_by, created_at, started_at, completed_at, total_count, processed_count, failed_count, error_detail`, pending.ID, total))
		if err != nil {
			return err
		}
		found = true
		return nil
	})
	return operation, found, err
}

func (reconciler Reconciler) executeRebuild(ctx context.Context, operation RebuildOperation) error {
	if err := reconciler.Index.EnsureIndex(ctx, reconciler.indexUID()); err != nil {
		return err
	}
	if err := reconciler.Index.EnsureIndex(ctx, operation.PhysicalIndexName); err != nil {
		return err
	}
	projectIDs, err := reconciler.publishedProjectIDs(ctx)
	if err != nil {
		return err
	}
	initialProjectIDs := map[uuid.UUID]bool{}
	for _, projectID := range projectIDs {
		initialProjectIDs[projectID] = true
	}
	batchSize := reconciler.BatchSize
	if batchSize <= 0 || batchSize > 100 {
		batchSize = 20
	}
	processed := 0
	for start := 0; start < len(projectIDs); start += batchSize {
		end := start + batchSize
		if end > len(projectIDs) {
			end = len(projectIDs)
		}
		documents := make([]Document, 0, end-start)
		for _, projectID := range projectIDs[start:end] {
			document, err := BuildProjectDocument(ctx, reconciler.Pool, projectID)
			if errors.Is(err, ErrProjectNotPublished) {
				continue
			}
			if err != nil {
				return err
			}
			documents = append(documents, document)
		}
		if err := reconciler.Index.UpsertDocuments(ctx, operation.PhysicalIndexName, documents); err != nil {
			return err
		}
		processed += len(documents)
		if _, err := reconciler.Pool.Exec(ctx, `UPDATE search_rebuild_operations SET processed_count=LEAST($2,total_count), updated_at=now() WHERE id=$1 AND state='processing'`, operation.ID, processed); err != nil {
			return err
		}
	}
	return database.InTransaction(ctx, reconciler.Pool, func(transaction pgx.Tx) error {
		if _, err := transaction.Exec(ctx, `LOCK TABLE projects, project_title_aliases, project_participations, project_taxonomy_values, artifacts IN SHARE MODE`); err != nil {
			return err
		}
		finalProjectIDs, err := reconciler.publishedProjectIDs(ctx)
		if err != nil {
			return err
		}
		finalProjectIDSet := map[uuid.UUID]bool{}
		for _, projectID := range finalProjectIDs {
			finalProjectIDSet[projectID] = true
		}
		for projectID := range initialProjectIDs {
			if !finalProjectIDSet[projectID] {
				if err := reconciler.Index.DeleteDocument(ctx, operation.PhysicalIndexName, projectID); err != nil {
					return err
				}
			}
		}
		for start := 0; start < len(finalProjectIDs); start += batchSize {
			end := start + batchSize
			if end > len(finalProjectIDs) {
				end = len(finalProjectIDs)
			}
			documents := make([]Document, 0, end-start)
			for _, projectID := range finalProjectIDs[start:end] {
				document, err := BuildProjectDocument(ctx, reconciler.Pool, projectID)
				if err != nil {
					return err
				}
				documents = append(documents, document)
			}
			if err := reconciler.Index.UpsertDocuments(ctx, operation.PhysicalIndexName, documents); err != nil {
				return err
			}
		}
		stats, err := reconciler.Index.Stats(ctx, operation.PhysicalIndexName)
		if err != nil {
			return err
		}
		if stats.NumberOfDocuments != len(finalProjectIDs) {
			return fmt.Errorf("rebuild document count %d does not match published Project count %d", stats.NumberOfDocuments, len(finalProjectIDs))
		}
		if err := reconciler.Index.SwapIndexes(ctx, reconciler.indexUID(), operation.PhysicalIndexName); err != nil {
			return err
		}
		if _, err := transaction.Exec(ctx, `
			UPDATE project_search_sync AS sync SET desired_revision=projects.revision, indexed_revision=projects.revision,
			desired_action=CASE WHEN projects.status='published' THEN 'upsert' ELSE 'remove' END,
			state='synced', next_attempt_at=NULL, lease_owner=NULL, lease_expires_at=NULL, error_detail=NULL, synchronized_at=now(), updated_at=now()
			FROM projects WHERE sync.project_id=projects.id`); err != nil {
			return err
		}
		if _, err := transaction.Exec(ctx, `UPDATE search_rebuild_operations SET state='completed', total_count=$2, processed_count=$2, completed_at=now(), updated_at=now() WHERE id=$1 AND state='processing'`, operation.ID, len(finalProjectIDs)); err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: operation.RequestedBy, EventType: "search.rebuild_completed", TargetType: "search_rebuild", TargetID: operation.ID, Metadata: map[string]any{"project_count": len(finalProjectIDs), "physical_index_name": operation.PhysicalIndexName}})
	})
}

func (reconciler Reconciler) publishedProjectIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := reconciler.Pool.Query(ctx, "SELECT id FROM projects WHERE status='published' ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projectIDs := []uuid.UUID{}
	for rows.Next() {
		var projectID uuid.UUID
		if err := rows.Scan(&projectID); err != nil {
			return nil, err
		}
		projectIDs = append(projectIDs, projectID)
	}
	return projectIDs, rows.Err()
}

func (reconciler Reconciler) indexUID() string {
	if strings.TrimSpace(reconciler.IndexUID) == "" {
		return "projects"
	}
	return reconciler.IndexUID
}

func (reconciler Reconciler) workerID() string {
	if strings.TrimSpace(reconciler.WorkerID) != "" {
		return reconciler.WorkerID
	}
	return "search-reconciler"
}

func retryDelay(projectID uuid.UUID, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exponent := attempt
	if exponent > 8 {
		exponent = 8
	}
	delay := time.Duration(1<<exponent) * time.Second
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	jitter := time.Duration((int(projectID[0])+attempt*37)%1000) * time.Millisecond
	return delay + jitter
}

func truncateUTF8(value string, maximumBytes int) string {
	if len(value) <= maximumBytes {
		return value
	}
	value = value[:maximumBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
