# MVP implementation status

## Technical completion

Status as of 2026-09-04: foundation, data contracts, identity and core records, Artifacts, search, imports, and audit administration complete; product completion work is next.

## Verified increments

### Foundation

- Exact pnpm workspace and dependency pins are tracked.
- Neutral Go module, typed configuration, normalized public base paths, structured logging, health endpoints, graceful shutdown, and non-root container runtime are implemented.
- Root Makefile, environment example, Docker ignore rules, Compose topology, pinned service images, and Nginx SPA routing are implemented.
- Frontend production builds work for `/` and `/ause-discovery/`.
- The production-like Compose stack builds and starts with healthy PostgreSQL, Meilisearch, and API services.
- Default-base-path liveness, readiness, redirect, SPA delivery, and nested-route fallback are verified through Nginx.
- Frontend TypeScript checking, JavaScript configuration linting, seven base-path tests, and the production build pass.

### Data and contracts

- OpenAPI 3.1 defines the stable public, authentication, administrator, Artifact, import, search, audit, and health endpoints.
- Generated Go transport types and Chi server interfaces are isolated from generated sqlc models; generated TypeScript client and wire types compile.
- sqlc queries, generated pgx code, transaction handling, UUIDv7 generation, and one forward Goose migration implement the accepted persistence boundaries.
- Fresh PostgreSQL migration, repeat migration, representative constraints, open-ended academic versions, single-active-rebuild enforcement, and transaction rollback pass integration tests.
- Version-controlled academic catalog and taxonomy validation plus non-destructive synchronization are implemented through `ausectl` and pass repeat-sync integration tests.
- `ausectl migrations status`, `catalog validate`, and `catalog sync` pass in the pinned backend image.
- Generated sqlc, Go OpenAPI, and TypeScript OpenAPI output is deterministic across repeated generation.
- A fresh isolated Compose project builds, migrates, starts healthy, synchronizes catalogs twice, and serves health and nested SPA routes under `/ause-discovery/`.

### Identity and core records

- Argon2id local credentials, opaque hashed sessions, independent CSRF tokens, idle and absolute expiry, session rotation, revocation, and database-backed login throttling are implemented.
- Operator CLI commands create, reset, and disable administrator accounts without accepting passwords through flags or environment variables.
- Append-only audit events cover identity, Person, and Project mutations, including bootstrap operations without an existing actor.
- Person create, update, get, and list behavior enforces Student ID and optimistic revision rules.
- Project aggregate create, replace, publish, delete, and restore behavior maintains ordered aliases, participation, taxonomy, revision, audit, and pending search state transactionally.
- Public Project and Person projections remain PostgreSQL-backed and enforce published visibility.
- HTTP transport enforces strict JSON, Problem Details, request IDs, security headers, authenticated mutation CSRF, same-origin checks, and trusted proxy boundaries.
- Administrator Project responses include complete academic, participation, taxonomy, and Artifact state for safe form round trips.
- The frontend provides search-first public routes, public Project and Person details, protected administration, Person maintenance, complete Project aggregate controls, revision-conflict preservation, and Project-specific delete confirmation.
- Backend tests pass against isolated temporary PostgreSQL databases. Frontend static checks, 20 component tests, root and subpath production builds, and the production-like Compose image build pass.
- Authenticated HTTP smoke verification covers administrator creation and login, Person creation, Project draft creation, publication, public Project and Person reads, deletion visibility, restoration, and logout.

### Artifacts

- Opaque versioned filesystem keys, path containment, symlink rejection, streamed SHA-256 hashing, atomic finalization, per-file limits, and compensating cleanup protect Artifact bytes.
- Artifact type and extension allowlists reject executable, HTML, SVG, script, disk-image, and macro-enabled Office uploads. Bounded server-side content detection rejects unsafe or mismatched file content.
- Upload quota checks serialize through the Project row and maintain Project revisions, pending search state, and append-only audit records in the metadata transaction.
- Metadata update, reversible deletion, quota-checked restoration, and immutable replacement preserve optimistic concurrency and existing bytes.
- Public serving requires an active Artifact on a published Project. PDF inline view, safe UTF-8 download filenames, byte ranges, missing-content handling, and no-sniff responses are implemented.
- Administrator Project editing includes accessible Artifact upload, metadata update, replacement, delete, and restore controls. Public Project pages expose PDF view and allowed Artifact download actions.
- Backend unit and isolated PostgreSQL integration tests cover storage safety, size and type validation, lifecycle revisions, quota, search state, audit records, immutable replacement, cleanup, and public visibility.
- Frontend static checks and 22 component tests pass. Authenticated HTTP smoke verification covers upload, partial PDF view with a `206` response, metadata update, replacement, public deletion visibility, restoration, and full administrator aggregate retrieval.

### Search

- PostgreSQL remains canonical while Meilisearch stores derived, rebuildable Project documents with a stable logical index and versioned physical rebuild indexes.
- Published Project projections include aliases, academic references, ordered People roles, taxonomy, active Artifact availability, revisions, and sort fields while excluding private metadata and storage identities.
- Query validation, exact seven-digit identifier precedence, deterministic cursor binding, OR-within-facet and AND-across-facet filtering, sorting, facet distribution, and bounded highlights are implemented.
- The reconciliation worker uses leases, bounded retries, changed-revision protection, expired-lease recovery, and removal of no-longer-public Projects.
- Full rebuilds configure a replacement index, populate stable pages, reconcile final published state under serialization, verify document counts, swap indexes, preserve the prior physical index, and record operation status and audit events.
- Public search, authenticated status, Project reindex, and rebuild HTTP endpoints are implemented. The operator CLI supports Project reindex and full rebuild commands. Administration includes search availability, queue counts, Project reindex, rebuild progress, and failure display.
- Focused backend tests, the full backend suite, all 23 frontend tests, TypeScript checking, and the production frontend build pass.
- Local PostgreSQL and Meilisearch smoke verification covers queued reconciliation to `synced`, filtered and alias search, exact Student ID search, complete rebuild, verified swap, and authenticated status with no pending or failed rows.

### Imports

- Strict CSV and XLSX adapters enforce exact headers, supported workbook structure, formula and macro rejection, bounded files, Student ID text preservation, and canonical Project import drafts.
- Persistent PostgreSQL previews retain row validity, warnings, duplicate candidates, selections, acknowledgements, and duplicate resolutions across process restarts.
- Commit revalidates selected rows against current catalogs, creates People and Projects in one transaction, publishes complete Projects, records search reconciliation and audit state, and returns an immutable result on repeated requests.
- Expired batches scrub temporary source files and persisted draft payloads while retaining bounded operational metadata.
- HTTP endpoints provide upload, paginated preview, row-decision updates, atomic commit, stable results, cursor validation, Problem Details, authentication, CSRF protection, and the `import.execute` session capability.
- Administration provides template downloads, client-side file checks, persistent review pagination, warning acknowledgement, duplicate resolution, selection saving, guarded commit, and stable result display.
- Bundled CSV and XLSX templates use shipped catalog keys and pass the production import adapters.
- Focused adapter and PostgreSQL integration tests cover valid and rejected files, restart persistence, correction state, all-or-nothing rollback, repeated commit, search state, and expiry cleanup. Focused HTTP tests, the import interaction test, and TypeScript checking pass.

### Audit administration

- `GET /admin/audit-events` requires an administrator session, filters by exact action and actor UUID, and pages with a stable keyset cursor ordered by `created_at DESC, id DESC`; malformed cursors return `400 validation_error` and missing sessions return `401`.
- The audit response exposes nullable `actor_id` for bootstrap and failed-login events, always-present `resource_type`, nullable `resource_id`, and a required metadata object, with `event_type`, `target_type`, and `target_id` mapped at the HTTP boundary.
- Session and login responses include `audit.read` in the single administrator permission list.
- `/admin/audit` renders the real audit interface with exact filters, explicit Apply and Clear controls, system-actor labeling, text-only metadata in a compact definition list, cursor Next and Previous navigation, and loading, empty, invalid-filter, and request-error states.
- Audit storage remains append-only: the SQL surface exposes no update or delete path for `audit_events`.
- Focused verification: audit service unit tests, one PostgreSQL integration test for ordering, filters, nullable values, limit behavior, and two-page cursor pagination without duplicates; an HTTP boundary integration test covering 401, 400 cursor and parameter validation, successful null-actor and metadata mapping, and `audit.read`; the frontend audit interaction test; `tsc -b`; and an authenticated Compose smoke path verifying the unfiltered listing, exact action filter, cursor paging, bad-cursor `400`, missing-session `401`, and invalid actor `400` through a disposable administrator and disposable database.

## Active implementation work

- CI, operator documentation, and product-completion verification.

## Known implementation constraints

- The host Node.js version is below the exact repository baseline; the pinned container toolchain provides the reproducible path.
- Go is absent on the host; Go verification uses the pinned container toolchain.
- `typescript-eslint` 8.69.0 rejects TypeScript 7.0.2. The current lint gate runs ESLint on compatible JavaScript configuration and uses `tsc -b` for TypeScript static checking while preserving the accepted exact pins.
- `@hey-api/openapi-ts` 0.99.0 rejects TypeScript 7.0.2. Reproducible client generation uses the isolated `api/generator` workspace with TypeScript 5.9.3 while application compilation remains on TypeScript 7.0.2.

## Launch-only inputs

GitHub ownership, registry ownership, production URL, VM details, branding, approved legal content, privacy approval, TLS ownership, and final institutional catalog content remain launch-readiness inputs rather than technical MVP blockers.
