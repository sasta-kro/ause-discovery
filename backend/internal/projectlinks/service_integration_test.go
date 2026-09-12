package projectlinks

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestValidateRejectsInvalidLinkSets(t *testing.T) {
	checked := time.Date(2026, 9, 11, 7, 32, 27, 0, time.UTC)
	cases := map[string][]Input{
		"http url":             {{URL: "http://github.com/example/repo", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}},
		"relative url":         {{URL: "/example/repo", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}},
		"missing host":         {{URL: "https:///repo", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}},
		"port without host":    {{URL: "https://:443/repo", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}},
		"overlong in runes":    {{URL: "https://github.com/" + strings.Repeat("é", 1100), IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}},
		"credentials":          {{URL: "https://user:secret@github.com/example/repo", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}},
		"fragment":             {{URL: "https://github.com/example/repo#readme", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}},
		"control characters":   {{URL: "https://github.com/example/repo\x00", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}},
		"overlong url":         {{URL: "https://github.com/" + strings.Repeat("a", MaxURLLength), IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}},
		"blank url":            {{URL: "   ", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}},
		"invalid availability": {{URL: "https://github.com/example/repo", IsPrimary: true, Availability: "public", CheckedAt: checked}},
		"missing timestamp":    {{URL: "https://github.com/example/repo", IsPrimary: true, Availability: AvailabilityAccessible}},
		"duplicate url": {
			{URL: "https://github.com/example/repo", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked},
			{URL: "https://github.com/example/repo", IsPrimary: false, Availability: AvailabilityAccessible, CheckedAt: checked},
		},
		"multiple primary": {
			{URL: "https://github.com/example/one", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked},
			{URL: "https://github.com/example/two", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked},
		},
		"missing primary": {
			{URL: "https://github.com/example/one", IsPrimary: false, Availability: AvailabilityAccessible, CheckedAt: checked},
			{URL: "https://github.com/example/two", IsPrimary: false, Availability: AvailabilityAccessible, CheckedAt: checked},
		},
	}
	for name, inputs := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Validate(inputs); err == nil {
				t.Fatalf("%s was accepted", name)
			}
		})
	}

	normalized, err := Validate([]Input{
		{URL: "  https://github.com/Example/Repo  ", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked},
	})
	if err != nil {
		t.Fatalf("valid set returned an error: %v", err)
	}
	if normalized[0].URL != "https://github.com/Example/Repo" || normalized[0].SortOrder != 0 {
		t.Fatalf("normalization produced %#v", normalized[0])
	}
	if _, err := Validate([]Input{}); err != nil {
		t.Fatalf("empty set returned an error: %v", err)
	}
	// PostgreSQL length(text) counts characters of the stored canonical
	// value: a multibyte path percent-encodes during normalization, so the
	// bound applies to the canonical form. A URL whose canonical form stays
	// within 2048 characters is valid for both the application and the
	// schema even though its raw bytes grow.
	multibyte := Input{URL: "https://github.com/" + strings.Repeat("é", 300), IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked}
	accepted, err := Validate([]Input{multibyte})
	if err != nil {
		t.Fatalf("canonical-bounded multibyte URL rejected: %v", err)
	}
	if !strings.Contains(accepted[0].URL, "%C3%A9") || utf8.RuneCountInString(accepted[0].URL) > MaxURLLength {
		t.Fatalf("canonical form was %q", accepted[0].URL)
	}
}

func TestReplaceValidatesOrdersRevisionsAndAudit(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLinksTestDatabase(t, ctx, databaseURL)
	publishedID, draftID := seedLinksProjects(t, ctx, pool)
	service := Service{Pool: pool}
	actorID := uuid.MustParse("018f0000-0000-7000-8000-000000000e01")
	checked := time.Date(2026, 9, 11, 7, 32, 27, 0, time.UTC)

	replaced, err := service.Replace(ctx, actorID, publishedID, 1, []Input{
		{URL: "https://github.com/example/one", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked},
		{URL: "https://github.com/example/two", IsPrimary: false, Availability: AvailabilityNotAccessible, CheckedAt: checked},
	})
	if err != nil || !replaced {
		t.Fatalf("first replace returned %t and %v, expected replacement", replaced, err)
	}
	links, listErr := service.List(ctx, publishedID)
	if listErr != nil || len(links) != 2 {
		t.Fatalf("List returned %d links with error %v", len(links), listErr)
	}
	if links[0].SortOrder != 0 || links[1].SortOrder != 1 || !links[0].IsPrimary || links[1].IsPrimary {
		t.Fatalf("stored ordering or primary designation was wrong: %#v", links)
	}
	assertLinksProjectState(t, ctx, pool, publishedID, 2, 1)

	// Exact rerun is unchanged: the revision is the search desired revision,
	// so an unchanged revision and audit count prove no write, revision
	// change, audit, or search update occurred.
	replaced, err = service.Replace(ctx, actorID, publishedID, 2, []Input{
		{URL: "https://github.com/example/one", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked},
		{URL: "https://github.com/example/two", IsPrimary: false, Availability: AvailabilityNotAccessible, CheckedAt: checked},
	})
	if err != nil || replaced {
		t.Fatalf("rerun returned %t and %v, expected unchanged", replaced, err)
	}
	assertLinksProjectState(t, ctx, pool, publishedID, 2, 1)

	// A changed set replaces once, including order and primary changes.
	replaced, err = service.Replace(ctx, actorID, publishedID, 2, []Input{
		{URL: "https://github.com/example/two", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked},
		{URL: "https://github.com/example/one", IsPrimary: false, Availability: AvailabilityAccessible, CheckedAt: checked.Add(time.Hour)},
	})
	if err != nil || !replaced {
		t.Fatalf("changed replace returned %t and %v", replaced, err)
	}
	links, _ = service.List(ctx, publishedID)
	if links[0].URL != "https://github.com/example/two" || !links[0].IsPrimary || links[1].Availability != AvailabilityAccessible {
		t.Fatalf("replacement stored the wrong set: %#v", links)
	}
	assertLinksProjectState(t, ctx, pool, publishedID, 3, 2)

	// Stale revision and deleted Project mutations fail safely.
	if _, err := service.Replace(ctx, actorID, publishedID, 1, []Input{
		{URL: "https://github.com/example/three", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked},
	}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale revision returned %v", err)
	}
	if _, err := service.Replace(ctx, actorID, draftID, 1, nil); err != nil {
		t.Fatalf("empty replacement on a draft Project returned %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE projects SET status='deleted', deleted_at=now() WHERE id=$1", draftID); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	if _, err := service.Replace(ctx, actorID, draftID, 1, []Input{
		{URL: "https://github.com/example/draft", IsPrimary: true, Availability: AvailabilityAccessible, CheckedAt: checked},
	}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("deleted Project returned %v", err)
	}

	// An explicitly empty set removes every row and still advances the
	// revision once with one audit event.
	replaced, err = service.Replace(ctx, actorID, publishedID, 3, nil)
	if err != nil || !replaced {
		t.Fatalf("empty-set removal returned %t and %v", replaced, err)
	}
	links, _ = service.List(ctx, publishedID)
	if len(links) != 0 {
		t.Fatalf("empty set left %d rows", len(links))
	}
	assertLinksProjectState(t, ctx, pool, publishedID, 4, 3)

	// Draft Projects accept and list links; the public boundary is the
	// parent Project lookup, not the link table.
	if _, err := pool.Exec(ctx, "UPDATE projects SET status='draft', deleted_at=NULL WHERE id=$1", draftID); err != nil {
		t.Fatalf("restore draft: %v", err)
	}
	if _, err := service.Replace(ctx, actorID, draftID, 1, []Input{
		{URL: "https://github.com/example/draft", IsPrimary: true, Availability: AvailabilityUnverified, CheckedAt: checked},
	}); err != nil {
		t.Fatalf("draft replacement returned %v", err)
	}
}

func TestSchemaConstraintsRejectInvalidRows(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLinksTestDatabase(t, ctx, databaseURL)
	publishedID, _ := seedLinksProjects(t, ctx, pool)
	base := "INSERT INTO project_repository_links (id, project_id, url, is_primary, availability, checked_at, sort_order) VALUES "
	first := "('" + uuid.NewString() + "', '" + publishedID.String() + "', 'https://github.com/example/one', true, 'accessible', now(), 0)"
	invalid := map[string]string{
		"duplicate url":        base + first + ", ('" + uuid.NewString() + "', '" + publishedID.String() + "', 'https://github.com/example/one', false, 'accessible', now(), 1)",
		"duplicate order":      base + first + ", ('" + uuid.NewString() + "', '" + publishedID.String() + "', 'https://github.com/example/two', false, 'accessible', now(), 0)",
		"multiple primary":     base + first + ", ('" + uuid.NewString() + "', '" + publishedID.String() + "', 'https://github.com/example/two', true, 'accessible', now(), 1)",
		"invalid availability": base + "('" + uuid.NewString() + "', '" + publishedID.String() + "', 'https://github.com/example/two', false, 'public', now(), 1)",
		"negative order":       base + "('" + uuid.NewString() + "', '" + publishedID.String() + "', 'https://github.com/example/two', false, 'accessible', now(), -1)",
		"blank url":            base + "('" + uuid.NewString() + "', '" + publishedID.String() + "', '   ', false, 'accessible', now(), 1)",
		"overlong url":         base + "('" + uuid.NewString() + "', '" + publishedID.String() + "', 'https://github.com/" + strings.Repeat("a", 2100) + "', false, 'accessible', now(), 1)",
	}
	for name, statement := range invalid {
		if _, err := pool.Exec(ctx, statement); err == nil {
			t.Fatalf("schema accepted %s", name)
		}
	}
}

func assertLinksProjectState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID, expectedRevision int64, expectedEvents int) {
	t.Helper()
	var revision int64
	var desiredRevision int64
	var action string
	err := pool.QueryRow(ctx, `SELECT projects.revision, project_search_sync.desired_revision, project_search_sync.desired_action FROM projects JOIN project_search_sync ON project_search_sync.project_id=projects.id WHERE projects.id=$1`, projectID).Scan(&revision, &desiredRevision, &action)
	if err != nil || revision != expectedRevision || desiredRevision != expectedRevision || action != "upsert" {
		t.Fatalf("Project state was revision %d desired %d action %q, error %v", revision, desiredRevision, action, err)
	}
	var events int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit_events WHERE event_type='project_repository_links.replaced' AND target_id=$1", projectID).Scan(&events); err != nil || events != expectedEvents {
		t.Fatalf("audit events were %d, expected %d, error %v", events, expectedEvents, err)
	}
}

func seedLinksProjects(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	publishedID := "018f0000-0000-7000-8000-000000000e10"
	draftID := "018f0000-0000-7000-8000-000000000e11"
	statements := []string{
		"INSERT INTO application_users (id, username, status) VALUES ('018f0000-0000-7000-8000-000000000e01', 'links-test-admin', 'active')",
		"INSERT INTO programs (id, key) VALUES ('018f0000-0000-7000-8000-000000000e12', 'links_program')",
		"INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000e13', '018f0000-0000-7000-8000-000000000e12', 'Links Program', 2020)",
		"INSERT INTO courses (id, key) VALUES ('018f0000-0000-7000-8000-000000000e14', 'links_course')",
		"INSERT INTO course_versions (id, course_id, label, code, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000e15', '018f0000-0000-7000-8000-000000000e14', 'Links Course', 'LNK499', 2020)",
		"INSERT INTO course_program_versions (course_version_id, program_version_id) VALUES ('018f0000-0000-7000-8000-000000000e15', '018f0000-0000-7000-8000-000000000e13')",
		"INSERT INTO people (id, display_name, normalized_name, student_id) VALUES ('018f0000-0000-7000-8000-000000000e20', 'Links Student', 'links student', '7654321')",
		"INSERT INTO people (id, display_name, normalized_name, staff_id) VALUES ('018f0000-0000-7000-8000-000000000e21', 'Links Advisor', 'links advisor', 'links-advisor')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000e22', 'category', 'links_category', '{\"en\":\"Links Category\"}', 1)",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000e23', 'platform', 'links_platform', '{\"en\":\"Links Platform\"}', 1)",
		"INSERT INTO projects (id, title, abstract, academic_year, semester, program_version_id, course_version_id) VALUES ('" + publishedID + "', 'Published Links Project', 'Complete.', 2026, 'first', '018f0000-0000-7000-8000-000000000e13', '018f0000-0000-7000-8000-000000000e15')",
		"INSERT INTO projects (id, title, abstract, academic_year, semester, program_version_id, course_version_id) VALUES ('" + draftID + "', 'Draft Links Project', 'Draft.', 2026, 'first', '018f0000-0000-7000-8000-000000000e13', '018f0000-0000-7000-8000-000000000e15')",
		"INSERT INTO project_participations (project_id, person_id, role, position) VALUES ('" + publishedID + "', '018f0000-0000-7000-8000-000000000e20', 'student', 0), ('" + publishedID + "', '018f0000-0000-7000-8000-000000000e21', 'advisor', 0)",
		"INSERT INTO project_taxonomy_values (project_id, taxonomy_value_id, dimension, position) VALUES ('" + publishedID + "', '018f0000-0000-7000-8000-000000000e22', 'category', 0), ('" + publishedID + "', '018f0000-0000-7000-8000-000000000e23', 'platform', 0)",
		"UPDATE projects SET status='published', published_at=now() WHERE id='" + publishedID + "'",
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed links fixture: %v", err)
		}
	}
	return uuid.MustParse(publishedID), uuid.MustParse(draftID)
}

func createLinksTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	adminConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	adminConfig.Database = "postgres"
	adminConnection, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	databaseName := "ause_links_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := adminConnection.Exec(ctx, "CREATE DATABASE "+databaseName); err != nil {
		adminConnection.Close(ctx)
		t.Fatalf("create temporary database: %v", err)
	}
	adminConnection.Close(ctx)

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse pool configuration: %v", err)
	}
	poolConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open temporary database: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		t.Fatalf("set migration dialect: %v", err)
	}
	sqlDatabase := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, sqlDatabase, "../../migrations"); err != nil {
		sqlDatabase.Close()
		pool.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	sqlDatabase.Close()

	t.Cleanup(func() {
		pool.Close()
		connection, connectErr := pgx.ConnectConfig(ctx, adminConfig)
		if connectErr != nil {
			return
		}
		defer connection.Close(ctx)
		_, _ = connection.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseName+" WITH (FORCE)")
	})
	return pool
}
