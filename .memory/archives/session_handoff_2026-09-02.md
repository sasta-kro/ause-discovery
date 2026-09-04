# Session handoff, 2026-09-02

## Continuation constraint

Keep all repository work in one primary session and proceed one task at a time. Optimize for completed vertical features and quota efficiency. Use focused tests during each increment, then one broader integration pass after the increment is coherent.

No external deployment or publication is authorized.

## Completion estimate

- Fully integrated, verified, and committed: approximately 30 to 35 percent of the technical MVP.
- Implemented code including the interrupted working tree: approximately 45 percent.
- Route or screen presence is higher because Artifact, import, search-maintenance, and audit administration pages are placeholders.

The project is not close to technical MVP completion. Artifacts, Meilisearch integration, imports, and final cross-system completion remain substantial.

## Git checkpoints

Current branch: `main`.

Current committed baseline:

1. `4993004 Established reproducible AUSE Discovery foundation`
2. `7996442 Completed AUSE Discovery data and contract foundation`

Commit messages should use a past-tense sentence without conventional prefixes. Example: `Implemented authenticated Project and People management`.

The working tree after `7996442` contains interrupted identity, People, Project, HTTP, CLI, and frontend work. Do not reset, checkout, clean, or otherwise discard it. `.DS_Store` is unrelated pre-existing untracked content and should remain excluded from commits.

## Verified committed baseline

### Foundation

- Exact pnpm workspace, Node, Go, service, image, and dependency pins.
- Go API skeleton with typed configuration, structured logging, health endpoints, graceful shutdown, normalized base paths, and non-root container execution.
- Docker Compose topology for PostgreSQL, Meilisearch, API, and Nginx web service.
- Nginx SPA routing and API proxy behavior for both `/` and `/ause-discovery/`.
- Frontend build, test, base-path, and lint infrastructure.
- Fresh production-like Compose startup and health checks.

### Data and contracts

- OpenAPI 3.1 contract for public, authentication, administrator, Artifact, import, search, audit, and health endpoints.
- Generated Go transport types and Chi interfaces plus generated TypeScript client and wire types.
- PostgreSQL schema and forward Goose migration for catalogs, taxonomy, users, credentials, sessions, People, Projects, Artifacts, search synchronization, imports, and audit.
- sqlc configuration, queries, and generated pgx code.
- Version-controlled academic catalog and taxonomy YAML with validation and non-destructive synchronization.
- `ausectl migrations status`, `catalog validate`, and `catalog sync`.
- Fresh migration, repeat migration, representative constraint checks, catalog repeat-sync, and transaction rollback checks.
- Deterministic sqlc, Go OpenAPI, and TypeScript OpenAPI regeneration.

The generated-output hash recorded after the committed generation pass was `ee4591021308f0444129d32faa962cc11495ab2022347d11677497a1d1dd7274`.

## Interrupted working tree

### Backend work present but not committed

Tracked changes:

- `backend/cmd/api/main.go`
- `backend/cmd/ausectl/main.go`
- `backend/generated/projects.sql.go`
- `backend/go.mod`
- `backend/go.sum`
- `backend/queries/projects.sql`

New source areas:

- `backend/internal/audit/`
- `backend/internal/auth/`
- `backend/internal/people/`
- `backend/internal/projects/`
- `backend/internal/platform/httpserver/api.go`
- `backend/internal/platform/identity/pgx.go`

Implemented behavior includes:

- Argon2id local passwords and 12-character minimum validation.
- Opaque hashed session and independent CSRF tokens.
- Idle and absolute session expiry, password-reset and account-disable revocation.
- Database-backed login failure throttling.
- Administrator CLI create, password-reset, and disable operations.
- Append-only audit service calls for identity and record mutations.
- Person create, update, get, and list behavior with revision checks.
- Project aggregate create, replace, publish, delete, restore, child replacement, publication validation, and revision checks.
- Public Project and Person projection assembly with catalog, taxonomy, participation, and Artifact metadata reads.
- Generated Chi handler wiring, strict JSON decoding, Problem Details helpers, request IDs, security headers, session checks, CSRF checks, same-origin checks, and trusted-proxy address handling.

An earlier verification run reported that pinned-container `go test ./...` passed before the final hardening follow-up. The final follow-up added projection and proxy code but was interrupted by the account usage limit before its isolated PostgreSQL integration run. Treat the current backend as unverified work in progress.

Priority review points before committing:

1. Run formatting and focused compile/tests first. Repair only actual failures.
2. Review `backend/internal/platform/httpserver/api.go` carefully. It is a large transport file and contains direct SQL projection reads that may deserve focused query extraction, but avoid a broad refactor before behavior works.
3. Validate trusted proxy and origin logic with focused unit cases. The current `trustedOrigin` implementation derives a remote address through `clientIP` using an empty configuration and should be checked for forwarded-protocol correctness.
4. Confirm every implemented OpenAPI handler returns the specified shapes and stable Problem Details codes.
5. Confirm published Project completeness, public visibility, revision conflict, delete confirmation, login throttle, session expiry, rotation, revocation, and audit behavior against PostgreSQL.
6. Run one isolated PostgreSQL integration pass for this increment. Do not repeat the earlier full foundation and generation matrix unless contract or schema files change.

### Frontend work present but not committed

Tracked changes:

- deleted `frontend/src/App.css`
- modified `frontend/src/App.tsx`
- modified `frontend/src/index.css`
- modified `frontend/tsconfig.app.json`

New source areas:

- `frontend/src/App.module.css`
- `frontend/src/app-shell.tsx`
- `frontend/src/app-shell.test.ts`
- `frontend/src/app/i18n.ts`
- `frontend/src/app/runtime.ts`
- `frontend/src/app/runtime.test.ts`
- `frontend/src/features/admin/`
- `frontend/src/features/search/`

Implemented behavior includes:

- Search-first public home.
- URL-driven search state, facets, result highlighting, and pagination control.
- Public Project and Person detail routes.
- Generated-client base URL and cookie configuration for root and subpath deployment.
- Session and CSRF loading, login, logout, protected routes, and preserved post-login destination.
- Administrator shell and Project and Person list/create/edit flows.
- Project draft save, publish, delete, restore, server issue display, and revision-conflict preservation.
- English i18n resources, CSS Modules, design tokens, responsive layout, focus styles, and reduced-motion handling.
- Focused tests for runtime paths, route protection, search state, highlighting, form conversion, and delete confirmation.

An earlier verification run reported frontend lint success, 17 passing tests, and successful production builds for `/` and `/ause-discovery/`. A follow-up intended to implement Project participation and taxonomy controls was interrupted by the account usage limit. Those controls are still placeholders. Artifact, import, search-maintenance, and audit administrator routes are also placeholders.

Priority review points before committing:

1. Run the frontend type/lint gate and focused tests once after backend contract behavior is confirmed.
2. Inspect generated-client error handling and ensure Problem Details reaches the form handlers as expected.
3. Finish Project participation and taxonomy assignment controls using the existing aggregate request types.
4. Keep Artifact, import, search-maintenance, and audit placeholders until their corresponding backend verticals are implemented.
5. Avoid unrelated design refinement before all MVP flows work.

## Environment and verification facts

- Go is absent on the host. Use the pinned Go Docker image or repository commands.
- The host Node version is below the repository baseline. Use the pinned Node container.
- Host `make` is unavailable because Xcode command-line tools are absent.
- `typescript-eslint` 8.69.0 does not support TypeScript 7.0.2. The current lint gate checks compatible JavaScript configuration and uses `tsc -b` for TypeScript.
- `@hey-api/openapi-ts` 0.99.0 does not support TypeScript 7.0.2. Client generation uses the isolated `api/generator` workspace with TypeScript 5.9.3.
- A fresh isolated Compose project named `ause-discovery-verification` was used on port 8088 and the network `ause-discovery-verification_default`.
- Original development volumes were preserved after a stale pre-checkpoint migration state was discovered.
- The verification Compose project was last known to be running. Its current state could not be rechecked from the sandbox during handoff. Do not delete volumes merely to resume development.

## Recommended single-agent sequence

### 1. Stabilize and commit the interrupted core increment

- Review the current diff rather than rereading the whole repository.
- Format and compile the backend.
- Fix the current identity, HTTP, People, and Project issues.
- Complete Project participation and taxonomy frontend controls.
- Run focused backend unit tests, one PostgreSQL integration pass, frontend lint/tests, and root plus subpath builds.
- Smoke test login, Person CRUD, Project draft, Project publish, public Project, delete, and restore.
- Update `.memory/mvp_implementation_status.md`.
- Commit the coherent increment.

### 2. Artifact vertical

- Add Artifact queries and service boundaries.
- Implement safe local storage, streamed upload, MIME and extension allowlist, size and Project quota, hashing, atomic finalization, and cleanup compensation.
- Implement replace, delete, restore, secure public view/download, byte ranges, and missing-content isolation.
- Replace the frontend Artifact placeholder with administration and public actions.
- Verify focused storage security, quota, range, and cleanup cases, then one HTTP smoke path.
- Commit the vertical.

### 3. Search vertical

- Implement PostgreSQL Project projection, pending reconciliation, Meilisearch settings, public search, facet validation, reindex, and full rebuild state.
- Wire the existing public search UI and administrator maintenance route.
- Verify exact Student ID behavior, filters, update/delete propagation, outage behavior, and one rebuild.
- Commit the vertical.

### 4. Import vertical

- Implement bounded CSV and XLSX adapters, normalization, validation, issues, persistent preview, selection, warning acknowledgement, duplicate resolution, atomic commit, idempotent result, and expiry cleanup.
- Replace import placeholders with preview and commit flows.
- Verify leading-zero Student IDs, limits, formula rejection, restart persistence, atomic rollback, and repeated commit.
- Commit the vertical.

### 5. Product completion

- Finish audit UI/API and remaining administrator coherence.
- Add CI and operator runbook without hardcoded ownership.
- Run one complete Compose acceptance pass, root and subpath checks, restart persistence, essential security cases, and representative accessibility checks.
- Do not deploy or publish externally.

## Verification policy for continuation

Use proportional verification:

- During implementation: compile changed packages and run focused tests for changed behavior.
- At each vertical boundary: run that vertical's integration test and a small HTTP or browser smoke path.
- After all verticals: run the broader generation, Compose, security, accessibility, and acceptance suite once.
- Repeat full clean builds only after dependency, generation, migration, container, or base-path changes.
- Do not rerun deterministic generation checks when source contracts and queries have not changed.
- Do not use verification as a substitute for finishing feature behavior.

## Durable lessons already recorded

- Goose procedural SQL requires `StatementBegin` and `StatementEnd` around PL/pgSQL bodies.
- Open-ended PostgreSQL `int4range` values must use an unbounded `NULL` upper limit rather than an inclusive maximum-integer sentinel.

## Launch-only inputs

GitHub ownership, registry ownership, production URL, VM details, branding, approved legal content, privacy approval, TLS ownership, and final institutional catalog content remain launch-readiness inputs. They do not block the technical MVP.
