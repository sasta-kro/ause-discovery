# MVP implementation status

## Technical completion

Status as of 2026-09-02: foundation complete; application features in progress.

## Verified increments

### Foundation

- Exact pnpm workspace and dependency pins are tracked.
- Neutral Go module, typed configuration, normalized public base paths, structured logging, health endpoints, graceful shutdown, and non-root container runtime are implemented.
- Root Makefile, environment example, Docker ignore rules, Compose topology, pinned service images, and Nginx SPA routing are implemented.
- Frontend production builds work for `/` and `/ause-discovery/`.
- The production-like Compose stack builds and starts with healthy PostgreSQL, Meilisearch, and API services.
- Default-base-path liveness, readiness, redirect, SPA delivery, and nested-route fallback are verified through Nginx.
- Frontend TypeScript checking, JavaScript configuration linting, seven base-path tests, and the production build pass.

## Active implementation work

- OpenAPI transport contract and generated clients.
- PostgreSQL migrations, sqlc queries, catalog fixtures, and synchronization.
- Authentication, canonical records, Artifacts, search, imports, and complete public and administrator interfaces.

## Known implementation constraints

- The host Node.js version is below the exact repository baseline; the pinned container toolchain provides the reproducible path.
- Go is absent on the host; Go verification uses the pinned container toolchain.
- `typescript-eslint` 8.69.0 rejects TypeScript 7.0.2. The current lint gate runs ESLint on compatible JavaScript configuration and uses `tsc -b` for TypeScript static checking while preserving the accepted exact pins.

## Launch-only inputs

GitHub ownership, registry ownership, production URL, VM details, branding, approved legal content, privacy approval, TLS ownership, and final institutional catalog content remain launch-readiness inputs rather than technical MVP blockers.
