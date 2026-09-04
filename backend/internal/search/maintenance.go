package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ause-discovery.local/backend/generated"
	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/platform/database"
	"ause-discovery.local/backend/internal/platform/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrProjectNotFound = errors.New("Project not found")
	ErrRebuildNotFound = errors.New("search rebuild not found")
	ErrRebuildActive   = errors.New("a search rebuild is already active")
)

type Status struct {
	Available     bool
	PendingCount  int
	FailedCount   int
	ActiveRebuild *RebuildOperation
}

type RebuildOperation struct {
	ID                uuid.UUID
	State             string
	PhysicalIndexName string
	RequestedBy       uuid.UUID
	RequestedAt       time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
	TotalProjects     int
	ProcessedProjects int
	FailedProjects    int
	ErrorCode         *string
}

type ReindexResult struct {
	ProjectID       uuid.UUID
	DesiredRevision int64
	State           string
}

func (service Service) SearchStatus(ctx context.Context) (Status, error) {
	var status Status
	if err := service.Pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE state IN ('pending','processing')), count(*) FILTER (WHERE state='failed') FROM project_search_sync`).Scan(&status.PendingCount, &status.FailedCount); err != nil {
		return Status{}, err
	}
	operation, err := service.activeRebuild(ctx)
	if err != nil {
		return Status{}, err
	}
	status.ActiveRebuild = operation
	status.Available = service.Index.Health(ctx) == nil
	return status, nil
}

func (service Service) ReindexProject(ctx context.Context, actorID, projectID uuid.UUID) (ReindexResult, error) {
	var result ReindexResult
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		var revision int64
		var projectStatus string
		err := transaction.QueryRow(ctx, "SELECT revision, status FROM projects WHERE id=$1 FOR UPDATE", projectID).Scan(&revision, &projectStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProjectNotFound
		}
		if err != nil {
			return err
		}
		desiredAction := "remove"
		if projectStatus == "published" {
			desiredAction = "upsert"
		}
		row, err := generated.New(transaction).UpsertProjectSearchSync(ctx, generated.UpsertProjectSearchSyncParams{ProjectID: identity.UUID(projectID), DesiredRevision: revision, DesiredAction: desiredAction})
		if err != nil {
			return err
		}
		if err := audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "search.project_reindexed", TargetType: "project", TargetID: projectID, Metadata: map[string]any{"desired_revision": revision, "desired_action": desiredAction}}); err != nil {
			return err
		}
		result = ReindexResult{ProjectID: projectID, DesiredRevision: revision, State: row.State}
		return nil
	})
	return result, err
}

func (service Service) CreateRebuild(ctx context.Context, actorID uuid.UUID) (RebuildOperation, error) {
	operationID := uuid.Must(uuid.NewV7())
	physicalIndexName := fmt.Sprintf("%s_rebuild_%d_%s_%s", service.indexUID(), SchemaVersion, time.Now().UTC().Format("20060102t150405"), strings.ReplaceAll(operationID.String()[:8], "-", ""))
	var operation RebuildOperation
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		row := transaction.QueryRow(ctx, `INSERT INTO search_rebuild_operations (id, state, physical_index_name, initiated_by) VALUES ($1, 'pending', $2, $3) RETURNING id, state, physical_index_name, initiated_by, created_at, started_at, completed_at, total_count, processed_count, failed_count, error_detail`, operationID, physicalIndexName, identity.NullableUUID(actorID))
		var err error
		operation, err = scanRebuildOperation(row)
		if err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "search.rebuild_requested", TargetType: "search_rebuild", TargetID: operationID, Metadata: map[string]any{"physical_index_name": physicalIndexName, "schema_version": SchemaVersion}})
	})
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return RebuildOperation{}, ErrRebuildActive
	}
	return operation, err
}

func (service Service) GetRebuild(ctx context.Context, operationID uuid.UUID) (RebuildOperation, error) {
	return scanRebuildOperation(service.Pool.QueryRow(ctx, `SELECT id, state, physical_index_name, initiated_by, created_at, started_at, completed_at, total_count, processed_count, failed_count, error_detail FROM search_rebuild_operations WHERE id=$1`, operationID))
}

func (service Service) activeRebuild(ctx context.Context) (*RebuildOperation, error) {
	operation, err := scanRebuildOperation(service.Pool.QueryRow(ctx, `SELECT id, state, physical_index_name, initiated_by, created_at, started_at, completed_at, total_count, processed_count, failed_count, error_detail FROM search_rebuild_operations WHERE state IN ('pending','processing') ORDER BY created_at LIMIT 1`))
	if errors.Is(err, ErrRebuildNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &operation, nil
}

func scanRebuildOperation(row rowScanner) (RebuildOperation, error) {
	var operation RebuildOperation
	var initiatedBy *uuid.UUID
	var errorDetail *string
	err := row.Scan(&operation.ID, &operation.State, &operation.PhysicalIndexName, &initiatedBy, &operation.RequestedAt, &operation.StartedAt, &operation.CompletedAt, &operation.TotalProjects, &operation.ProcessedProjects, &operation.FailedProjects, &errorDetail)
	if errors.Is(err, pgx.ErrNoRows) {
		return RebuildOperation{}, ErrRebuildNotFound
	}
	if err != nil {
		return RebuildOperation{}, err
	}
	if initiatedBy != nil {
		operation.RequestedBy = *initiatedBy
	}
	if errorDetail != nil {
		code := strings.SplitN(*errorDetail, ":", 2)[0]
		if code == "" {
			code = "search_rebuild_failed"
		}
		operation.ErrorCode = &code
	}
	return operation, nil
}

type rowScanner interface {
	Scan(destinations ...any) error
}
