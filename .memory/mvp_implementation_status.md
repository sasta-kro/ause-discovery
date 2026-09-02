# MVP implementation status

## Technical completion

Status as of 2026-09-02: foundation and data contracts complete; application features in progress.

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

## Active implementation work

- Authentication, canonical records, Artifacts, search, imports, and complete public and administrator interfaces.

## Known implementation constraints

- The host Node.js version is below the exact repository baseline; the pinned container toolchain provides the reproducible path.
- Go is absent on the host; Go verification uses the pinned container toolchain.
- `typescript-eslint` 8.69.0 rejects TypeScript 7.0.2. The current lint gate runs ESLint on compatible JavaScript configuration and uses `tsc -b` for TypeScript static checking while preserving the accepted exact pins.
- `@hey-api/openapi-ts` 0.99.0 rejects TypeScript 7.0.2. Reproducible client generation uses the isolated `api/generator` workspace with TypeScript 5.9.3 while application compilation remains on TypeScript 7.0.2.

## Launch-only inputs

GitHub ownership, registry ownership, production URL, VM details, branding, approved legal content, privacy approval, TLS ownership, and final institutional catalog content remain launch-readiness inputs rather than technical MVP blockers.
