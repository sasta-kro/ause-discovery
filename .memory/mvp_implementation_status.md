# MVP implementation status

## Technical completion

Status as of 2026-09-02: foundation, data contracts, and identity and core records complete; Artifact implementation is next.

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

## Active implementation work

- Artifacts, search, imports, audit administration, CI, operator documentation, and product-completion verification.

## Known implementation constraints

- The host Node.js version is below the exact repository baseline; the pinned container toolchain provides the reproducible path.
- Go is absent on the host; Go verification uses the pinned container toolchain.
- `typescript-eslint` 8.69.0 rejects TypeScript 7.0.2. The current lint gate runs ESLint on compatible JavaScript configuration and uses `tsc -b` for TypeScript static checking while preserving the accepted exact pins.
- `@hey-api/openapi-ts` 0.99.0 rejects TypeScript 7.0.2. Reproducible client generation uses the isolated `api/generator` workspace with TypeScript 5.9.3 while application compilation remains on TypeScript 7.0.2.

## Launch-only inputs

GitHub ownership, registry ownership, production URL, VM details, branding, approved legal content, privacy approval, TLS ownership, and final institutional catalog content remain launch-readiness inputs rather than technical MVP blockers.
