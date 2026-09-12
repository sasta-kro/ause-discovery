// Package projectlinks owns Project Repository Links: ordered Project
// metadata identifying source-control repositories. A link owns no bytes, so
// it never enters artifacts, storage providers, quotas, or Project File
// lists, and it carries only its last historical availability evidence.
package projectlinks

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"ause-discovery.local/backend/generated"
	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/platform/database"
	"ause-discovery.local/backend/internal/platform/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MaxURLLength = 2048

	AvailabilityAccessible    = "accessible"
	AvailabilityNotAccessible = "not_accessible"
	AvailabilityUnverified    = "unverified"
)

var (
	ErrNotFound            = errors.New("project repository links not found")
	ErrRevisionConflict    = errors.New("project repository link revision conflict")
	ErrInvalidState        = errors.New("project repository link operation is not permitted in the current state")
	ErrInvalidURL          = errors.New("project repository link URL is invalid")
	ErrInvalidAvailability = errors.New("project repository link availability is invalid")
	ErrInvalidPrimary      = errors.New("project repository link primary designation is invalid")
	ErrDuplicateURL        = errors.New("project repository link URL is duplicated")
	ErrInvalidTimestamp    = errors.New("project repository link checked_at is invalid")
)

// Service owns Project Repository Link metadata.
type Service struct {
	Pool *pgxpool.Pool
}

type Link struct {
	ID           uuid.UUID
	ProjectID    uuid.UUID
	URL          string
	IsPrimary    bool
	Availability string
	CheckedAt    time.Time
	SortOrder    int
}

// Input is one desired repository link before validation.
type Input struct {
	URL          string
	IsPrimary    bool
	Availability string
	CheckedAt    time.Time
}

// Validate normalizes and validates one complete ordered input set. It
// returns the normalized set with sort orders assigned by position.
func Validate(inputs []Input) ([]Link, error) {
	normalized := make([]Link, 0, len(inputs))
	seen := map[string]bool{}
	primarySeen := false
	for index, input := range inputs {
		trimmed := strings.TrimSpace(input.URL)
		if trimmed == "" || len(trimmed) > MaxURLLength {
			return nil, fmt.Errorf("%w: %q", ErrInvalidURL, input.URL)
		}
		if strings.ContainsAny(trimmed, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f") {
			return nil, fmt.Errorf("%w: control characters", ErrInvalidURL)
		}
		parsed, err := url.Parse(trimmed)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, fmt.Errorf("%w: %q must be an absolute https URL with a hostname", ErrInvalidURL, trimmed)
		}
		if parsed.User != nil {
			return nil, fmt.Errorf("%w: credentials are not permitted", ErrInvalidURL)
		}
		if parsed.Fragment != "" || parsed.EscapedFragment() != "" {
			return nil, fmt.Errorf("%w: fragments are not permitted", ErrInvalidURL)
		}
		canonical := parsed.String()
		if seen[canonical] {
			return nil, fmt.Errorf("%w: %q appears more than once", ErrDuplicateURL, canonical)
		}
		seen[canonical] = true
		switch input.Availability {
		case AvailabilityAccessible, AvailabilityNotAccessible, AvailabilityUnverified:
		default:
			return nil, fmt.Errorf("%w: %q", ErrInvalidAvailability, input.Availability)
		}
		if input.CheckedAt.IsZero() {
			return nil, fmt.Errorf("%w: a checked_at timestamp is required", ErrInvalidTimestamp)
		}
		if input.IsPrimary {
			if primarySeen {
				return nil, fmt.Errorf("%w: at most one primary link is permitted", ErrInvalidPrimary)
			}
			primarySeen = true
		}
		normalized = append(normalized, Link{URL: canonical, IsPrimary: input.IsPrimary, Availability: input.Availability, CheckedAt: input.CheckedAt, SortOrder: index})
	}
	if len(normalized) > 0 && !primarySeen {
		return nil, fmt.Errorf("%w: a nonempty set requires exactly one primary link", ErrInvalidPrimary)
	}
	return normalized, nil
}

// List returns the Project's repository links in sort_order, id order.
func (service Service) List(ctx context.Context, projectID uuid.UUID) ([]Link, error) {
	rows, err := service.Pool.Query(ctx, `
		SELECT id, project_id, url, is_primary, availability, checked_at, sort_order
		FROM project_repository_links WHERE project_id=$1
		ORDER BY sort_order, id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := []Link{}
	for rows.Next() {
		link, scanErr := scanLink(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

// Equal reports whether the stored set matches the desired normalized set in
// content and order. Planning uses it to classify unchanged link sets.
func (service Service) Equal(ctx context.Context, projectID uuid.UUID, desired []Link) (bool, error) {
	current, err := service.List(ctx, projectID)
	if err != nil {
		return false, err
	}
	if len(current) != len(desired) {
		return false, nil
	}
	for index := range current {
		if current[index].URL != desired[index].URL || current[index].IsPrimary != desired[index].IsPrimary || current[index].Availability != desired[index].Availability || !current[index].CheckedAt.Equal(desired[index].CheckedAt) || current[index].SortOrder != desired[index].SortOrder {
			return false, nil
		}
	}
	return true, nil
}

// Replace authoritatively replaces the Project's repository-link set. An
// unchanged set writes nothing; a changed set deletes and reinserts rows in
// one transaction, advances the Project revision exactly once, records one
// audit event, and re-queues search synchronization for the Project's
// status. Revision enforcement and deleted-Project rejection follow the
// shared Project-mutation pattern.
func (service Service) Replace(ctx context.Context, actorID, projectID uuid.UUID, expectedProjectRevision int64, inputs []Input) (bool, error) {
	desired, err := Validate(inputs)
	if err != nil {
		return false, err
	}
	replaced := false
	err = database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		projectRevision, projectStatus, lockErr := lockProject(ctx, transaction, projectID)
		if lockErr != nil {
			return lockErr
		}
		if projectRevision != expectedProjectRevision {
			return ErrRevisionConflict
		}
		if projectStatus == "deleted" {
			return ErrInvalidState
		}
		current, listErr := listForUpdate(ctx, transaction, projectID)
		if listErr != nil {
			return listErr
		}
		if setsEqual(current, desired) {
			return nil
		}
		if _, err := transaction.Exec(ctx, "DELETE FROM project_repository_links WHERE project_id=$1", projectID); err != nil {
			return err
		}
		for _, link := range desired {
			if _, err := transaction.Exec(ctx, `
				INSERT INTO project_repository_links (id, project_id, url, is_primary, availability, checked_at, sort_order, actor_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				uuid.Must(uuid.NewV7()), projectID, link.URL, link.IsPrimary, link.Availability, link.CheckedAt, link.SortOrder, actorID); err != nil {
				return err
			}
		}
		updatedRevision, advanceErr := advanceProjectAfterLinkMutation(ctx, transaction, projectID, projectStatus)
		if advanceErr != nil {
			return advanceErr
		}
		replaced = true
		return audit.AppendTx(ctx, transaction, audit.Event{
			ActorID: actorID, EventType: "project_repository_links.replaced", TargetType: "project", TargetID: projectID,
			Metadata: map[string]any{"old_count": len(current), "new_count": len(desired), "project_revision": updatedRevision},
		})
	})
	if err != nil {
		return false, err
	}
	return replaced, nil
}

func setsEqual(current, desired []Link) bool {
	if len(current) != len(desired) {
		return false
	}
	for index := range current {
		if current[index].URL != desired[index].URL || current[index].IsPrimary != desired[index].IsPrimary || current[index].Availability != desired[index].Availability || !current[index].CheckedAt.Equal(desired[index].CheckedAt) || current[index].SortOrder != desired[index].SortOrder {
			return false
		}
	}
	return true
}

func listForUpdate(ctx context.Context, transaction pgx.Tx, projectID uuid.UUID) ([]Link, error) {
	rows, err := transaction.Query(ctx, `
		SELECT id, project_id, url, is_primary, availability, checked_at, sort_order
		FROM project_repository_links WHERE project_id=$1
		ORDER BY sort_order, id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := []Link{}
	for rows.Next() {
		link, scanErr := scanLink(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		links = append(links, link)
	}
	return links, rows.Err()
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

func advanceProjectAfterLinkMutation(ctx context.Context, transaction pgx.Tx, projectID uuid.UUID, projectStatus string) (int64, error) {
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

func scanLink(row rowScanner) (Link, error) {
	var result Link
	err := row.Scan(&result.ID, &result.ProjectID, &result.URL, &result.IsPrimary, &result.Availability, &result.CheckedAt, &result.SortOrder)
	if err != nil {
		return Link{}, err
	}
	return result, nil
}
