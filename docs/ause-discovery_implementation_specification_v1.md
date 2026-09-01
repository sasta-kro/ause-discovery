# AUSE Discovery Implementation Specification and Build Guide

**Status:** Accepted implementation baseline

**Document version:** 1.1.0

**Date:** 2026-09-02

**Product specification:** `docs/ause-discovery_specification_v1.md`

**Domain glossary:** `CONTEXT.md`

## 1. Role of This Document

The product specification defines product scope, behavior, content, and acceptance intent. This guide sits on top of that specification. It resolves architecture, technical boundaries, risky implementation details, version pins, and a practical build sequence.

This guide deliberately does not repeat every product requirement. Feature behavior that is not narrowed here comes directly from the product specification.

Implementation agents use the documents in this order:

1. `AGENTS.md` for repository and delegation rules.
2. `.memory/MEMORY.md` and routed project memory for durable context.
3. `CONTEXT.md` for exact domain language.
4. `docs/ause-discovery_specification_v1.md` for product requirements.
5. this guide for implementation decisions and sequencing.

### 1.1 Decision Boundaries

The following choices are fixed because divergence would create cross-cutting rework:

- the monorepo shape and root developer commands;
- the Go modular monolith boundary;
- PostgreSQL as canonical state;
- Meilisearch as disposable derived state;
- OpenAPI as the HTTP contract source;
- database, search, Artifact, import, authentication, and deployment architecture;
- public URL base-path behavior;
- exact dependency and service versions;
- lifecycle, transaction, concurrency, and security semantics stated in this guide.

Implementation agents retain discretion over local details that do not change those choices, including:

- private function and file decomposition;
- unexported types;
- small helper APIs;
- query names and query grouping;
- component composition below route level;
- test fixture construction;
- CSS implementation within the token and accessibility rules;
- minor indexes justified by query plans;
- exact error wording when a stable error code is already defined.

Root-agent review is required before changing a fixed choice. A local decision does not need an ADR, document update, or user confirmation merely because multiple valid implementations exist.

### 1.2 Autonomous Progress Rule

Recoverable or launch-time unknowns do not block local implementation.

In particular, the following are not bootstrap prerequisites:

- GitHub owner;
- final repository URL;
- GHCR owner;
- production hostname;
- final public base path;
- VM size or CPU architecture;
- TLS termination owner;
- final branding, legal copy, contact details, or institutional links;
- final backup operator or schedule.

Local defaults and configuration seams cover those values until deployment information exists. GitHub Actions publication, production deployment, and final content remain incomplete until their specific external inputs arrive, but unrelated application work continues.

User input is reserved for secrets, external account authorization, destructive external actions, legally sensitive content, and genuinely irreversible product choices not settled by the specifications. Implementation uncertainty alone is handled through repository inspection, a reasonable local decision, verification, and escalation to the root agent when the decision is cross-cutting.

### 1.3 Change Recording

The repository uses lightweight durable records:

- `CONTEXT.md` changes only when domain language changes.
- `.memory/architectural_decision_logs.md` records hard-to-reverse architecture choices with real tradeoffs.
- `.memory/lessons_learned.md` records verified failure patterns that could recur.
- `.memory/mvp_implementation_status.md` records completed and verified implementation state after coding begins.

Routine implementation choices, normal bug fixes, work logs, and speculative concerns do not belong in ADRs or long-term memory.

## 2. Architecture at a Glance

### 2.1 Runtime Topology

```text
Browser -> Nginx
           |-- static SPA files
           `-- /api/v1/* -> Go API
                              |-- PostgreSQL, canonical state
                              |-- Meilisearch, derived index
                              `-- local Artifact volume
```

The production shape contains five Compose services:

- `web`: Nginx serving the built SPA and proxying application traffic;
- `api`: one Go API and in-process background reconciliation process;
- `postgres`: canonical relational storage;
- `meilisearch`: rebuildable search storage;
- `migrate`: one-shot migration service using the same backend image.

No Redis, broker, distributed worker, or microservice is part of the MVP.

### 2.2 Canonical and Derived State

PostgreSQL owns every business fact. Meilisearch owns no fact that cannot be recreated from PostgreSQL. Artifact metadata belongs in PostgreSQL; Artifact bytes belong on a persistent controlled filesystem volume.

The normal searchable mutation flow is:

```text
database transaction
  -> mutate canonical records
  -> increment Project revision
  -> upsert pending search-sync state
  -> append audit event when required
commit
  -> attempt immediate index update
  -> retry in-process until synchronized
```

Meilisearch failure never rolls back a committed PostgreSQL mutation. Search can return an unavailable response while public Project and Person detail continue from PostgreSQL.

### 2.3 Backend Shape

The backend is a feature-oriented Go modular monolith. Modules own domain logic and SQL-facing behavior for a capability. Cross-module calls go through narrow application interfaces or shared domain values, not through generic repository abstractions.

Target modules:

- `auth`
- `audit`
- `catalog`
- `projects`
- `people`
- `artifacts`
- `imports`
- `search`

Shared infrastructure is limited to configuration, database transaction handling, HTTP middleware, generated transport types, logging, time, IDs, and storage adapters.

Handlers perform transport translation, authentication lookup, application-service invocation, and response mapping. Business validation does not live in handlers. SQL does not live in handlers. Generated files contain no handwritten logic.

### 2.4 Frontend Shape

The frontend is one React SPA with public and administrator route groups. It uses:

- React Router for navigation and base-path support;
- TanStack Query for server state;
- React Hook Form plus Zod for forms and immediate client validation;
- the generated OpenAPI client for wire calls and wire types;
- CSS Modules plus semantic CSS custom properties;
- i18next for all visible application copy.

Server state is not duplicated in a global client store. Local component state handles transient presentation. Authentication state is obtained from the session endpoint and query cache.

## 3. Monorepo Contract

### 3.1 Target Layout

```text
/
├── .github/workflows/
├── .memory/
├── api/
│   ├── openapi.yaml
│   └── openapi.codegen.yaml
├── backend/
│   ├── cmd/
│   │   ├── api/
│   │   └── ausectl/
│   ├── internal/
│   │   ├── auth/
│   │   ├── audit/
│   │   ├── catalog/
│   │   ├── projects/
│   │   ├── people/
│   │   ├── artifacts/
│   │   ├── imports/
│   │   ├── search/
│   │   └── platform/
│   ├── migrations/
│   ├── queries/
│   ├── generated/
│   ├── go.mod
│   └── go.sum
├── config/
│   ├── catalogs/
│   └── taxonomy/
├── deploy/
│   └── nginx/
├── docs/
├── frontend/
│   ├── src/
│   │   ├── app/
│   │   ├── api/generated/
│   │   ├── features/
│   │   ├── routes/
│   │   ├── shared/
│   │   ├── styles/
│   │   └── locales/
│   ├── package.json
│   └── vite.config.ts
├── compose.yaml
├── Makefile
├── package.json
├── pnpm-lock.yaml
├── pnpm-workspace.yaml
├── .env.example
└── README.md
```

Minor additions are allowed when an implemented feature needs them. Empty speculative directories are unnecessary.

### 3.2 Neutral Local Identity

Local bootstrap must not wait for a remote repository identity.

- Initial Go module path: `ause-discovery.local/backend`.
- Local backend image: `ause-discovery-api:dev`.
- Local web image: `ause-discovery-web:dev`.
- Default Compose project name: `ause-discovery`.

The Go module path can be changed mechanically when a permanent repository path is selected. Production image publication is configured later from the actual GitHub repository context. No fake organization name is committed.

### 3.3 Root Commands

The root `Makefile` is the stable interface used by humans and agents. At minimum:

```text
make doctor          verify required local tool versions
make install         install locked frontend dependencies and Go tools
make generate        run OpenAPI and sqlc generation
make generate-check  fail when generated output differs
make fmt             format frontend, Go, SQL, YAML, and Markdown where configured
make lint            run static checks
make test            run fast unit and component tests
make test-integration run database, search, and storage integration tests
make test-e2e        run browser acceptance tests
make test-all        run the complete verification set
make compose-up      start the local stack
make compose-down    stop the local stack without deleting persistent volumes
make migrate         apply database migrations
make seed            validate and synchronize catalog data
make dev             start the normal local development environment
```

Commands may be added as implementation needs become concrete. Command names above remain stable after introduction.

### 3.4 Tracked and Ignored State

Tracked:

- lockfiles;
- generated API and SQL code;
- migration files;
- catalog and taxonomy seeds;
- Compose and Nginx configuration;
- non-secret example configuration;
- tests and deterministic fixtures.

Ignored:

- `.env` and secret variants;
- dependency directories;
- build output;
- local database, Meilisearch, and Artifact data;
- import uploads and temporary files;
- test reports and browser traces unless intentionally added as fixtures;
- editor, operating-system, and local-agent scratch files.

## 4. Exact Version Baseline

All direct application dependencies use exact versions in manifests. Lockfiles are committed. Go modules remain fixed through `go.mod` and `go.sum`. Production container references use both an exact tag and a manifest digest.

### 4.1 Toolchains and Services

| Component | Version |
| --- | --- |
| Go | `1.27.0` |
| Node.js | `24.20.0` |
| pnpm | `11.25.0` |
| Docker Engine baseline | `29.7.2` |
| Docker Compose baseline | `5.4.0` |
| PostgreSQL | `18.6` |
| Meilisearch | `1.53.1` |
| Nginx | `1.30.4` |

`package.json` uses `packageManager: pnpm@11.25.0` and exact `engines.node` compatibility. The Go module uses `go 1.27.0`.

An absent host Go installation does not block work that can run through the pinned container toolchain. `make doctor` reports mismatch and available container fallback instead of treating every mismatch as a repository blocker.

### 4.2 Frontend Runtime Dependencies

| Package | Version |
| --- | --- |
| `react` | `19.2.8` |
| `react-dom` | `19.2.8` |
| `react-router` | `8.3.1` |
| `@tanstack/react-query` | `5.102.8` |
| `react-hook-form` | `7.87.0` |
| `zod` | `4.5.4` |
| `i18next` | `26.4.1` |
| `react-i18next` | `17.0.13` |

### 4.3 Frontend Development Dependencies

| Package | Version |
| --- | --- |
| `vite` | `8.2.2` |
| `@vitejs/plugin-react` | `6.1.1` |
| `typescript` | `7.0.2` |
| `@hey-api/openapi-ts` | `0.99.0` |
| `vitest` | `4.1.11` |
| `@playwright/test` | `1.62.1` |
| `@testing-library/react` | `16.3.3` |
| `@testing-library/user-event` | `14.6.6` |
| `msw` | `2.15.0` |
| `@axe-core/playwright` | `4.13.0` |
| `eslint` | `10.9.1` |
| `typescript-eslint` | `8.69.0` |
| `prettier` | `3.9.6` |

Supporting Vite and testing packages may be added at exact compatible versions. A new major framework, state library, component system, or data client requires root-agent approval.

### 4.4 Backend Dependencies and Tools

| Module or tool | Version |
| --- | --- |
| `github.com/go-chi/chi/v5` | `v5.3.2` |
| `github.com/jackc/pgx/v5` | `v5.10.0` |
| `github.com/sqlc-dev/sqlc` | `v1.31.1` |
| `github.com/pressly/goose/v3` | `v3.27.3` |
| `github.com/oapi-codegen/oapi-codegen/v2` | `v2.8.0` |
| `github.com/meilisearch/meilisearch-go` | `v0.36.3` |
| `github.com/xuri/excelize/v2` | `v2.11.0` |
| `github.com/google/uuid` | `v1.6.0` |
| `golang.org/x/crypto` | `v0.55.0` |
| `github.com/testcontainers/testcontainers-go` | `v0.44.0` |

Standard-library facilities remain preferred when sufficient. Additional direct modules require a concrete implementation need and an exact version.

### 4.5 Container Images

```text
golang:1.27.0-alpine3.23@sha256:3747dcba41c8b0db3211fda4db61638b980e17ac5bb3c94460a975a9cfe19395
node:24.20.0-alpine3.23@sha256:0388af2af070cd4736a1567cfed02469ba117848845b4165d87a333edb53d2ca
nginx:1.30.4-alpine@sha256:97d490c12ba55b4946b01546d1c3ed324e8d41ab1c9fcb2a616aa470620e5b46
postgres:18.6-alpine@sha256:d3e1620b530c944afa6e887d22eb899824da68e19c52024bf98f5220c88a65b2
getmeili/meilisearch:v1.53.1@sha256:8d6643d86d71fad6ad3cba92cde7ccfce9e4d6c384bda67598eb553571c32431
```

Multi-stage application builds use the pinned Go and Node builder images. Final backend runtime may use the pinned Alpine version after the runtime libraries and non-root account are explicit.

### 4.6 Upgrade Rule

Version upgrades are ordinary maintenance, not a ceremony. A coherent upgrade changes manifests, lockfiles, image digests, generated output if affected, and relevant tests in one change. Major upgrades receive a short ADR only when they alter architecture or persisted behavior.

## 5. Configuration and Base Paths

### 5.1 Configuration Model

The backend reads environment variables once at startup into a typed configuration value. Production has no silent defaults for secrets. Development receives safe defaults through Compose and `.env.example`.

Core variables:

| Variable | Development default | Meaning |
| --- | --- | --- |
| `AUSE_ENV` | `development` | `development`, `test`, or `production` |
| `AUSE_LISTEN_ADDR` | `:8080` | API listen address |
| `AUSE_PUBLIC_BASE_PATH` | `/ause-discovery/` | normalized public path prefix |
| `AUSE_DATABASE_URL` | Compose connection | PostgreSQL DSN |
| `AUSE_MEILISEARCH_URL` | `http://meilisearch:7700` | internal search URL |
| `AUSE_MEILISEARCH_API_KEY` | local-only key | search API key |
| `AUSE_MEILISEARCH_INDEX` | `projects` | stable index UID |
| `AUSE_ARTIFACT_ROOT` | `/var/lib/ause-discovery/artifacts` | absolute storage root |
| `AUSE_IMPORT_TEMP_ROOT` | `/var/lib/ause-discovery/imports` | same-filesystem temporary root |
| `AUSE_MAX_ARTIFACT_BYTES` | `262144000` | per-file upper bound |
| `AUSE_MAX_PROJECT_ARTIFACT_BYTES` | `2147483648` | active Project quota |
| `AUSE_SESSION_IDLE_TTL` | `30m` | idle expiration |
| `AUSE_SESSION_ABSOLUTE_TTL` | `12h` | absolute expiration |
| `AUSE_COOKIE_SECURE` | `false` | secure cookies outside local HTTP |
| `AUSE_TRUSTED_PROXY_CIDRS` | empty | explicitly trusted proxies |
| `AUSE_FEATURED_PROJECT_IDS` | empty | comma-separated UUID list |
| `AUSE_LOG_LEVEL` | `info` | structured log threshold |

Frontend build configuration is limited to public values:

| Variable | Default | Meaning |
| --- | --- | --- |
| `VITE_PUBLIC_BASE_PATH` | `/ause-discovery/` | router and asset base |
| `VITE_API_PATH` | `api/v1` | relative API path below public base |

No browser bundle contains database, Meilisearch, session, registry, or deployment secrets.

### 5.2 Path Normalization

The public base path:

- starts and ends with `/`;
- collapses duplicate separators;
- rejects query strings and fragments;
- supports `/` and `/ause-discovery/`;
- is applied exactly once to assets, browser routes, API calls, cookies, redirects, and public Artifact URLs.

The OpenAPI document defines paths relative to `/api/v1`. Deployment configuration adds the public base path. Application handlers do not hardcode `/ause-discovery/`.

### 5.3 Startup and Health

Startup fails on malformed configuration, missing production secrets, unwritable required directories, or an unsupported database migration state. Meilisearch unavailability does not prevent API startup.

Endpoints:

- `/health/live`: process is running;
- `/health/ready`: PostgreSQL reachable, schema supported, Artifact root usable;
- search availability appears separately in administrator search status.

Compose health checks use internal service endpoints. Nginx exposes the application health route under the configured base path.

## 6. Persistence Model

This section fixes table boundaries and invariants. Exact SQL column ordering, helper views, generated query names, and additional evidence-backed indexes remain implementation details.

### 6.1 Shared Database Rules

- PostgreSQL 18.6 is authoritative.
- UUIDv7 values are generated in Go.
- Timestamps use `timestamptz` and UTC.
- Mutable business records carry `created_at` and `updated_at`.
- Mutable aggregate roots carry a positive `revision` for optimistic concurrency.
- Status fields use `text` plus `CHECK` constraints, not PostgreSQL enums.
- Empty optional strings normalize to `NULL`.
- Foreign keys and frequent filter paths receive indexes.
- Archival records do not use cascade deletion.
- Migrations are forward-only for production recovery. Application rollback never assumes a down migration.

### 6.2 Project Aggregate

`projects` is the aggregate root. Required implementation fields:

- UUID identity;
- optional non-unique `reference_code`;
- title, abstract, academic year, and semester;
- exact Program, Major, and Course version references;
- `draft`, `published`, or `deleted` status;
- pre-delete status;
- JSON object extension metadata;
- revision and lifecycle timestamps.

`project_title_aliases` stores ordered aliases with normalized duplicate detection.

`project_participations` joins a Person to a Project with role `student`, `advisor`, `co_advisor`, or `committee_member`, plus order within the role.

`project_taxonomy_values` joins Project to controlled Taxonomy Value with order within a dimension.

Lifecycle rules:

- drafts may be incomplete;
- publication validates the complete Project aggregate;
- publication requires at least one student, one advisor, one category, one platform, Program, Course, title, abstract, academic year, and semester;
- Major is required only where the selected Program configuration requires one;
- deletion preserves the prior `draft` or `published` state;
- restoration returns to that prior state;
- permanent Project destruction is absent;
- every successful aggregate mutation increments revision once;
- published searchable changes mark search synchronization in the same transaction.

Collection updates replace the submitted collection atomically after validating the complete result. Partial mutation of ordered child collections is unnecessary for the MVP.

### 6.3 People

`people` stores academic identity separately from authenticated Application Users:

- display name;
- normalized name for matching;
- optional public Student ID;
- optional staff ID;
- revision and timestamps.

Student ID is exactly seven ASCII digits and unique when present. Staff ID is unique after normalization when present. Name-only matches produce duplicate candidates but never an automatic merge. Person deletion and automatic merge are outside the MVP.

### 6.4 Academic Catalog

Stable identities and historical labels are separate:

- `programs` and `program_versions`;
- `majors` and `major_versions`;
- `courses` and `course_versions`;
- `course_program_versions` for supported Program and Course combinations.

Projects reference version rows, preserving the historical display label. Version validity uses inclusive Gregorian years. Validity ranges for the same stable identity cannot overlap.

Catalog source files use fixed committed UUIDs and stable lower-case snake-case keys. Synchronization validates the complete YAML before a transaction, inserts missing records, applies allowed corrections, and retires values only when retirement is explicit. Absence from a later seed never means deletion.

The initial vocabulary can be minimal and clearly marked pending institutional expansion. Lack of final institutional vocabulary does not block schema, synchronization logic, fixtures, or CRUD development.

### 6.5 Taxonomy

`taxonomy_values` contains:

- fixed UUID;
- dimension;
- stable key;
- localized labels with at least `en`;
- optional description;
- sort order;
- optional retirement timestamp;
- timestamps.

Dimensions are `category`, `platform`, `domain`, `topic`, and `technology`. Stable key uniqueness is scoped to dimension. Retired values remain readable but cannot be newly assigned.

The initial key set comes from product specification Section 12.3 and committed YAML. Seed synchronization follows the academic catalog strategy.

### 6.6 Artifacts

`artifacts` stores metadata only:

- UUID and Project reference;
- Artifact type and display name;
- sanitized original filename for display;
- opaque unique storage key;
- server-detected MIME type and normalized extension;
- byte count and SHA-256 digest;
- `active` or `deleted` status;
- revision, actor, and timestamps.

Artifact bytes are publicly accessible only when the Artifact is active and the Project is published. Deletion keeps bytes. Restoration revalidates quota and compatibility.

### 6.7 Authentication, Sessions, and Audit

Tables:

- `application_users`: local administrator identity and status;
- `local_credentials`: encoded Argon2id credential for one Application User;
- `sessions`: hashes of session and CSRF tokens plus expiry and revocation state;
- `login_failures`: bounded rate-limit evidence by normalized username and source IP;
- `audit_events`: append-only security and mutation record.

Raw passwords, session tokens, CSRF tokens, abstracts, import row payloads, and Artifact bytes never enter audit metadata or logs.

### 6.8 Search Synchronization

`project_search_sync` stores one desired projection state per Project:

- desired Project revision;
- indexed revision;
- desired action `upsert` or `remove`;
- `pending`, `processing`, `synced`, or `failed` state;
- attempt count and next attempt time;
- lease owner and expiry;
- bounded safe error details;
- attempt and synchronization timestamps.

`search_rebuild_operations` stores full rebuild state, physical index names, progress counts, initiator, timestamps, and safe error details. A database constraint allows only one active rebuild.

### 6.9 Import Persistence

`import_batches` stores upload identity, source hash, format, state, row counts, expiry, revision, and immutable commit result.

`import_rows` stores row number, stable import key, canonical normalized draft, validation issues, selection, warning acknowledgement, duplicate resolution, and committed Project identity.

Source upload bytes are temporary. Parsed preview state remains for 24 hours. Expired uncommitted row payloads are cleaned in bounded repeatable batches. Audit summaries remain.

### 6.10 Migration Order

The initial migration series follows dependency order:

1. shared database prerequisites;
2. academic catalog identities and versions;
3. taxonomy;
4. Application Users, credentials, sessions, and login failures;
5. people;
6. projects and aliases;
7. participation and taxonomy joins;
8. Artifacts;
9. search synchronization and rebuild operations;
10. imports;
11. audit events and final indexes.

Catalog seed data is not embedded in migrations. Migrations create structure; the catalog command synchronizes version-controlled YAML.

## 7. Application and HTTP Contract

### 7.1 OpenAPI Ownership

`api/openapi.yaml` is the wire source of truth for paths, methods, parameters, JSON shapes, errors, and security declarations.

Generated output includes:

- Go transport and shared schema types;
- TypeScript wire types and client functions.

Generated Go business handlers are excluded. Handwritten TypeScript duplicates of generated wire models are excluded. `make generate-check` regenerates into a clean state and fails on drift.

### 7.2 General HTTP Rules

- API version root: `/api/v1` below the deployment base path.
- JSON media type: `application/json; charset=utf-8`.
- Timestamps: RFC 3339 UTC strings.
- UUIDs: lower-case canonical text.
- Unknown JSON fields: rejected on mutation requests.
- Request body size: bounded per route.
- Request ID: accepted only when valid or generated; returned in response and Problem Details.
- Errors: RFC 9457 Problem Details with stable application `code` and optional field issues.
- Administrator mutations: require session, CSRF, authorization, and expected revision where an aggregate can race.
- Pagination: opaque cursor preferred for growing ordered collections; bounded `limit`, default 20 and maximum 100.

Stable error codes carry client behavior. Human-readable detail can improve without a client release.

### 7.3 Public Endpoint Groups

The public API exposes:

```text
GET /search
GET /projects/{project_id}
GET /people/{person_id}
GET /catalogs
GET /artifacts/{artifact_id}/view
GET /artifacts/{artifact_id}/download
```

`GET /search` accepts query, academic filters, Person filters, taxonomy filters, Artifact availability filters, sort, cursor, and limit. Multiple values within one facet use OR. Different facets use AND.

Public Project and Person details come from PostgreSQL. Search result projections come from Meilisearch. Deleted and draft Projects never appear publicly.

### 7.4 Authentication Endpoint Group

```text
POST /admin/auth/login
POST /admin/auth/logout
GET  /admin/auth/session
GET  /admin/auth/csrf
```

Login rotates any existing session. Logout revokes the server-side session. The browser receives only opaque tokens in cookies or the CSRF response contract.

### 7.5 Administrator Resource Groups

Project operations:

```text
GET  /admin/projects
POST /admin/projects
GET  /admin/projects/{project_id}
PUT  /admin/projects/{project_id}
POST /admin/projects/{project_id}/publish
POST /admin/projects/{project_id}/delete
POST /admin/projects/{project_id}/restore
```

Person operations:

```text
GET   /admin/people
POST  /admin/people
GET   /admin/people/{person_id}
PATCH /admin/people/{person_id}
```

Artifact operations:

```text
POST  /admin/projects/{project_id}/artifacts
PATCH /admin/artifacts/{artifact_id}
POST  /admin/artifacts/{artifact_id}/delete
POST  /admin/artifacts/{artifact_id}/restore
POST  /admin/artifacts/{artifact_id}/replace
```

Import operations:

```text
POST  /admin/imports
GET   /admin/imports/{batch_id}
GET   /admin/imports/{batch_id}/rows
PATCH /admin/imports/{batch_id}/rows
POST  /admin/imports/{batch_id}/commit
GET   /admin/imports/{batch_id}/result
```

Search and audit operations:

```text
GET  /admin/search/status
POST /admin/search/projects/{project_id}/reindex
POST /admin/search/rebuilds
GET  /admin/search/rebuilds/{operation_id}
GET  /admin/audit-events
```

Transport naming and exact response composition are finalized in OpenAPI before endpoint implementation. Endpoint semantics above are stable.

### 7.6 Optimistic Concurrency

Project, Person, Artifact metadata, and Import Batch mutations carry `expected_revision`. SQL updates include the expected revision in the predicate and increment revision atomically.

A stale revision returns `409` with code `revision_conflict`, current revision, and a safe current-resource summary when useful. The frontend preserves unsaved form data and offers reload or compare behavior. Silent last-write-wins behavior is prohibited.

### 7.7 Operator CLI

`backend/cmd/ausectl` calls the same application services and configuration loader as the API.

Required commands:

```text
ausectl admin create
ausectl admin reset-password --username <value>
ausectl admin disable --username <value>
ausectl catalog validate
ausectl catalog sync
ausectl search reindex-project --project-id <uuid>
ausectl search rebuild
ausectl migrations status
```

Passwords use a non-echoing terminal prompt. Password flags and password environment variables are absent.

## 8. Search Implementation

### 8.1 Index Contract

- stable serving UID: `projects`;
- primary key: `id`;
- one document per published Project;
- source-controlled integer search schema version;
- physical rebuild UID: `projects_rebuild_<schema-version>_<timestamp>`.

The Project search document contains:

- Project ID and revision;
- reference code, title, aliases, and abstract;
- academic period and Program, Major, Course keys and labels;
- Person IDs, names, and identifiers grouped by role;
- taxonomy keys and labels grouped by dimension;
- active Artifact types and report, slides, source-code, and dataset booleans;
- publication and update timestamps;
- an internal semester sort value.

It never contains Artifact storage details, session data, audit data, import data, `extra_metadata`, deleted data, or file contents.

### 8.2 Search Settings

Searchable attributes, in priority order:

1. Student IDs;
2. reference code;
3. title;
4. title aliases;
5. student names;
6. advisor, co-advisor, and committee names;
7. taxonomy labels and keys;
8. academic catalog codes and labels;
9. abstract.

Filterable attributes:

- academic year and semester;
- Program, Major, and Course keys;
- advisor Person IDs and Student IDs;
- every taxonomy key array;
- Artifact type array and availability booleans.

Sortable attributes:

- academic year;
- internal semester order;
- normalized title;
- publication timestamp;
- update timestamp.

The default ranking favors exact identifiers, title, aliases, people, controlled classification, then abstract. A seven-digit query first attempts exact Student ID or exact reference-code matching. Typo tolerance is not used for identifier matching.

Meilisearch formatted matches are converted to React nodes without `dangerouslySetInnerHTML`. Abstract excerpts are bounded.

### 8.3 Incremental Synchronization

The API starts an in-process reconciler after database readiness. The reconciler:

1. leases due pending or failed rows in small bounded batches;
2. rebuilds the document from current PostgreSQL state;
3. submits an upsert or remove task;
4. waits for the Meilisearch task result with a bounded timeout;
5. marks synchronized only if desired revision still matches;
6. returns changed-during-flight work to pending;
7. retries failures with capped exponential backoff and jitter.

Leases expire, allowing recovery after a crash. Multiple API instances remain safe even though one instance is the initial deployment target.

### 8.4 Full Rebuild

Full rebuild uses a new physical index:

1. claim the single active rebuild record;
2. create target index and apply complete settings;
3. stream published Projects from PostgreSQL in stable bounded pages;
4. submit batches and wait for task completion;
5. catch up Projects changed since rebuild start;
6. briefly serialize searchable mutations for final catch-up;
7. verify counts and sampled revisions;
8. atomically swap indexes;
9. reconcile synchronization revisions;
10. retain the old index through one verification window, then delete it.

Failure before swap leaves the serving index unchanged. A restart resumes or safely marks the recorded operation failed. Manual Project reindex only marks current desired state pending and lets the normal reconciler perform the write.

## 9. Artifact Implementation

### 9.1 Storage Boundary

The Artifact module owns a narrow filesystem abstraction equivalent to:

```text
Put(stream, expectedSize) -> storageKey, size, sha256
Open(storageKey) -> seekable stream, size
Exists(storageKey) -> boolean
```

Permanent deletion is intentionally absent.

Storage keys are application-generated and opaque. A versioned fan-out layout such as `v1/01/9a/<uuid>` avoids oversized directories. Resolved paths must remain beneath the configured root. Symlinks in storage paths are rejected. Temporary and final files share a filesystem so finalization can use atomic rename.

### 9.2 Limits and Allowlist

- maximum file size: `262144000` bytes;
- maximum active bytes per Project: `2147483648` bytes;
- zero-byte files: rejected;
- browser MIME type: advisory only;
- type decision: normalized extension plus server-side content detection;
- executable, HTML, SVG, script, disk-image, and macro-enabled Office formats: rejected.

Core allowlist:

| Artifact type | Extensions | Inline view |
| --- | --- | --- |
| `report` | `.pdf` | PDF |
| `slides` | `.pdf`, `.ppt`, `.pptx` | PDF only |
| `source_code` | `.zip`, `.tar.gz`, `.tgz` | none |
| `proposal` | `.pdf`, `.docx` | PDF only |
| `poster` | `.pdf`, `.png`, `.jpg`, `.jpeg` | PDF only |
| `dataset` | `.zip`, `.csv`, `.tsv`, `.json`, `.xlsx` | none |
| `demo_video` | `.mp4`, `.webm` | none |
| `other` | `.pdf`, `.docx`, `.pptx`, `.zip`, `.csv`, `.xlsx`, `.json` | PDF only |

### 9.3 Upload and Replacement

Upload sequence:

1. authorize administrator and validate expected Project revision;
2. validate metadata before consuming the file body;
3. stream to a unique temporary file while counting bytes and hashing;
4. inspect bounded content and enforce the allowlist;
5. lock the Project row and recheck active quota;
6. atomically rename to the final key;
7. insert Artifact metadata, increment Project revision, append audit event, and mark search pending in one transaction;
8. remove temporary or newly finalized bytes when the metadata transaction fails.

Concurrent uploads serialize final quota checks through the Project row lock.

Replacement creates a new Artifact identity and new bytes, soft-deletes the old Artifact, increments Project revision once, and records both IDs. Existing bytes are never overwritten.

### 9.4 Serving

Public serving checks both Project and Artifact status before opening bytes.

- PDF view: `Content-Disposition: inline`, `nosniff`, byte ranges.
- Download: safe RFC 5987 filename, attachment disposition, detected safe MIME or `application/octet-stream`.
- Missing bytes: `404 artifact_content_unavailable`, structured error log, Project metadata still available.
- Nginx and Go request handling: streaming, no whole-file buffering.

## 10. Import Implementation

### 10.1 Supported Inputs

Imports are metadata-only. No Artifact paths, Artifact URLs, or file bytes enter an import.

Limits:

| Limit | Value |
| --- | --- |
| upload | 25 MiB |
| Project rows | 5000 |
| participation rows | 30000 |
| classification rows | 50000 |
| cell text | 10000 Unicode code points |
| JSON array items per cell | 200 |
| expanded XLSX content | 250 MiB |
| parse duration | 60 seconds |

CSV contains one Project per row and exact case-sensitive headers:

```text
import_key,title,reference_code,abstract,academic_year,semester,program_key,major_key,course_key,title_aliases,students,advisors,co_advisors,committee_members,categories,platforms,domains,topics,technologies
```

Repeated fields are strict JSON arrays. Student IDs remain JSON strings so leading zeroes survive.

XLSX contains exactly three visible sheets:

- `Projects`
- `Participations`
- `Classifications`

The exact column lists and semantics come from product specification Section 15.7. Formulas, macros, external links, hidden sheets, and unknown sheets are rejected. Numeric Student ID cells that lost leading zeroes are errors, not guessed values.

### 10.2 Adapter Boundary

CSV and XLSX parsers both emit the same versioned canonical draft:

```json
{
  "schema_version": 1,
  "import_key": "row-001",
  "project": {},
  "participations": [],
  "classifications": []
}
```

Validation after adaptation is shared. Parser-specific code does not create Projects or People.

Each issue has severity, stable code, source location, message, normalized value when relevant, and candidate summaries when resolution is required. Stable codes include missing fields, invalid identifiers and periods, unknown or retired catalog keys, invalid JSON, unsupported formulas, duplicate participation, Person match required, Project duplicate candidate, and limit exceeded.

### 10.3 Preview and Commit

Batch states:

```text
previewing -> ready -> committing -> committed
     |          |          |
     v          v          v
   failed     expired     failed
```

Ready rows can be selected. Invalid rows cannot be selected. Warning rows require explicit acknowledgement. Ambiguous duplicates require explicit resolution. Name-only Person matches never auto-merge.

Commit is one PostgreSQL transaction:

1. lock batch and validate state, revision, expiry, and administrator;
2. load selected rows and reject unresolved issues;
3. lock matched Projects in stable UUID order;
4. revalidate catalog and duplicate decisions;
5. create or update People and Projects through the same application services used by manual entry;
6. replace child collections and increment Project revisions;
7. mark published Projects pending for search;
8. store row results, immutable batch result, and audit events;
9. mark committed and commit once.

Any row failure rolls back the complete selected set. Repeated commit after success returns the stored result. Meilisearch is never called inside the transaction.

## 11. Authentication and Security

### 11.1 Local Administrator Authentication

- Argon2id encoded password hashes with parameters defined as source constants and tested against a memory ceiling;
- minimum 12-character password for operator-created accounts;
- opaque random session token stored only as SHA-256 hash;
- independent random CSRF token stored only as SHA-256 hash;
- session idle timeout 30 minutes;
- session absolute timeout 12 hours;
- session rotation at login and password reset;
- all sessions revoked when an account is disabled or password is reset;
- generic login failure response;
- rate limiting by normalized username and source IP using PostgreSQL evidence;
- no public registration or browser account administration.

`ausectl` creates the first administrator and performs password reset or disable operations.

### 11.2 Cookies and CSRF

The session cookie is `HttpOnly`, `SameSite=Lax`, scoped to the normalized application base path, and `Secure` in production. State-changing administrator requests require a matching CSRF token in a custom header. Origin checks reject untrusted cross-origin mutation attempts.

### 11.3 Proxy and Headers

Forwarded client addresses are trusted only from configured proxy CIDRs. Nginx is the expected trusted proxy in Compose.

Nginx and the API set a compatible security baseline:

- `X-Content-Type-Options: nosniff`;
- restrictive `Content-Security-Policy` for the SPA;
- `Referrer-Policy`;
- frame protection through CSP `frame-ancestors`;
- `Permissions-Policy` disabling unused capabilities;
- HSTS only where HTTPS termination is confirmed.

### 11.4 Upload and Output Safety

- request limits at Nginx and Go layers;
- no trust in filenames or browser MIME claims;
- no path construction from user values;
- no raw HTML rendering for abstracts, labels, import issues, or search highlights;
- no secrets, tokens, password hashes, or full sensitive payloads in logs;
- bounded error strings stored for search and import failures;
- safe attachment filename encoding.

Security-sensitive operations append audit events. Audit storage is append-only through application permissions, not a general update API.

## 12. Frontend Implementation

### 12.1 Route Map

Public routes:

```text
/
/search
/projects/:projectId
/people/:personId
/about
/privacy
/accessibility
```

Administrator routes:

```text
/admin/login
/admin
/admin/projects
/admin/projects/new
/admin/projects/:projectId/edit
/admin/people
/admin/people/:personId
/admin/imports
/admin/imports/:batchId
/admin/search
/admin/audit
```

Routes remain relative to the configured basename. Refreshing a nested route works through Nginx SPA fallback.

### 12.2 Application Foundations

Provider order:

1. error boundary;
2. i18n;
3. router;
4. query client;
5. session and CSRF integration where needed.

The generated API client receives a base URL built from Vite base configuration and `VITE_API_PATH`. Feature modules expose query options and mutation hooks, while route components own navigation and page composition.

### 12.3 Public Experience

The home route remains search-first. Featured Projects are configuration-driven and disappear cleanly when empty.

Search state is URL state. Query, facets, sort, and cursor are parseable and shareable. Invalid values normalize or produce a clear empty/error state. Changing a filter resets pagination. Search responses preserve previous results during short refetches without hiding a new error.

Project and Person detail use canonical public endpoints, not data copied from search hits. Artifact actions expose View only for PDFs and Download for every permitted active Artifact.

### 12.4 Administrator Experience

Protected routes load session state before rendering. An expired session preserves the intended route and returns to login.

Project editing is one aggregate form with sections for core metadata, academic context, people, taxonomy, and Artifacts. Draft save allows incomplete data. Publish shows server validation issues in the relevant sections.

Delete and Restore use explicit confirmation and product wording. No permanent-destruction control appears.

Import screens support upload, persistent row preview, issue filtering, row selection, warning acknowledgement, duplicate resolution, atomic commit, and stable result display. Search maintenance exposes counts, Project reindex, rebuild start, and live rebuild progress.

Revision conflict behavior preserves unsaved data and offers a reload path. Destructive automatic conflict resolution is excluded.

### 12.5 Styling, Localization, and Accessibility

Semantic CSS variables define color, typography, spacing, radius, shadow, focus, and layout values. Feature styles use CSS Modules. A general-purpose design-system abstraction is added only after repeated components make the boundary real.

Visible copy uses translation keys from the first implementation. English is complete. Thai resources may begin as an incomplete explicitly tracked locale without blocking English MVP development.

Accessibility baseline:

- semantic landmarks and heading order;
- keyboard access for every action;
- visible focus;
- programmatic form labels, descriptions, and errors;
- status announcements for async search, import, upload, and rebuild operations;
- tables with headers and responsive alternatives where necessary;
- no color-only meaning;
- reduced-motion support;
- 200 percent zoom without loss of critical function;
- skip link and useful document titles.

## 13. Docker Compose and Operations

### 13.1 Compose Behavior

`compose.yaml` is the primary local and production-like orchestration file. It contains pinned PostgreSQL and Meilisearch images and locally built API and web images.

Persistent named volumes:

- PostgreSQL data;
- Meilisearch data;
- Artifact bytes.

Import temporary state may use an internal named volume or a subdirectory of the Artifact filesystem, provided cleanup and same-filesystem rename behavior remain correct.

PostgreSQL and Meilisearch have no host-published ports in the production profile. Local debug port exposure belongs in an opt-in development override or profile. Nginx is the normal public entry point.

### 13.2 Service Ordering

- PostgreSQL health precedes migration.
- Successful migration precedes API readiness.
- API startup tolerates temporarily unavailable Meilisearch.
- Nginx can start before API readiness but health reports remain accurate.
- Catalog synchronization is an explicit idempotent command, not a hidden migration side effect.

### 13.3 Nginx Routing

Nginx:

- serves fingerprinted static assets with long immutable caching;
- serves `index.html` without immutable caching;
- falls back to `index.html` for browser routes;
- proxies API and Artifact traffic without duplicating the base path;
- preserves streaming and range behavior;
- applies upload limits and timeouts appropriate to 250 MiB Artifacts;
- forwards request IDs and trusted client metadata;
- does not expose PostgreSQL or Meilisearch.

Both `/` and `/ause-discovery/` builds receive routing smoke tests. The default Compose configuration uses `/ause-discovery/`.

### 13.4 Logs and Recovery

Application logs are structured JSON in production and concise human-readable text in development. Logs include timestamp, level, request ID, operation, safe resource IDs, duration, and stable error code where applicable.

Operational recovery relies on:

- forward migrations;
- database and Artifact-volume backups managed outside MVP automation;
- complete Meilisearch rebuild from PostgreSQL;
- immutable prior application image availability after release publication;
- explicit operator commands for migration status, catalog sync, and search rebuild.

Unknown production backup policy does not block implementing backup-compatible persistent volumes and documenting required backup targets.

## 14. CI and Release

### 14.1 Validation Workflow

CI runs when a GitHub remote is available. The repository can define the workflow before final ownership is known because checkout and validation do not require a hardcoded owner.

Validation jobs cover:

- exact toolchain setup;
- dependency install from lockfiles;
- formatting and lint;
- OpenAPI validation and generation drift;
- sqlc generation drift;
- Go tests;
- frontend tests;
- PostgreSQL and Meilisearch integration tests;
- production builds;
- Playwright critical-path tests when the stack is available;
- dependency and container vulnerability reporting.

GitHub Actions use immutable commit SHA pins with a nearby release comment.

### 14.2 Image Publication

Tagged releases build API and web images and publish to GHCR using the runtime repository owner rather than a committed placeholder. Tags include the release version and commit SHA. Deployment consumes a digest.

Image publication is the only feature that depends on confirmed registry ownership. Local builds, tests, Compose, and all application features remain independent.

Automated VM deployment is outside the MVP. A human-operated deployment runbook consumes published image digests.

## 15. Verification Strategy

Testing follows risk boundaries rather than a required test count.

### 15.1 Fast Tests

Backend unit tests focus on:

- normalization and validation;
- Project lifecycle and publication readiness;
- Student ID rules;
- aggregate replacement and revision handling;
- import adaptation and issue generation;
- search document projection;
- retry state transitions;
- Artifact type compatibility and quota arithmetic;
- password and session rules;
- base-path normalization.

Frontend component tests focus on:

- URL search-state parsing and serialization;
- forms and server issue mapping;
- revision conflict preservation;
- authentication route behavior;
- import preview interactions;
- accessible delete, restore, upload, and rebuild controls;
- safe search highlighting.

### 15.2 Integration Tests

PostgreSQL integration tests cover migrations from empty state, constraints, transaction rollback, concurrency, search-sync writes, import atomicity, catalog synchronization, and audit creation.

Meilisearch integration tests cover settings, ranking intent, facets, exact Student ID behavior, incremental upsert and removal, failure retry, and full rebuild swap.

Filesystem integration tests cover traversal resistance, symlink rejection, size enforcement, type detection, cleanup after failures, quota races, byte ranges, and missing content.

### 15.3 Browser Acceptance Paths

Critical browser paths:

1. search by topic and refine across facets;
2. search exact Student ID and open Person then Project;
3. open Project and view or download Artifacts;
4. administrator login and session expiry recovery;
5. create draft, complete required fields, publish, search, edit, and observe update;
6. delete and restore a Project;
7. upload, replace, delete, and restore an Artifact;
8. import CSV and XLSX through preview and atomic commit;
9. observe search failure while Project detail remains available;
10. rebuild search after deleting derived index state;
11. refresh nested routes under `/ause-discovery/` and `/`;
12. keyboard and automated accessibility pass through critical flows.

Tests that need unavailable final institutional content use explicit deterministic fixtures. Launch content approval is not an application test dependency.

## 16. Implementation Map

This map is sequencing guidance, not a bureaucratic gate system. Agents can split or combine increments when file ownership and dependency order remain clear. A missing launch-time value does not stop an increment.

### 16.1 Foundation Increment

Primary outcome: a reproducible monorepo that builds through stable root commands.

Work:

- replace the default npm setup with the pinned pnpm workspace;
- establish neutral Go module identity;
- add exact manifests, lockfiles, tool install commands, and ignore rules;
- add root Makefile and `.env.example`;
- add initial Compose services with pinned PostgreSQL and Meilisearch;
- establish OpenAPI and sqlc generation directories;
- add backend configuration, logging, health, and shutdown skeleton;
- normalize frontend base-path configuration and test harness.

Useful verification:

- dependency installation from lockfiles;
- empty backend and frontend builds;
- PostgreSQL and Meilisearch health through Compose;
- root and `/ause-discovery/` static routing smoke test;
- repeated generation produces no diff.

No GitHub owner or remote URL is required.

### 16.2 Data and Contract Increment

Primary outcome: stable contracts for independent backend and frontend feature work.

Work:

- define shared OpenAPI schemas, Problem Details, pagination, auth, and revision conventions;
- create migrations in dependency order;
- create sqlc configuration and initial query groups;
- implement transaction runner and UUID/time boundaries;
- create academic and taxonomy YAML schemas plus minimal valid seed fixtures;
- implement catalog validate and sync commands;
- generate Go and TypeScript contract output.

Useful verification:

- migrate from empty PostgreSQL;
- regenerate and compile both languages;
- catalog sync is deterministic and non-destructive;
- key constraints fail as intended.

Final university vocabulary is not required for the engine or fixtures.

### 16.3 Identity and Core Records Increment

Primary outcome: authenticated administration and canonical Project records.

Work:

- implement Application User, credential, session, CSRF, login throttling, and audit services;
- implement `ausectl admin` operations;
- implement People and Project aggregate services;
- implement draft, update, publish, delete, and restore;
- implement classification and participation replacement;
- expose public Project and Person details;
- expose administrator endpoints and generated-client hooks.

Useful verification:

- authorization and session tests;
- publication invariant tests;
- concurrent revision conflict tests;
- public visibility tests;
- audit assertions for security and mutations.

### 16.4 Artifact Increment

Primary outcome: controlled large-file storage and public access.

Work:

- implement storage adapter and directory safety;
- implement streaming upload, MIME detection, hashing, limits, and quota;
- implement metadata mutation, replacement, delete, and restore;
- implement public view and download with range support;
- add administrator UI and Project detail Artifact presentation.

Useful verification:

- no whole-file buffering;
- failure cleanup and transaction compensation;
- concurrent quota protection;
- traversal and symlink rejection;
- missing-content isolation.

### 16.5 Search Increment

Primary outcome: complete public discovery backed by rebuildable derived state.

Work:

- implement Project search projection;
- configure Meilisearch settings;
- implement pending state and reconciler;
- expose public search and facet validation;
- implement reindex and full rebuild operations;
- build Home, Search, Project, and Person public routes;
- add administrator search-status controls.

Useful verification:

- ranking and exact Student ID behavior;
- multi-value facet semantics;
- published update and deletion propagation;
- recovery after Meilisearch outage or data loss;
- Project detail remains available during search outage.

### 16.6 Import Increment

Primary outcome: persistent preview and atomic metadata import.

Work:

- implement bounded CSV and XLSX adapters;
- implement shared normalization, validation, duplicate candidates, and issue model;
- persist batch and row previews;
- implement selection, warning acknowledgement, and duplicate resolution;
- commit through canonical Project and Person services;
- add templates and administrator import UI;
- add expiry cleanup.

Useful verification:

- format and expansion limits;
- leading-zero Student IDs;
- formula and unsupported-sheet rejection;
- preview persistence across restart;
- all-or-nothing commit;
- repeated commit returns stable result.

### 16.7 Product Completion Increment

Primary outcome: coherent administrator experience and production-like local stack.

Work:

- complete administrator Project, Person, Artifact, Import, Search, and Audit routes;
- complete localization coverage and semantic styling;
- complete responsive and accessibility behavior;
- harden Nginx and container runtime users;
- add CI validation and release publication logic without hardcoded ownership;
- write operator runbook for migration, administrator creation, catalog sync, search recovery, persistent data, and manual deployment;
- run browser acceptance paths and fix cross-system defects.

Useful verification:

- clean-clone bootstrap through documented commands;
- complete Compose smoke test;
- root and subpath behavior;
- restart persistence;
- security and accessibility review;
- no unresolved critical-path defect.

Final branding, legal text, production URL, registry ownership, and VM values are inserted when available. Their absence affects launch readiness, not application completion.

## 17. Agent Execution Guidance

### 17.1 Root-Agent Responsibilities

The root Sol agent owns:

- architecture and interpretation of ambiguous cross-cutting requirements;
- assignment of bounded implementation areas;
- exclusive file ownership across concurrent workers;
- resolution of contract conflicts;
- review of major deviations and integration risk;
- updates to this guide, ADRs, and durable memory when truly warranted;
- final integration and acceptance assessment.

### 17.2 Worker Responsibilities

Implementation workers own routine engineering inside an assigned boundary:

- inspect relevant code and specifications before editing;
- choose sensible local structure without requesting approval for every detail;
- preserve fixed architecture and public contracts;
- implement a usable vertical or infrastructure increment rather than empty scaffolding;
- verify the changed behavior;
- return a concise report with changes, checks, risks, and decisions needed.

Workers escalate when a choice changes a fixed architecture boundary, public product behavior, persisted compatibility, security posture, or another worker's owned area. Workers do not escalate merely because several ordinary coding approaches are possible.

### 17.3 Coordination and Context Use

- Read-only exploration may run in parallel when questions are independent.
- Coding workers receive exclusive ownership of files or project areas.
- Root waits for final or milestone reports instead of consuming routine progress streams.
- Coding progress receives targeted checks at meaningful boundaries, especially before schema, contract, or cross-module divergence becomes expensive.
- Worker reports stay concise and decision-relevant.
- Failed approaches become lessons only after the failure mode is verified and reusable.
- Small fixes remain code changes, not process artifacts.

### 17.4 Default Resolution of Unknowns

When specifications do not settle a minor detail:

1. inspect adjacent contracts and existing conventions;
2. choose the simplest implementation consistent with current architecture;
3. add a focused test when the choice affects behavior;
4. continue work;
5. report the choice only when it matters to integration or future maintenance.

When a major ambiguity appears, the worker returns the concrete conflict and recommended resolution to the root agent. The root agent decides without involving the product owner unless the choice is genuinely product-defining, externally destructive, credential-dependent, or legally sensitive.

## 18. Explicit MVP Boundaries

The following remain outside implementation unless the product specification changes:

- Microsoft Entra ID;
- public accounts for students or lecturers;
- taxonomy or academic catalog CMS;
- Person merge UI;
- recommendations and similar-Project features;
- semantic or vector search;
- file-content indexing;
- AI extraction or tagging;
- analytics or telemetry;
- external-link Artifacts;
- embedded video or custom document viewers;
- antivirus or content-disarm infrastructure;
- permanent Project or Artifact destruction;
- automated backups and disaster recovery;
- Redis, message brokers, distributed workers, and microservices;
- automated VM deployment.

No speculative abstraction is needed for deferred work.

## 19. External Inputs and Their Actual Impact

| External input | Work that can proceed without it | Work that waits for it |
| --- | --- | --- |
| GitHub owner and repository URL | complete local monorepo, Go module, CI validation file, tests | final remote setup, permanent module-path rename if desired |
| GHCR owner | local images and Compose | image publication verification |
| final catalog vocabulary | schema, seed engine, fixtures, administration, imports | production catalog content approval |
| VM details | containers, Compose, health, persistence design | capacity tuning and host deployment |
| DNS and TLS owner | HTTP local stack and proxy security baseline | production HTTPS and HSTS activation |
| final public base path | root and default subpath support | final deployment setting |
| branding and legal copy | complete layout, tokens, i18n placeholders | production content approval |
| Student ID privacy approval | full implementation against accepted product spec | public production launch if policy changes |
| backup operator and schedule | persistent volumes and backup-compatible design | operational production sign-off |

External values remain visible in deployment readiness notes. None should be converted into a broad implementation blocker.

## 20. Completion Signal

The MVP implementation is technically complete when the product specification's in-scope journeys operate through the production-like Compose stack, the architecture in this guide is preserved, generated contracts and migrations reproduce cleanly, critical security and accessibility behavior is verified, persistent state survives restart, and Meilisearch can be destroyed and rebuilt from PostgreSQL.

Production launch is a separate readiness decision. Launch additionally needs approved institutional content, deployment values, credentials, registry ownership, TLS, privacy approval, and an operator-backed persistence plan. Separating technical completion from launch readiness keeps agentic implementation moving while preserving honest deployment status.
