# MVP implementation status

## Technical completion

Status as of 2026-09-04: foundation, data contracts, identity and core records, Artifacts, search, imports, audit administration, CI with operator readiness, and product coherence are implemented; product coherence is submitted for independent review, and final product-completion verification follows acceptance.

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
- `/admin/audit` renders the real audit interface with exact filters, actor UUID normalization, explicit Apply, Clear, and failure Retry controls, system-actor labeling, text-only metadata in a compact definition list, cursor Next and Previous navigation, and loading, empty, invalid-filter, and request-error states.
- Audit storage remains append-only: the SQL surface exposes no update or delete path for `audit_events`.
- Focused verification: audit service unit tests, one PostgreSQL integration test for ordering, filters, nullable values, limit behavior, and two-page cursor pagination without duplicates; an HTTP boundary integration test covering 401, 400 cursor and parameter validation, successful null-actor and metadata mapping, and `audit.read`; the frontend audit interaction test; `tsc -b`; and an authenticated Compose smoke path verifying the unfiltered listing, exact action filter, cursor paging, bad-cursor `400`, missing-session `401`, and invalid actor `400` through a disposable administrator and disposable database.

### CI and operations

- `validate.yml` runs for pull requests and for default-branch pushes routed through the event context (`github.ref_name == github.event.repository.default_branch` on each job, so no branch name is hardcoded and tag pushes are excluded), covering frontend source checks (Node 24.20.0, pnpm 11.25.0, action-pin validation, lint, component tests, default and root base-path builds), backend checks (Go 1.27.0 container, `go mod verify`, tracked-file `gofmt`, `go vet`, full tests with a digest-pinned PostgreSQL 18.6 service and a non-optional `AUSE_TEST_DATABASE_URL`), generated-code drift for sqlc and both OpenAPI outputs, API and web image builds with root web variant using GitHub cache, and vulnerability reporting: gating `govulncheck` v1.7.0 plus advisory `pnpm audit` and Trivy image scans with explicit `continue-on-error` rationale.
- `release-images.yml` triggers only on `v*` tags or manual dispatch defaulting to no push, grants `packages: write` only to the publication job, derives GHCR names from `github.repository`, publishes version and commit-SHA tags without `latest`, applies OCI `source`, `revision`, `version`, and image-specific `title` labels from runtime context, emits digests into the workflow summary, and contains no deployment step.
- Every external action is pinned to a full commit SHA with a release comment; `scripts/check-action-pins.sh` enforces this for tracked workflows and a fixture test proves acceptance and rejection behavior.
- `compose.yaml` consumes `AUSE_API_IMAGE` and `AUSE_WEB_IMAGE` overrides, accepts complete digest references while keeping local defaults and builds, parameterizes the web host port, and adds bounded log rotation defaults; `.env.example` documents production-required values and image digest consumption with `docker compose pull` and `up -d --no-build`.
- Local CI parity targets (`check-source`, `check-integration`, `generate-check`, `check-images`, `check-compose`, `vuln-report`, `check-action-pins`, `check-action-pins-test`) cover the locally reproducible CI boundaries; `vuln-report` covers govulncheck and the pnpm audit while the Trivy image scans run in CI only. `generate-openapi-typescript` invokes the pure-JavaScript generator shim in the pinned Node container instead of containerized pnpm.
- `docs/operations/runbook.md` covers scope invariants, host prerequisites, configuration, ordered bootstrap, routine operations, health, backup targets, restore ordering, upgrade and rollback constraints, recovery scenarios, shutdown, and the manual deployment boundary using repository service names and `ausectl` commands.
- Focused verification passed: `git diff --check`; actionlint v1.7.12; repository and fixture action-pin checks; Compose configuration rendering with local defaults and with production-like dummy values including digest-shaped API and web references; frontend lint, component tests, and both production builds; Go module verification, tracked-file formatting, vet, unit tests, and PostgreSQL-backed integration tests; generation drift check; API and web image builds including the root base-path variant; govulncheck (zero reachable findings), pnpm audit, and Trivy scans of both built images.
- Known advisory findings, visible rather than suppressed: pnpm audit reports two high js-yaml advisories reachable only through the pinned OpenAPI generator toolchain, and Trivy reports upstream base-image findings including a critical golang.org/x/crypto advisory fixed in 0.55.0 that govulncheck confirms is not reachable from application code. Dependency upgrades are deliberately outside this increment.

### Product coherence

- One shared application frame renders the site header, primary and footer navigation, a single focusable `<main id="main-content">` landmark, and a `Skip to main content` link for public, login, administrator, and Not Found routes; route changes move focus to the main region once per path change without stealing focus during refetches.
- Every named route sets a specific document title, administrator secondary navigation stays inside the main region, the unexpected-error boundary renders a minimal recovery page, and About, Privacy, Accessibility, Terms, and a new Contact route present factual content with visible pending-approval notices where institutional content is required.
- A generated-client error interceptor installed exactly once clears cached administrator session state on any API `401`, so `AdminGuard` returns to `/admin/login` with the full intended path in `next`; failed logins keep the generic credential message and public `401` responses cause no redirect loop.
- Search uses a draft query field applied on submit, distinguishes first load from background refresh, reports catalog loading or failure states, renames paging to `Next page`/`Previous page` with local cursor history that resets on filter changes, disables paging during requests, and renders API abstract highlights as text-only excerpts.
- Public Project and Person pages translate participation roles, render Reference Code and Student ID only when present, distinguish Not Found from request failure through the problem code, and open PDF View links in a new browsing context with `noopener noreferrer`; the same behavior applies to administrator Artifact View links.
- Administrator Project and People lists gained query and status filters with explicit Apply and Clear, shared cursor-history paging, and loading, empty, and request-failure states without false empties; the backend now implements the declared list contract: Projects honor `q` (trimmed, case-insensitive title or reference code), `status`, `cursor`, and `limit`, People honor `q`, `cursor`, and `limit`, both order deterministically with a stable tie-break, fetch limit plus one, emit `next_cursor` only when another page exists, and reject malformed cursors with `400 validation_error` through shared opaque `pagecursor` payloads bounded at 2048 characters, a size derived from the worst contract-legal sort key so maximum-length Unicode Person names page correctly; Person creation and editing validate locally, disable while pending or without CSRF, show saved status, and preserve entered values across a revision conflict while an explicit reload awaits the actual server response through a separate cache entry, adopts its fields and revision only on success, disables submission while reloading, and preserves entered values with a retry notice when the reload itself fails; the Project form blocks behind Catalog and People loading with an explicit dependency retry, reuses the revision returned by a successful edit for the next save, publish, and lifecycle operation, and lifecycle revision conflicts offer explicit reload without discarding unsaved form input, with the confirmation dialog closing on failure and restoring focus to its trigger so recovery controls are reachable; Artifact operations and sign out gained pending and failure feedback without discarding sessions or input.
- The delete confirmation traps keyboard focus among enabled controls only, including the single-enabled-control pending case, focuses the confirmation input initially, closes on Escape through cancel, restores the triggering control, blocks background pointer activation, disables actions while pending, and ignores Escape and backdrop dismissal while the deletion request is pending; its controls are explicitly `type="button"` so they can never submit an enclosing form.
- Vite and React starter assets were removed after tracked-reference checks, and the favicon is a repository-owned neutral mark; the focus ring moved to a semantic token, disabled controls gained non-color state, and long identifiers and titles wrap without page-level horizontal overflow.
- Nginx serves all static and proxied responses with `nosniff`, `same-origin` referrer, permissions policy, and a restrictive CSP through an included snippet repeated in caching locations so `add_header` inheritance cannot drop it; the entry document stays `no-cache`, assets stay immutable, redirects are port-agnostic, and no HSTS is emitted.
- The web container runs the pinned Nginx image entirely as the unprivileged `nginx` account on internal port `8080` with narrowly granted runtime directory ownership; Compose maps the published web port to the new internal port.
- Focused verification passed: `git diff --check`; `tsc -b`; ESLint; all component test files (21 files, 55 tests after corrections) including the new shell, session-expiry, search paging and history synchronization, cursor hook, Project and People lists, Project form dependency and lifecycle boundaries, Person form conflict reload, public presentation, and delete-confirmation boundaries; both production base-path builds; both web images built without publication; Compose config rendering with the new internal port; and a disposable container smoke confirming the `nginx` process user, `nginx -t`, the 8080 listener, entry and nested SPA delivery, immutable asset caching, security headers on home, nested, and asset responses with no HSTS, for both default and root base-path images.
- Correction verification passed: backend package tests for `pagecursor`, People, Projects, and the HTTP server covering q/status filtering, two-page cursor pagination without duplicates, deterministic ties, final-page behavior, malformed-cursor `400` responses, and maximum-length Unicode names producing usable in-bounds cursors; `gofmt` clean; generation drift clean after the cursor-bound contract adjustment; and an authenticated HTTP smoke against a disposable administrator and database proving Projects and People `next_cursor` round trips at small limits, uppercase `q` filtering, and malformed-cursor rejection on both lists. A second correction round verified the awaited Person reload (deferred response with renamed fields and a newer revision, next save using both), reload-failure input preservation, dialog-closed lifecycle recovery with focus restoration, enabled-only focus trapping in both multi- and single-control cases, and the Unicode cursor bound, through focused frontend regression tests with the full suite (21 files, 58 tests), TypeScript, and lint passing.

## Active implementation work

- Product coherence with its first correction is submitted for review; final product-completion verification follows acceptance.

## Known implementation constraints

- The host Node.js version is below the exact repository baseline; the pinned container toolchain provides the reproducible path.
- Go is absent on the host; Go verification uses the pinned container toolchain.
- `typescript-eslint` 8.69.0 rejects TypeScript 7.0.2. The current lint gate runs ESLint on compatible JavaScript configuration and uses `tsc -b` for TypeScript static checking while preserving the accepted exact pins.
- `@hey-api/openapi-ts` 0.99.0 rejects TypeScript 7.0.2. Reproducible client generation uses the isolated `api/generator` workspace with TypeScript 5.9.3 while application compilation remains on TypeScript 7.0.2.

## Launch-only inputs

GitHub ownership, registry ownership, production URL, VM details, branding, approved legal content, privacy approval, TLS ownership, and final institutional catalog content remain launch-readiness inputs rather than technical MVP blockers.
