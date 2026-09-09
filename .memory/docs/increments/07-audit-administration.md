# Audit administration increment

## Assignment

Implement and commit the audit administration vertical for AUSE Discovery.

This is one bounded increment. Complete the audit query service, authenticated HTTP endpoint, administrator audit interface, focused tests, and durable status updates. Stop after a coherent commit and handoff. Do not continue into CI, operator documentation, broad product polish, final acceptance, deployment, or publication.

The accepted baseline is commit `722a6ff` (`Implemented persistent atomic Project imports`). Treat that commit as working software. Preserve all existing behavior unless this brief explicitly requires a change.

## Required reading before coding

Read the following files in order:

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/project_context.md`
4. `.memory/architectural_decision_logs.md`
5. `.memory/mvp_implementation_status.md`
6. `.memory/lessons_learned.md`
7. `CONTEXT.md`
8. `docs/ause-discovery_specification_v1.md`, especially sections 16 and 35
9. `docs/ause-discovery_implementation_specification_v1.md`, especially sections 6.7, 7, 11.4, 12, 15, 16.7, and 17
10. This brief

Then inspect only the implementation surfaces named below and nearby tests. Do not reread unrelated implementation packages.

## Repository starting state

The following audit foundations already exist:

- `audit_events` is defined in `backend/migrations/00001_initial_schema.sql`.
- Audit writes use `backend/internal/audit/service.go`.
- Existing identity, Person, Project, Artifact, search, and import mutations append audit events.
- `backend/queries/audit.sql` contains `CreateAuditEvent` and an unused offset-based `ListAuditEvents` query.
- `GET /admin/audit-events` already exists in `api/openapi.yaml` and generated clients, but no handwritten HTTP handler exists.
- `/admin/audit` exists in the navigation and router, but still renders `AdminPlaceholder`.
- The application currently has one administrator role. Session permission strings are descriptive capabilities, not a second authorization system.

Known contract gaps must be resolved as part of this increment:

- Database audit actors are nullable for bootstrap and failed-login events, while the current OpenAPI `actor_id` response property is required and non-nullable.
- The current SQL list query does not filter by event action even though the HTTP contract exposes `action`.
- The current SQL list query uses an offset even though the HTTP contract exposes an opaque cursor.
- Session responses do not yet include `audit.read`.

## Required outcome

An authenticated administrator can open `/admin/audit`, inspect newest-first audit events, filter by exact action and actor UUID, move through stable cursor pages, and distinguish actor-initiated events from system or unauthenticated events. Audit metadata is displayed safely as text. Invalid filters or cursors receive controlled validation responses. Audit records remain read-only.

## Scope

### Included

- Audit list domain/service behavior.
- Exact action and actor UUID filters already declared by OpenAPI.
- Stable keyset pagination ordered by `created_at DESC, id DESC`.
- Opaque cursor encoding and strict cursor validation.
- Nullable audit actor response handling.
- Authenticated `GET /admin/audit-events` transport implementation.
- `audit.read` in the single administrator session capability list.
- Administrator audit page with filters, states, metadata presentation, and cursor navigation.
- Generated sqlc and OpenAPI outputs when their source contracts change.
- Focused backend, HTTP, frontend, and one small live boundary check.
- Progress and durable-memory updates after verification.
- One coherent past-tense commit.

### Excluded

- Audit mutation, deletion, export, retention, or redaction endpoints.
- Enterprise audit integrations or external log shipping.
- New roles, permission tables, or policy engines.
- Free-text audit search.
- Date-range, target-type, or target-ID filters unless required to correct an implementation defect discovered during this increment.
- Changes to event names already emitted by existing services.
- CI workflows, release automation, operator runbooks, global localization cleanup, broad accessibility review, or final product acceptance.
- Deployment or external publication.

## Contract decisions

Treat `api/openapi.yaml` as the wire source of truth.

1. Keep the existing request parameters: `action`, `actor_id`, `cursor`, and `limit`.
2. Make response `actor_id` nullable because `admin.created` and `login.failure` legitimately have no authenticated actor. A required nullable field is acceptable and makes the absence explicit. Generated Go and TypeScript types must represent null safely.
3. Keep `resource_type` and `resource_id` aligned with stored `target_type` and `target_id`. Stored target type is always present. Target ID may be absent.
4. Require `metadata` in every audit response and return an object, including `{}` when no metadata was stored.
5. Add a documented `400` Problem Details response for invalid audit filters or cursors. Preserve `401` for missing or invalid sessions.
6. Keep the public response names `action`, `resource_type`, and `resource_id`. Map them from database fields `event_type`, `target_type`, and `target_id` in the HTTP boundary.
7. Do not expose credential hashes, session identifiers, tokens, CSRF values, abstracts, import row drafts, Artifact storage keys, file hashes, or raw request bodies.
8. Do not add CSRF requirements to this read-only endpoint.

If a generated type cannot express required nullable `actor_id` cleanly with the existing generator, use the smallest valid OpenAPI 3.1 schema adjustment. Do not hand-edit generated files.

## Backend implementation

### Audit query

Update `backend/queries/audit.sql` and regenerate sqlc output.

Replace offset pagination with a keyset query that:

- filters by exact `event_type` when an action is supplied;
- filters by exact `actor_id` when an actor is supplied;
- applies the cursor boundary `(created_at, id) < (cursor_created_at, cursor_id)` when a cursor is supplied;
- orders by `created_at DESC, id DESC`;
- accepts a bounded limit from the service;
- fetches one extra row so the service can determine whether a next cursor exists.

The query must not expose a generic update or delete operation for `audit_events`. Existing write behavior remains append-only through the application surface.

Regenerate `backend/generated/audit.sql.go` with the repository command. Generated output must never be edited manually.

### Audit service

Extend `backend/internal/audit/service.go` or add small files in the same package when that improves separation.

The service boundary must:

- accept optional action, optional actor UUID, optional opaque cursor, and limit;
- normalize the limit through one clear rule: default 20, minimum 1, maximum 100;
- decode and validate the cursor before querying;
- return a typed page containing events, effective limit, and optional next cursor;
- preserve nullable actor and target IDs;
- decode metadata into an object and fail safely if persisted data cannot be decoded;
- fetch `limit + 1`, return at most `limit`, and derive the next cursor from the final returned event;
- use `created_at` plus UUID as the complete cursor position;
- reject malformed, incomplete, oversized, or trailing-data cursor payloads with a package-level validation error;
- avoid placing raw cursor contents or metadata in logs or error messages.

An opaque base64url-encoded versioned JSON cursor is sufficient. Cursor signing is not required because the endpoint is administrator-only and the decoded fields are strictly validated. Keep cursor helpers private unless tests benefit from package-level access.

Minor model names, file decomposition, and helper names are implementation choices.

### HTTP handler

Update `backend/internal/platform/httpserver/api.go`.

- Add an audit read service to `Controller` and initialize it in `NewAPIHandler`.
- Implement the generated `ListAuditEvents` method.
- Require a valid administrator session through the existing read-only actor path.
- Map OpenAPI parameters to the audit service without introducing duplicate pagination logic.
- Return `400` Problem Details for service validation errors.
- Return `500` Problem Details for unexpected storage or decoding failures without exposing internal details.
- Return an `AuditEventPage` with a non-null empty `items` array when no records match.
- Map UUID and timestamp values using existing identity conversion conventions.
- Return nullable actor and resource IDs correctly.
- Include the service-provided effective limit and next cursor in `PageInfo`.
- Add `audit.read` to both login and current-session permission arrays. Keep both arrays identical.

Avoid unrelated refactoring of the large controller file. A small shared permission constant is acceptable only if it removes the existing duplication cleanly.

## Frontend implementation

Create a focused feature module such as:

- `frontend/src/features/admin/audit-log.tsx`
- `frontend/src/features/admin/audit-log.test.tsx`

Replace the `/admin/audit` placeholder in `frontend/src/app-shell.tsx` with the real feature component.

The page must provide:

- a visible page heading;
- an exact action filter;
- an actor UUID filter;
- explicit Apply and Clear controls;
- newest-first event rows;
- event timestamp, action, actor, resource type, resource ID, and metadata;
- a clear `System or unauthenticated` label when `actor_id` is null;
- loading, empty, request-error, and invalid-filter feedback;
- Next navigation when `next_cursor` exists;
- Previous navigation by retaining the cursor history in component state;
- filter changes that reset pagination to the first page;
- disabled pagination controls while a request is pending;
- safe text rendering only, with no `dangerouslySetInnerHTML` and no HTML interpretation of metadata values.

Metadata may use a compact definition list or equivalent semantic structure. Primitive values should be readable. Nested arrays or objects may use bounded JSON text, but rendering must remain safe and must not create an unbounded expanding tree.

Use the generated `listAuditEvents` client and TanStack Query conventions already present in the repository. Do not duplicate the application API client setup. Keep audit filter state local unless URL state becomes substantially simpler during implementation. URL persistence is not required for this increment.

Add only the translations and CSS needed by this page. Preserve the compact formatting of existing shared files and avoid running Prettier across `app-shell.tsx`, `i18n.ts`, or `App.module.css`. Format the new feature files directly, then inspect the shared-file diff.

## Validation and error behavior

- Omitted action means no action filter.
- Supplied action is an exact match, not a substring search.
- Omitted actor UUID means no actor filter.
- An invalid actor UUID is rejected by generated HTTP parameter binding as `400`.
- A malformed cursor returns `400` with a stable problem code such as `validation_error`.
- A valid cursor that has no remaining rows returns `200` with an empty array and no next cursor.
- An empty result is not an error.
- Audit storage failures never return raw SQL, cursor payloads, or metadata.
- Frontend error messages remain general and accessible.

## Focused verification

Follow `AGENTS.md`. Do not run the full repository suite during this increment unless a focused failure reveals cross-system risk.

### Backend service tests

Add focused tests covering:

1. Newest-first ordering with a deterministic UUID tie-breaker.
2. Exact action filtering.
3. Exact actor filtering.
4. Nullable actors and nullable target IDs.
5. `limit + 1` next-cursor behavior across at least two pages with no duplicates.
6. Malformed cursor rejection without a database query when practical.
7. Metadata object decoding.

Use one PostgreSQL integration test for query and pagination behavior. Reuse existing database-test patterns. Do not add a migration solely for test data.

### HTTP tests

Add focused coverage for:

- audit cursor validation mapping to `400`;
- unauthenticated access mapping to `401`;
- successful response mapping, especially null actor and empty metadata;
- `audit.read` appearing consistently in login and session responses if an existing authentication transport test can cover this cheaply.

Avoid building an extensive custom HTTP harness when the existing controller and integration patterns can cover the same boundary with less code.

### Frontend component test

Add one focused interaction test that proves:

- events render with action, actor or system label, resource, and metadata;
- Apply sends exact action and actor parameters;
- Next uses the returned cursor;
- Previous returns to the earlier cursor;
- Clear removes filters and resets pagination;
- an API failure produces an accessible alert.

Multiple assertions may live in one or two coherent tests. Do not create snapshot tests.

### Commands

Prefer repository scripts and pinned toolchains. The host may not have Go, and the host Node.js version may be below the repository baseline.

Run only the checks affected by this increment:

1. sqlc generation after changing `backend/queries/audit.sql`.
2. OpenAPI Go and TypeScript generation after changing `api/openapi.yaml`.
3. `gofmt` on changed Go files.
4. Go tests for `backend/internal/audit` and `backend/internal/platform/httpserver`.
5. The new frontend audit component test.
6. Frontend `tsc -b`.
7. One authenticated HTTP smoke path against the local Compose dependencies.

The HTTP smoke path should create or reuse a disposable administrator, perform one ordinary mutation that emits an audit event, then verify:

- the unfiltered endpoint returns the event;
- the action filter returns the expected event;
- a bad cursor returns `400`;
- a request without the session returns `401`.

Use disposable names and data. Remove only resources created by this increment. Preserve active Compose PostgreSQL and Meilisearch services, networks, and volumes. Do not run broad Docker prune commands.

If Docker or Git requires approval, request approval directly with the exact command and continue after approval. Do not use an automatic review workflow.

## Completion criteria

The increment is complete only when all conditions below hold:

- `/admin/audit` is no longer a placeholder.
- Authenticated audit listing works through the real API.
- Action and actor filters behave exactly.
- Cursor pagination is stable and rejects malformed input.
- Null actor events render clearly.
- Metadata rendering cannot execute markup.
- Generated code matches source contracts.
- Focused backend, HTTP, and frontend checks pass.
- The targeted authenticated HTTP smoke path passes.
- `.memory/mvp_implementation_status.md` records verified audit behavior and evidence.
- `.memory/MEMORY.md` advances the checkpoint only after verification.
- `.memory/lessons_learned.md` changes only when a durable reusable lesson was actually discovered.
- No unrelated formatting or cleanup is included.
- `.DS_Store` remains untracked and unstaged unless separately requested.
- One coherent commit exists with a past-tense message, for example `Implemented administrator audit visibility`.

## Required handoff for review

Stop after the commit. Do not begin CI or operator documentation.

The completion report must include:

- final commit hash and exact commit message;
- concise behavior summary;
- source files changed by area;
- contract changes and generation commands used;
- focused test commands and outcomes;
- HTTP smoke steps and outcome;
- any unresolved defect, assumption, skipped check, or environmental limitation;
- final `git status --short` output, including unrelated pre-existing files;
- confirmation that disposable Docker resources were removed and persistent development resources were preserved.

The reviewing task will inspect the full diff from `722a6ff` through the submitted commit, verify test claims, and either accept the increment or return a bounded correction list. Do not rewrite earlier accepted commits.
