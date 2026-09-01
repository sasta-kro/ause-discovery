# AUSE Discovery Implementation Specification

**Status:** Approved implementation baseline

**Document version:** 1.0.0

**Date:** 2026-09-01

**Product:** AUSE Discovery

**Technical slug:** `ause-discovery`

**Primary audience:** Implementation agents, maintainers, reviewers, operators

**Source product specification:** `docs/ause-discovery_specification_v1.md`

## 1. Purpose and Authority

This document converts the product specification into a decision-complete implementation contract. It defines repository structure, component boundaries, dependency versions, database design, API behavior, frontend behavior, import formats, search synchronization, artifact storage, authentication, security controls, deployment, operations, testing, and implementation order.

The product specification remains authoritative for product intent and MVP scope. This document is authoritative for implementation details. When an implementation detail in the product specification is deferred, optional, inconsistent, or expressed as an example, the decision in this document controls the MVP implementation.

This document does not authorize features outside the product specification. In particular, implementation work must not add recommendation systems, analytics, semantic search, report-body indexing, AI extraction, student accounts, lecturer accounts, custom document viewers, source-code browsing, Redis, message brokers, microservices, automatic backups, or automated disaster recovery.

### 1.1 Current Repository State

At document creation time:

- Git is initialized on `main` with an initial commit;
- `backend/` is empty;
- `frontend/` contains a default Vite React scaffold created during specification work;
- the frontend scaffold uses npm, version ranges, TypeScript 6.0.2, and `package-lock.json`, which do not match the accepted pnpm and exact-pin baseline;
- no root workspace manifests, Go module, migrations, backend source, application tests, Dockerfiles, Compose files, or CI workflows exist;
- `.memory/MEMORY.md` contains stale context from an unrelated project.

Implementation starts by normalizing the existing Git and Vite scaffold to the repository and reproducibility contracts in this document. No existing application behavior requires backward compatibility.

### 1.2 Requirement Language

The terms `MUST`, `MUST NOT`, `SHOULD`, `SHOULD NOT`, and `MAY` define requirement strength.

- `MUST` and `MUST NOT` are acceptance requirements.
- `SHOULD` and `SHOULD NOT` require a recorded justification when implementation differs.
- `MAY` describes permitted behavior that is not required.

### 1.3 Implementation Change Control

Any implementation change that contradicts this document requires all of the following:

1. a written rationale;
2. an update to this document;
3. an update to affected API, schema, or acceptance contracts;
4. a migration or compatibility plan when persisted state or a public interface changes;
5. reviewer approval before merge.

Unrecorded deviations are defects.

## 2. Canonical Product Decisions

### 2.1 Product Identity

- Public product name: `AUSE Discovery`.
- Technical slug: `ause-discovery`.
- Default production base path: `/ause-discovery/`.
- Root deployment at `/` remains supported and tested.
- Default PostgreSQL database name: `ause_discovery`.
- Default stable Meilisearch index UID: `projects`.
- Container image names use `ause-discovery-api` and `ause-discovery-web` beneath the confirmed GHCR owner.
- The GitHub owner and final repository URL are external identifiers that MUST be supplied before module initialization and image publication. Placeholder owners MUST NOT enter committed module paths or production image references.

### 2.2 Canonical Domain Terms

| Term | Definition | Prohibited substitutes |
| --- | --- | --- |
| Project | One historical senior project and the unit returned by search. | document, submission, search result record |
| Project ID | Application-owned UUIDv7 for a Project. | reference code, legacy ID |
| Reference code | Optional external university reference string with no inferred semantics or uniqueness guarantee. | project ID |
| Person | Academic person record associated with one or more Projects. | user, account |
| Student ID | Public, searchable, exactly seven-digit institutional student identifier. | student number, person ID |
| Project Participation | Relationship between a Person and a Project in a contextual role. | membership, embedded person |
| Application User | Authenticated administrative identity. | Person, admin Person |
| Artifact | Stored file associated with a Project. | attachment URL, project column |
| Taxonomy Value | Stable keyed classification within category, platform, domain, topic, or technology. | free-form tag |
| Import Batch | Persistent preview and commit boundary for one CSV or XLSX upload. | upload job, temporary file |
| Search Document | Denormalized Meilisearch projection for one published Project. | canonical project record |
| Search Synchronization | PostgreSQL-to-Meilisearch projection process. | replication |
| Delete | Reversible transition to the `deleted` Project or Artifact state. | hard delete, archive |
| Restore | Reversible transition from `deleted` to the preceding visible state. | undelete, unarchive |

### 2.3 Project Lifecycle

Project status values are:

- `draft`: administrator-visible only;
- `published`: publicly visible and eligible for search indexing;
- `deleted`: hidden from public reads and public search, retained in PostgreSQL, restorable.

Allowed transitions are:

| From | Action | To | Required effects |
| --- | --- | --- | --- |
| draft | Publish | published | Set `published_at`; mark search synchronization pending. |
| published | Save edit | published | Increment revision; mark search synchronization pending when searchable data changes. |
| draft | Delete | deleted | Set `deleted_at`; preserve `status_before_delete=draft`; do not index. |
| published | Delete | deleted | Set `deleted_at`; preserve `status_before_delete=published`; mark search removal pending. |
| deleted | Restore | prior status | Clear `deleted_at`; increment revision; mark search synchronization pending when restored to published. |

Direct transitions from `deleted` to an arbitrary status are prohibited. Permanent physical deletion is outside MVP scope.

### 2.4 Academic Calendar

- Academic year is a four-digit Gregorian completion year.
- Accepted semester values are `1`, `2`, and `summer`.
- Database values use lower-case text for semester.
- Display labels are localized through frontend translation resources.
- Imports reject Buddhist Era years, year ranges, and undocumented semester aliases unless a versioned normalization rule explicitly maps the value.

### 2.5 Student ID Policy

- Student ID MUST match `^[0-9]{7}$`.
- Student ID is public.
- Student ID is searchable with exact matching.
- Student ID is displayed on public Person pages and relevant Project participation rows.
- Student ID is unique when present.
- Staff identifiers are not required and are not assumed to follow the student ID format.

### 2.6 Catalog Administration

Taxonomy values, programs, majors, and courses are developer-controlled in the MVP.

- Definitions live in versioned YAML seed files.
- Administrators may assign existing definitions.
- Administrators may not create, rename, merge, split, or delete definitions through the UI.
- A catalog synchronization operator command validates and applies seed changes.
- Stable keys are never repurposed.
- Retired values remain readable for historical Projects.

### 2.7 Import Policy

- CSV and XLSX imports contain metadata only.
- Imported files do not contain artifact bytes or artifact filesystem paths.
- Parsed previews persist in PostgreSQL for 24 hours.
- Selected valid or warning rows commit atomically in one transaction.
- Invalid rows cannot be selected.
- Warning rows require explicit acknowledgement.
- Duplicate candidates require an explicit resolution.

### 2.8 Artifact Policy

- Artifact storage supports stored files only.
- External-link artifacts are deferred.
- Maximum file size is 250 MiB, represented as `262144000` bytes.
- Maximum active artifact bytes per Project is 2 GiB, represented as `2147483648` bytes.
- Both limits are configurable downward.
- The MVP artifact types are `report`, `slides`, `source_code`, `proposal`, `poster`, `dataset`, `demo_video`, and `other`.
- Deleted artifact bytes remain stored in the MVP.
- Permanent artifact-byte cleanup is deferred.

### 2.9 Optional Discovery Features Included in MVP

- Artifact availability facets: report, slides, source code, and dataset.
- Configuration-driven Featured Projects list.
- Featured Projects configuration may be empty.
- Invalid, draft, or deleted featured Project IDs are ignored and logged at warning level.

## 3. Repository Contract

### 3.1 Target Layout

```text
/
├── .github/
│   └── workflows/
├── api/
│   ├── openapi.yaml
│   └── openapi.codegen.yaml
├── backend/
│   ├── cmd/
│   │   ├── api/
│   │   └── ausectl/
│   ├── db/
│   │   ├── migrations/
│   │   ├── queries/
│   │   └── seeds/
│   ├── internal/
│   │   ├── artifacts/
│   │   ├── audit/
│   │   ├── auth/
│   │   ├── catalogs/
│   │   ├── config/
│   │   ├── database/
│   │   ├── httpapi/
│   │   ├── imports/
│   │   ├── people/
│   │   ├── projects/
│   │   └── search/
│   ├── go.mod
│   ├── go.sum
│   ├── oapi-codegen.yaml
│   └── sqlc.yaml
├── deploy/
│   ├── nginx/
│   └── production/
├── docs/
├── frontend/
│   ├── public/
│   ├── src/
│   │   ├── app/
│   │   ├── components/
│   │   ├── features/
│   │   ├── generated/
│   │   ├── i18n/
│   │   ├── routes/
│   │   ├── styles/
│   │   └── test/
│   ├── package.json
│   └── vite.config.ts
├── scripts/
├── .env.example
├── .gitignore
├── .node-version
├── compose.dev.yaml
├── compose.yaml
├── Makefile
├── package.json
├── pnpm-lock.yaml
├── pnpm-workspace.yaml
└── README.md
```

### 3.2 Git Tracking Rules

Tracked files include:

- all source code;
- product and implementation documentation;
- database migrations and SQL queries;
- taxonomy and academic catalog seed files;
- generated OpenAPI and sqlc code;
- package and Go lock/checksum files;
- Dockerfiles and Compose files;
- Nginx configuration templates;
- environment examples without secrets;
- scripts, Make targets, and CI workflows;
- sanitized test fixtures and import templates.

Ignored files include:

- `.env` and secret variants;
- PostgreSQL volumes or dumps;
- Meilisearch data or dumps;
- artifact storage contents;
- uploaded import files;
- temporary files;
- application logs;
- frontend build output;
- Go build output;
- editor and operating-system metadata.

Generated code MUST be committed. CI MUST regenerate code and fail when the generated diff is non-empty.

### 3.3 Root Command Contract

The root Makefile exposes these stable commands:

| Command | Behavior |
| --- | --- |
| `make bootstrap` | Validate required tools, enable pinned pnpm through Corepack, and install dependencies. |
| `make doctor` | Print required and detected tool versions; fail on unsupported versions. |
| `make generate` | Run OpenAPI, sqlc, and any deterministic generation. |
| `make check-generated` | Regenerate and fail when tracked output changes. |
| `make fmt` | Format Go, TypeScript, CSS, YAML, Markdown, and SQL where safe. |
| `make lint` | Run all non-mutating linters. |
| `make test` | Run unit and component tests. |
| `make test-integration` | Run PostgreSQL, Meilisearch, artifact, and API integration tests. |
| `make test-e2e` | Run Playwright tests against a built Compose environment. |
| `make test-all` | Run generation checks, lint, unit, integration, and E2E tests. |
| `make compose-up` | Start the local Compose stack. |
| `make compose-down` | Stop the local Compose stack without deleting persistent volumes. |
| `make migrate-up` | Apply all pending PostgreSQL migrations. |
| `make migrate-status` | Show migration state. |
| `make catalog-sync` | Validate and synchronize versioned catalog seed files. |
| `make admin-create` | Invoke the interactive operator-side administrator creation command. |
| `make search-rebuild` | Invoke the operator-side full index rebuild command. |

Destructive volume removal MUST NOT be part of a routine Make target.

## 4. Version Pinning

### 4.1 Pinning Policy

- Direct npm dependencies use exact versions without `^` or `~`.
- `pnpm-lock.yaml` is committed.
- `packageManager` is set to `pnpm@11.25.0` with the Corepack integrity hash.
- `.node-version` contains `24.20.0`.
- `go.mod` declares `go 1.27.0` and `toolchain go1.27.0`.
- Direct Go dependencies use exact module versions.
- `go.sum` is committed.
- Go tools are declared through the Go tool dependency mechanism and invoked with pinned versions.
- Production container images use exact tags and multi-platform manifest digests.
- GitHub Actions use full commit SHAs with a release comment.
- `latest`, floating major tags, and unbounded version ranges are prohibited.

### 4.2 Toolchain and Service Versions

| Component | Version |
| --- | --- |
| Go | 1.27.0 |
| Node.js | 24.20.0 LTS |
| pnpm | 11.25.0 |
| Docker Engine baseline | 29.7.2 |
| Docker Compose baseline | 5.4.0 |
| PostgreSQL | 18.6 |
| Meilisearch | 1.53.1 |
| Nginx | 1.30.4 |

### 4.3 Frontend Direct Dependencies

| Package | Version | Purpose |
| --- | --- | --- |
| `react` | 19.2.8 | UI runtime |
| `react-dom` | 19.2.8 | Browser renderer |
| `react-router` | 8.3.1 | Client routing |
| `@tanstack/react-query` | 5.102.8 | Server-state cache |
| `react-hook-form` | 7.87.0 | Form state |
| `zod` | 4.5.4 | Client validation and form schemas |
| `i18next` | 26.4.1 | Localization runtime |
| `react-i18next` | 17.0.13 | React localization bindings |

### 4.4 Frontend Development Dependencies

| Package | Version | Purpose |
| --- | --- | --- |
| `vite` | 8.2.2 | Build tool and development server |
| `@vitejs/plugin-react` | 6.1.1 | React Vite integration |
| `typescript` | 7.0.2 | Type checking |
| `@hey-api/openapi-ts` | 0.99.0 | TypeScript API client generation |
| `vitest` | 4.1.11 | Unit and component tests |
| `@playwright/test` | 1.62.1 | Browser tests |
| `@testing-library/react` | 16.3.3 | Component testing |
| `@testing-library/user-event` | 14.6.6 | User interaction testing |
| `msw` | 2.15.0 | API mocking |
| `@axe-core/playwright` | 4.13.0 | Browser accessibility assertions |
| `eslint` | 10.9.1 | JavaScript and TypeScript linting |
| `typescript-eslint` | 8.69.0 | TypeScript lint rules |
| `prettier` | 3.9.6 | Supported text formatting |

Transitive versions remain controlled by `pnpm-lock.yaml`.

### 4.5 Backend Dependencies and Tools

| Module | Version | Purpose |
| --- | --- | --- |
| `github.com/go-chi/chi/v5` | 5.3.2 | HTTP routing |
| `github.com/jackc/pgx/v5` | 5.10.0 | PostgreSQL driver and pool |
| `github.com/sqlc-dev/sqlc` | 1.31.1 | SQL code generation tool |
| `github.com/pressly/goose/v3` | 3.27.3 | Database migration tool and library |
| `github.com/oapi-codegen/oapi-codegen/v2` | 2.8.0 | Go OpenAPI type generation |
| `github.com/meilisearch/meilisearch-go` | 0.36.3 | Meilisearch client |
| `github.com/xuri/excelize/v2` | 2.11.0 | XLSX parsing and template generation |
| `github.com/google/uuid` | 1.6.0 | UUIDv7 generation |
| `golang.org/x/crypto` | 0.55.0 | Argon2id password hashing |
| `github.com/testcontainers/testcontainers-go` | 0.44.0 | Backend integration test dependencies |

Standard library packages are preferred for JSON, CSV, HTTP servers, structured logging, cryptographic randomness, hashing, content detection, and filesystem operations.

### 4.6 Container Image Pins

The following multi-platform manifest digests were verified on 2026-09-01:

| Use | Image reference |
| --- | --- |
| Go build | `golang:1.27.0-alpine3.23@sha256:3747dcba41c8b0db3211fda4db61638b980e17ac5bb3c94460a975a9cfe19395` |
| Frontend build | `node:24.20.0-alpine3.23@sha256:0388af2af070cd4736a1567cfed02469ba117848845b4165d87a333edb53d2ca` |
| Web runtime | `nginx:1.30.4-alpine@sha256:97d490c12ba55b4946b01546d1c3ed324e8d41ab1c9fcb2a616aa470620e5b46` |
| PostgreSQL | `postgres:18.6-alpine@sha256:d3e1620b530c944afa6e887d22eb899824da68e19c52024bf98f5220c88a65b2` |
| Meilisearch | `getmeili/meilisearch:v1.53.1@sha256:8d6643d86d71fad6ad3cba92cde7ccfce9e4d6c384bda67598eb553571c32431` |

Digest refreshes require an explicit dependency update with release-note review and verification.

### 4.7 Upgrade Procedure

Each dependency upgrade pull request MUST contain:

1. old and new versions;
2. upstream release-note links;
3. security and breaking-change assessment;
4. lockfile or checksum changes;
5. regenerated code changes;
6. migration impact for PostgreSQL or Meilisearch;
7. local and CI verification results;
8. rollback instructions for service-image or schema changes.

PostgreSQL major upgrades require a dedicated migration plan. Meilisearch upgrades require review of index-format compatibility, dump compatibility, and reindex behavior.

### 4.8 Version Verification Sources

The 2026-09-01 baseline was checked against primary distribution sources:

- [Go release history](https://go.dev/doc/devel/release)
- [Node.js 24 distribution index](https://nodejs.org/dist/latest-v24.x/)
- [npm registry](https://registry.npmjs.org/)
- [official Go module proxy](https://proxy.golang.org/)
- [PostgreSQL versioning policy](https://www.postgresql.org/support/versioning/)
- [Meilisearch v1.53.1 release](https://github.com/meilisearch/meilisearch/releases/tag/v1.53.1)
- [Nginx 1.30 changelog](https://nginx.org/en/CHANGES-1.30)
- [Docker Engine 29 release notes](https://docs.docker.com/engine/release-notes/29/)
- [Docker Compose releases](https://github.com/docker/compose/releases)
- Docker Hub multi-platform manifests inspected through Docker Buildx

Future implementation must not replace verified versions merely because a newer release exists. Version changes follow Section 4.7.

## 5. System Architecture

### 5.1 Runtime Topology

```text
Public browser
    |
    v
Nginx
    |-- static React assets
    |-- /ause-discovery/api/* -> Go API
    |-- all other application routes -> index.html
    |
    v
Go modular monolith
    |-- PostgreSQL
    |-- Meilisearch
    `-- artifact filesystem
```

Only Nginx exposes a public host port. PostgreSQL and Meilisearch remain on the internal Compose network. The Go API is reachable from Nginx and from internal health tooling only.

### 5.2 Canonical and Derived State

| State | Canonical | Backup priority | Rebuild behavior |
| --- | --- | --- | --- |
| PostgreSQL | Yes | Critical | Restore from PostgreSQL backup. |
| Artifact bytes | Yes | Critical | Restore from artifact backup. |
| Meilisearch index | No | Non-critical | Rebuild from PostgreSQL. |
| React assets | No | Non-critical | Rebuild from Git-tracked source. |
| Import upload temp files | No | None | Upload again. |

### 5.3 Backend Module Rules

Each backend feature module may contain domain types, validation, application services, and module-specific persistence adapters. Cross-module access occurs through explicit application-service calls or narrow interfaces.

Rules:

- HTTP handlers translate transport data and call application services.
- HTTP handlers do not contain SQL or filesystem paths.
- sqlc-generated code remains inside the database adapter boundary.
- Domain validation does not depend on HTTP status codes.
- Search projection consumes canonical read models, not HTTP response models.
- Audit recording is called by application services after authorization context is established.
- Transactions are owned by application services that define atomic business operations.
- No generic repository interface wraps every table.
- Interfaces require either a real replacement boundary or repeated use.

### 5.4 Frontend Architecture Rules

- React Router owns route composition and nested layouts.
- TanStack Query owns remote server state.
- React Hook Form owns edit-form state.
- Zod mirrors client-side form constraints for immediate feedback; the server remains authoritative.
- Generated OpenAPI types and client functions are the only API wire types used by features.
- Feature code does not construct API paths manually.
- Localization keys replace hardcoded interface text.
- CSS Modules own component styles.
- Global CSS contains only reset rules, typography, semantic tokens, and base element behavior.
- Global application state is limited to session presentation, localization, and theme readiness.
- No Redux store or component framework is added.

## 6. Configuration Contract

### 6.1 Normalization Rules

- `AUSE_BASE_PATH` MUST begin and end with `/`.
- Root deployment is represented as `/`.
- Double slashes are rejected rather than silently normalized.
- `AUSE_PUBLIC_URL` MUST be an absolute HTTPS URL in production.
- Secret values MUST NOT appear in logs.
- Production startup fails when required configuration is absent or unsafe.
- Development defaults are permitted only when `AUSE_ENV=development`.

### 6.2 Backend Environment Variables

| Variable | Required | Default | Validation and purpose |
| --- | --- | --- | --- |
| `AUSE_ENV` | Yes | `development` outside production | `development`, `test`, or `production`. |
| `AUSE_HTTP_ADDR` | No | `:8080` | Go listen address. |
| `AUSE_PUBLIC_URL` | Production | none | Absolute external application URL including base path. |
| `AUSE_BASE_PATH` | Yes | `/ause-discovery/` | Shared normalized base path. |
| `AUSE_DATABASE_URL` | Yes | none | PostgreSQL DSN; secret. |
| `AUSE_DATABASE_MAX_CONNS` | No | `10` | Positive integer sized for the VM. |
| `AUSE_MEILISEARCH_URL` | Yes | `http://meilisearch:7700` | Internal service URL. |
| `AUSE_MEILISEARCH_API_KEY` | Yes | none | Backend-only API key; secret. |
| `AUSE_MEILISEARCH_INDEX_UID` | No | `projects` | Stable serving index UID used in index-swap operations. |
| `AUSE_ARTIFACT_ROOT` | Yes | `/var/lib/ause-discovery/artifacts` | Absolute storage root outside web root. |
| `AUSE_MAX_ARTIFACT_BYTES` | No | `262144000` | Maximum file size; value may only decrease without review. |
| `AUSE_MAX_PROJECT_ARTIFACT_BYTES` | No | `2147483648` | Maximum active bytes per Project. |
| `AUSE_IMPORT_PREVIEW_TTL` | No | `24h` | Import preview lifetime. |
| `AUSE_SESSION_IDLE_TTL` | No | `30m` | Server-side idle timeout. |
| `AUSE_SESSION_ABSOLUTE_TTL` | No | `8h` | Server-side absolute timeout. |
| `AUSE_SESSION_COOKIE_NAME` | No | `ause_discovery_session` | Opaque session cookie name. |
| `AUSE_COOKIE_SECURE` | Production | `true` | Must be true in production. |
| `AUSE_LOGIN_MAX_FAILURES` | No | `5` | Failure threshold per key. |
| `AUSE_LOGIN_FAILURE_WINDOW` | No | `15m` | Rate-limit window. |
| `AUSE_SEARCH_RETRY_MIN` | No | `5s` | Initial retry delay. |
| `AUSE_SEARCH_RETRY_MAX` | No | `5m` | Maximum retry delay. |
| `AUSE_SEARCH_RECONCILE_INTERVAL` | No | `30s` | Pending-project scan interval. |
| `AUSE_FEATURED_PROJECT_IDS` | No | empty | Comma-separated UUID list. |
| `AUSE_DEVELOPER_CREDIT` | No | empty | Footer text. |
| `AUSE_DEVELOPER_GITHUB_URL` | No | empty | Absolute HTTPS URL. |
| `AUSE_ALLOWED_DEV_ORIGIN` | Development | `http://localhost:5173` | Exact development origin. |

### 6.3 Frontend Build Variables

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `VITE_AUSE_BASE_PATH` | Yes | `/ause-discovery/` | Vite asset base and React Router basename. |
| `VITE_AUSE_API_PATH` | No | `api/v1` | Relative API segment beneath the base path. |
| `VITE_AUSE_PRODUCT_NAME` | No | `AUSE Discovery` | Public product label. |

Frontend production configuration contains no secrets. Runtime branding and featured-project data are returned through a public runtime-configuration endpoint when values require post-build changes.

### 6.4 Startup Validation

Production startup MUST fail for:

- missing database or Meilisearch credentials;
- non-HTTPS public URL;
- insecure cookie configuration;
- invalid base path;
- relative artifact root;
- artifact root equal to or above the application source directory;
- non-positive connection or size limits;
- idle session timeout greater than absolute timeout;
- malformed featured Project UUIDs;
- database migration version newer than the application-supported version.

Meilisearch unavailability does not block API startup after configuration validation. PostgreSQL unavailability blocks readiness.

## 7. Database Design

### 7.1 General Conventions

- PostgreSQL 18.6 is authoritative.
- Table and column names use `snake_case`.
- UUIDs are generated in the Go application with UUIDv7.
- Timestamps use `timestamptz` in UTC.
- All mutable records include `created_at` and `updated_at`.
- Business status values use `text` plus `CHECK` constraints rather than PostgreSQL enum types.
- Foreign-key columns receive indexes.
- User-provided text is stored as supplied after Unicode and whitespace normalization; a separate normalized value supports matching where required.
- Empty strings are rejected or normalized to `NULL` according to field optionality.
- Physical cascade deletion is not used for archival business records.
- Migrations are forward-only in production. Down migrations exist only when safe and are never the production rollback contract.

### 7.2 Projects

#### `projects`

| Column | Type | Null | Constraint or meaning |
| --- | --- | --- | --- |
| `id` | `uuid` | No | Primary key; application-generated UUIDv7. |
| `reference_code` | `text` | Yes | Trimmed external value; not unique. |
| `title` | `text` | No | 1 to 300 Unicode code points after trimming. |
| `abstract` | `text` | No | 1 to 10000 Unicode code points after trimming. |
| `academic_year` | `smallint` | No | Between 1900 and 2200. |
| `semester` | `text` | No | `1`, `2`, or `summer`. |
| `program_version_id` | `uuid` | Yes | Exact historical Program Version; required for publication. |
| `major_version_id` | `uuid` | Yes | Exact historical Major Version; conditionally required for publication. |
| `course_version_id` | `uuid` | Yes | Exact historical Course Version; required for publication. |
| `status` | `text` | No | `draft`, `published`, or `deleted`. |
| `status_before_delete` | `text` | Yes | `draft` or `published`; populated only while deleted. |
| `extra_metadata` | `jsonb` | No | Default `{}`; extension data only. |
| `revision` | `bigint` | No | Starts at 1; increments on each successful mutation. |
| `published_at` | `timestamptz` | Yes | First successful publication timestamp. |
| `deleted_at` | `timestamptz` | Yes | Current soft-deletion timestamp. |
| `created_at` | `timestamptz` | No | Creation timestamp. |
| `updated_at` | `timestamptz` | No | Last mutation timestamp. |

Project constraints:

- `deleted_at` and `status_before_delete` are both non-null exactly when `status='deleted'`.
- `published_at` is non-null after first publication and remains populated after later deletion.
- `major_version_id` must belong to the selected Program Version when populated.
- `course_version_id` must be valid for the selected Program Version and academic year when populated.
- `program_version_id`, `major_version_id`, and `course_version_id` may be null only while a Project remains an incomplete draft.
- `extra_metadata` must be a JSON object and must not contain reserved canonical field names.

Indexes:

- `projects(status, academic_year desc, semester)`;
- `projects(reference_code)` with a normal non-unique B-tree index;
- `projects(program_version_id)`;
- `projects(major_version_id)`;
- `projects(course_version_id)`;
- `projects(updated_at)`;
- partial index on published Projects;
- partial index on deleted Projects for administrator filtering.

#### `project_title_aliases`

| Column | Type | Null | Constraint or meaning |
| --- | --- | --- | --- |
| `id` | `uuid` | No | Primary key. |
| `project_id` | `uuid` | No | Project foreign key. |
| `alias` | `text` | No | 1 to 300 Unicode code points. |
| `normalized_alias` | `text` | No | Matching and duplicate detection. |
| `sort_order` | `integer` | No | Zero-based display order. |
| `created_at` | `timestamptz` | No | Creation timestamp. |

Unique constraints prevent duplicate normalized aliases within one Project and duplicate sort positions within one Project.

### 7.3 People and Participation

#### `people`

| Column | Type | Null | Constraint or meaning |
| --- | --- | --- | --- |
| `id` | `uuid` | No | Primary key; UUIDv7. |
| `display_name` | `text` | No | 1 to 200 Unicode code points. |
| `normalized_name` | `text` | No | Unicode-normalized, case-folded, whitespace-collapsed matching value. |
| `student_id` | `text` | Yes | Exactly seven ASCII digits; public. |
| `staff_id` | `text` | Yes | Optional institutional identifier; 1 to 100 characters. |
| `revision` | `bigint` | No | Optimistic concurrency value. |
| `created_at` | `timestamptz` | No | Creation timestamp. |
| `updated_at` | `timestamptz` | No | Mutation timestamp. |

Constraints and indexes:

- unique partial index on `student_id` where non-null;
- unique partial index on normalized `staff_id` where non-null;
- index on `normalized_name`;
- a Person may hold student and staff identifiers at the same time;
- no permanent person-type column exists.

#### `project_participations`

| Column | Type | Null | Constraint or meaning |
| --- | --- | --- | --- |
| `id` | `uuid` | No | Primary key. |
| `project_id` | `uuid` | No | Project foreign key. |
| `person_id` | `uuid` | No | Person foreign key. |
| `role` | `text` | No | `student`, `advisor`, `co_advisor`, or `committee_member`. |
| `sort_order` | `integer` | No | Zero-based order within role. |
| `created_at` | `timestamptz` | No | Creation timestamp. |

Constraints:

- unique `(project_id, person_id, role)`;
- unique `(project_id, role, sort_order)`;
- at least one student and one advisor are required before publication;
- duplicate participation roles for the same Person and Project are prohibited;
- a Person may participate in different roles across different Projects.

### 7.4 Academic Catalog

Academic catalog identity and historical display are separated.

#### Stable identity tables

- `programs(id, key, created_at)`
- `majors(id, key, program_id, created_at)`
- `courses(id, key, created_at)`

Stable keys use lower-case ASCII snake case and never change meaning.

#### Version tables

- `program_versions(id, program_id, label_en, valid_from_year, valid_to_year, retired_at, created_at)`
- `major_versions(id, major_id, label_en, valid_from_year, valid_to_year, retired_at, created_at)`
- `course_versions(id, course_id, code, label_en, valid_from_year, valid_to_year, retired_at, created_at)`
- `course_program_versions(course_version_id, program_version_id)`

Version rules:

- `valid_to_year` is inclusive and nullable.
- overlapping validity ranges for one stable identity are prohibited.
- historical labels are not rewritten for branding changes; a new version is added.
- correction of an objective spelling error requires an audited seed correction.
- a Project references exact version rows.

#### Academic seed format

The academic seed file uses this structural shape:

```yaml
schema_version: 1
programs:
  - id: fixed-uuid
    key: computer_science
    versions:
      - id: fixed-uuid
        label:
          en: Computer Science
        valid_from_year: 2020
        valid_to_year: null
    majors:
      - id: fixed-uuid
        key: software_engineering
        versions:
          - id: fixed-uuid
            label:
              en: Software Engineering
            valid_from_year: 2020
            valid_to_year: null
courses:
  - id: fixed-uuid
    key: senior_project
    versions:
      - id: fixed-uuid
        code: CS4900
        label:
          en: Senior Project
        valid_from_year: 2020
        valid_to_year: null
        program_version_ids:
          - fixed-program-version-uuid
```

Every seed identity and version contains a fixed committed UUID so database environments produce identical references. Seed synchronization rules:

- validate the complete file before opening a mutation transaction;
- insert missing identities and versions;
- update only fields permitted by the applicable historical-label rule;
- require explicit `retired_at` metadata to retire an identity or version;
- never infer deletion from a missing YAML entry;
- reject changed keys for an existing fixed UUID;
- reject reused keys with a different fixed UUID;
- record a catalog synchronization audit event with counts and source checksum.

### 7.5 Taxonomy

#### `taxonomy_values`

| Column | Type | Null | Constraint or meaning |
| --- | --- | --- | --- |
| `id` | `uuid` | No | Primary key. |
| `dimension` | `text` | No | `category`, `platform`, `domain`, `topic`, or `technology`. |
| `key` | `text` | No | Stable lower-case snake-case identifier. |
| `labels` | `jsonb` | No | Localized label object containing at least `en`. |
| `description` | `text` | Yes | Administrator guidance. |
| `sort_order` | `integer` | No | Display order within dimension. |
| `retired_at` | `timestamptz` | Yes | New assignments blocked when populated. |
| `created_at` | `timestamptz` | No | Creation timestamp. |
| `updated_at` | `timestamptz` | No | Seed synchronization timestamp. |

Unique constraints apply to `(dimension, key)` and `(dimension, sort_order)`.

#### `project_taxonomy_values`

| Column | Type | Null | Constraint or meaning |
| --- | --- | --- | --- |
| `project_id` | `uuid` | No | Project foreign key. |
| `taxonomy_value_id` | `uuid` | No | Taxonomy Value foreign key. |
| `sort_order` | `integer` | No | Order within dimension. |
| `created_at` | `timestamptz` | No | Assignment timestamp. |

Primary key is `(project_id, taxonomy_value_id)`. Publication requires at least one category and one platform. Domain, topic, and technology assignments may be empty.

#### Initial taxonomy seed

The initial seed contains these exact stable keys and English labels:

| Dimension | Key | English label |
| --- | --- | --- |
| category | `software_application` | Software / Application |
| category | `ai_data` | AI / Data |
| category | `research` | Research |
| category | `game` | Game |
| category | `hardware_iot` | Hardware / IoT |
| platform | `web` | Web |
| platform | `mobile` | Mobile |
| platform | `desktop` | Desktop |
| platform | `embedded_hardware` | Embedded / Hardware |
| platform | `game` | Game |
| platform | `other` | Other |
| domain | `education` | Education |
| domain | `business` | Business |
| domain | `ecommerce` | E-commerce |
| domain | `healthcare` | Healthcare |
| domain | `transportation` | Transportation |
| domain | `entertainment` | Entertainment |
| domain | `security` | Security |
| domain | `finance` | Finance |
| domain | `campus` | Campus |
| domain | `other` | Other |
| topic | `computer_vision` | Computer Vision |
| topic | `ocr` | OCR |
| topic | `nlp` | NLP |
| topic | `gamification` | Gamification |
| topic | `geolocation` | Geolocation |
| technology | `react` | React |
| technology | `go` | Go |
| technology | `postgresql` | PostgreSQL |
| technology | `opencv` | OpenCV |
| technology | `unity` | Unity |
| technology | `python` | Python |
| technology | `firebase` | Firebase |

Additional values require a reviewed seed-file change. `other` is not added to topic or technology because assignments in those dimensions must remain meaningful.

The taxonomy seed uses fixed committed UUIDs, stable dimension keys, stable value keys, localized labels, descriptions, sort order, and optional retirement timestamps. Synchronization follows the same validate-then-transaction pattern as the academic seed. Missing entries are not deleted or retired implicitly.

### 7.6 Artifacts

#### `artifacts`

| Column | Type | Null | Constraint or meaning |
| --- | --- | --- | --- |
| `id` | `uuid` | No | Primary key; UUIDv7. |
| `project_id` | `uuid` | No | Project foreign key. |
| `type` | `text` | No | Approved artifact type. |
| `display_name` | `text` | No | 1 to 200 Unicode code points. |
| `original_filename` | `text` | No | Sanitized display metadata only. |
| `storage_key` | `text` | No | Application-generated opaque key; unique. |
| `mime_type` | `text` | No | Server-detected MIME type. |
| `file_extension` | `text` | No | Normalized lower-case extension. |
| `size_bytes` | `bigint` | No | Positive and no greater than configured limit. |
| `sha256` | `bytea` | No | Content digest for integrity and duplicate warnings. |
| `status` | `text` | No | `active` or `deleted`. |
| `deleted_at` | `timestamptz` | Yes | Soft-deletion timestamp. |
| `revision` | `bigint` | No | Optimistic concurrency value. |
| `created_by_user_id` | `uuid` | No | Uploading Application User. |
| `created_at` | `timestamptz` | No | Creation timestamp. |
| `updated_at` | `timestamptz` | No | Mutation timestamp. |

Artifact bytes are public only when both Artifact status is `active` and Project status is `published`.

### 7.7 Authentication and Sessions

#### `application_users`

- `id uuid primary key`
- `username text not null`
- `normalized_username text not null unique`
- `display_name text not null`
- `role text not null check role='administrator'`
- `status text not null check status in ('active','disabled')`
- `revision bigint not null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`
- `disabled_at timestamptz null`

#### `local_credentials`

- `user_id uuid primary key`
- `password_hash text not null`
- `password_changed_at timestamptz not null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

The Argon2id encoded hash stores algorithm parameters, salt, and derived hash. Password plaintext never enters logs, audit metadata, or persistent import data.

#### `sessions`

- `id uuid primary key`
- `user_id uuid not null`
- `token_hash bytea not null unique`
- `csrf_token_hash bytea not null`
- `created_at timestamptz not null`
- `last_seen_at timestamptz not null`
- `idle_expires_at timestamptz not null`
- `absolute_expires_at timestamptz not null`
- `revoked_at timestamptz null`
- `created_ip inet null`
- `last_seen_ip inet null`
- `user_agent_hash bytea null`

Raw session and CSRF tokens are returned only to the authenticated client. Database values use SHA-256 token hashes. Session validation uses constant-time comparison.

#### `login_failures`

- `id uuid primary key`
- `username_key text not null`
- `source_ip inet not null`
- `occurred_at timestamptz not null`

Expired failure rows are removed by bounded periodic cleanup. Rate limiting evaluates both username and source address windows.

### 7.8 Audit Events

#### `audit_events`

- `id uuid primary key`
- `event_type text not null`
- `actor_user_id uuid null`
- `target_type text null`
- `target_id uuid null`
- `request_id uuid null`
- `source_ip inet null`
- `metadata jsonb not null default '{}'`
- `occurred_at timestamptz not null`

Audit events are append-only through application permissions. Event metadata contains identifiers and changed-field names, not passwords, tokens, full abstracts, artifact bytes, or imported row payloads.

Required event types include:

- `auth.login_succeeded`
- `auth.login_failed`
- `auth.logout`
- `auth.user_created`
- `auth.password_reset`
- `auth.user_disabled`
- `project.created`
- `project.updated`
- `project.published`
- `project.deleted`
- `project.restored`
- `person.created`
- `person.updated`
- `artifact.uploaded`
- `artifact.deleted`
- `artifact.restored`
- `import.preview_created`
- `import.committed`
- `search.project_reindexed`
- `search.rebuild_started`
- `search.rebuild_completed`
- `search.rebuild_failed`

### 7.9 Search Synchronization State

#### `project_search_sync`

- `project_id uuid primary key`
- `desired_revision bigint not null`
- `indexed_revision bigint null`
- `desired_action text not null check desired_action in ('upsert','remove')`
- `state text not null check state in ('pending','processing','synced','failed')`
- `attempt_count integer not null default 0`
- `next_attempt_at timestamptz not null`
- `lease_owner text null`
- `lease_expires_at timestamptz null`
- `last_error_code text null`
- `last_error_message text null`
- `last_attempt_at timestamptz null`
- `synced_at timestamptz null`
- `updated_at timestamptz not null`

The row is inserted or updated in the same transaction as each searchable Project mutation. Error messages are bounded and sanitized. Leases allow recovery after process interruption without distributed queue infrastructure.

#### `search_rebuild_operations`

- `id uuid primary key`
- `initiated_by_user_id uuid null`
- `state text not null`
- `target_index_name text not null`
- `source_index_name text null`
- `total_projects bigint not null default 0`
- `processed_projects bigint not null default 0`
- `started_at timestamptz null`
- `cutover_started_at timestamptz null`
- `completed_at timestamptz null`
- `last_error_code text null`
- `last_error_message text null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

A unique partial index permits only one operation in an active state. Failure messages are bounded and sanitized.

### 7.10 Import Persistence

#### `import_batches`

- `id uuid primary key`
- `created_by_user_id uuid not null`
- `source_format text not null check source_format in ('csv','xlsx')`
- `original_filename text not null`
- `source_sha256 bytea not null`
- `source_storage_key text null`
- `state text not null check state in ('previewing','ready','committing','committed','expired','failed')`
- `total_rows integer not null`
- `valid_rows integer not null`
- `warning_rows integer not null`
- `invalid_rows integer not null`
- `selected_rows integer not null`
- `expires_at timestamptz not null`
- `committed_at timestamptz null`
- `commit_result jsonb null`
- `revision bigint not null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

#### `import_rows`

- `id uuid primary key`
- `batch_id uuid not null`
- `row_number integer not null`
- `import_key text not null`
- `canonical_draft jsonb not null`
- `issues jsonb not null`
- `validation_state text not null check validation_state in ('valid','warning','invalid')`
- `selected boolean not null default false`
- `warning_acknowledged boolean not null default false`
- `duplicate_resolution jsonb null`
- `committed_project_id uuid null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Unique constraints apply to `(batch_id, row_number)` and `(batch_id, import_key)`.

Import preview source bytes are removed after parsing. Canonical drafts and issues remain until batch expiration. Expired batches retain only summary audit data after cleanup.

### 7.11 Migration Order

Initial migrations are ordered by dependency:

1. extensions and shared timestamp helpers, if any;
2. academic catalog stable identities and versions;
3. taxonomy values;
4. people;
5. projects and title aliases;
6. participations and taxonomy assignments;
7. application users and local credentials;
8. sessions and login failures;
9. artifacts;
10. audit events;
11. search synchronization and rebuild-operation state;
12. import batches and rows;
13. initial catalog and taxonomy seed application markers.

Migrations MUST NOT contact Meilisearch or the artifact filesystem.

## 8. Domain Validation and Mutation Semantics

### 8.1 Text Normalization

Canonical user-visible text normalization performs these operations in order:

1. reject invalid UTF-8;
2. normalize Unicode to NFC;
3. convert CRLF and CR line endings to LF;
4. trim leading and trailing Unicode whitespace;
5. collapse internal whitespace for identifiers and names only;
6. preserve meaningful internal whitespace and paragraphs for titles and abstracts.

Matching normalization additionally applies Unicode case folding and whitespace collapse. Matching normalization never replaces stored display text.

### 8.2 Draft Validation

A draft Project requires:

- title;
- abstract;
- academic year;
- semester.

Program, major, course, people, taxonomy, and artifacts may remain incomplete in a draft. Database catalog foreign keys are nullable for this reason. When present, catalog relationships MUST be internally compatible.

### 8.3 Publication Validation

Publication requires all of the following:

- non-empty title and abstract;
- academic year and semester;
- Program Version;
- Course Version compatible with the Program Version and academic year;
- Major Version compatible with the Program Version when the Program defines majors;
- at least one student participation;
- every student participation has a valid student ID;
- at least one advisor participation;
- at least one category;
- at least one platform;
- no retired catalog or taxonomy assignment that became retired before the Project academic year;
- no unresolved duplicate warning created by the active edit or import workflow.

Artifacts are not required for publication. A published Project with no artifacts remains valid and searchable.

### 8.4 Optimistic Concurrency

Every administrator mutation of a Project or Project child collection requires `expected_revision`.

The following operations increment the Project revision:

- scalar metadata update;
- title-alias replacement;
- participation replacement;
- taxonomy-assignment replacement;
- publication;
- deletion;
- restoration;
- artifact upload;
- artifact metadata update;
- artifact deletion;
- artifact restoration.

The transaction locks the Project row, compares `expected_revision`, applies the complete mutation, increments the revision once, records the audit event, and marks search synchronization pending when public search output can change.

Searchable mutations acquire the shared searchable-mutation advisory lock for the transaction. Normal operation uses shared acquisition. Search rebuild cutover uses exclusive acquisition for a short final-catch-up window.

A mismatch returns HTTP `409` with code `stale_revision`, the submitted revision, and the current revision. Automatic retry of a stale administrator mutation is prohibited.

### 8.5 Collection Replacement

Participation, title-alias, and taxonomy edits use full collection replacement rather than item-by-item mutation.

Collection replacement rules:

- the request contains the expected Project revision;
- the complete desired collection is validated before mutation;
- duplicate entries are rejected;
- all collection changes and the Project revision update occur in one transaction;
- deterministic `sort_order` values are stored from request order;
- publication validity is rechecked for published Projects;
- removal that would invalidate a published Project is rejected with `project_publication_requirements_not_met`.

### 8.6 Duplicate Person Rules

- Exact student ID match resolves to the existing Person.
- Exact normalized staff ID match resolves to the existing Person.
- Normalized name alone never causes automatic Person merging.
- Same-name records without identifiers create a duplicate warning.
- Manual entry requires explicit existing-Person selection or explicit new-Person creation after a name warning.
- Import preview requires a per-participation resolution when a same-name candidate exists without an authoritative identifier.
- Person merge tooling is outside MVP scope.

### 8.7 Project Duplicate Rules

Reference code does not establish uniqueness.

Duplicate candidates are produced by these rules:

| Rule | Severity | Reason |
| --- | --- | --- |
| Exact normalized title, academic year, and semester | Warning | Strong project-level similarity. |
| Same student IDs and academic year with similar normalized title | Warning | Strong team and period similarity. |
| Exact non-empty reference code | Warning | External code may not be unique. |
| Similar title without matching year or people | Information only | Insufficient evidence for a duplicate block. |

No rule automatically merges Projects. An import row duplicate resolution is `create_new`, `match_existing_project`, or `skip`.

`match_existing_project` converts the import row into an update only when the administrator has project-edit permission and supplies the current target Project revision. Otherwise the resolution is invalid.

## 9. OpenAPI and HTTP Contract

### 9.1 Contract Ownership

`api/openapi.yaml` is the source of truth for public HTTP paths, methods, parameters, request bodies, response bodies, error shapes, and security declarations.

Generation outputs:

- Go request, response, and shared schema types;
- TypeScript types and typed client functions;
- no generated Go business handlers;
- no handwritten duplicate TypeScript wire interfaces.

CI performs OpenAPI validation, generation, compilation, and generated-diff checks.

### 9.2 URL Construction

The default production API root is:

```text
/ause-discovery/api/v1
```

The root-deployment API root is:

```text
/api/v1
```

API paths in the OpenAPI document are relative to `/api/v1`. Nginx and application base-path configuration prepend the deployment base path exactly once.

### 9.3 Media Types and Encoding

- JSON requests and responses use `application/json; charset=utf-8`.
- Problem Details responses use `application/problem+json`.
- Artifact and import uploads use `multipart/form-data`.
- CSV templates use `text/csv; charset=utf-8`.
- XLSX templates use the official XLSX MIME type.
- Timestamps use RFC 3339 with UTC `Z` output.
- UUIDs use lower-case canonical string form.
- Unknown JSON properties are rejected for mutation requests.
- Request bodies larger than endpoint-specific limits are rejected before decoding.

### 9.4 Request Correlation

- Nginx accepts or creates `X-Request-ID` only when the value is a valid UUID.
- The Go API creates a UUIDv7 request ID when a valid value is absent.
- The response always includes `X-Request-ID`.
- Structured logs and audit events include the same request ID.
- Client-supplied invalid request IDs are ignored and replaced.

### 9.5 Problem Details

All non-download error responses follow this shape:

```json
{
  "type": "https://ause-discovery.invalid/problems/validation_failed",
  "title": "Validation failed",
  "status": 422,
  "detail": "One or more fields are invalid.",
  "instance": "/ause-discovery/api/v1/admin/projects",
  "code": "validation_failed",
  "request_id": "0198...",
  "field_errors": [
    {
      "field": "student_id",
      "code": "invalid_student_id",
      "message": "Student ID must contain exactly seven digits."
    }
  ]
}
```

Required stable error codes include:

- `validation_failed`
- `unauthenticated`
- `session_expired`
- `csrf_failed`
- `forbidden`
- `not_found`
- `stale_revision`
- `conflict`
- `duplicate_resolution_required`
- `project_publication_requirements_not_met`
- `import_not_ready`
- `import_expired`
- `artifact_type_mismatch`
- `artifact_limit_exceeded`
- `search_unavailable`
- `rate_limited`
- `internal_error`

Internal stack traces, SQL details, filesystem paths, Meilisearch task details, and secrets never appear in responses.

### 9.6 Pagination

List and search endpoints use:

- `page`: one-based integer, default `1`;
- `page_size`: integer, default `20`, maximum `100`.

List responses contain:

```json
{
  "items": [],
  "page": 1,
  "page_size": 20,
  "total_items": 0,
  "total_pages": 0
}
```

Out-of-range pages return an empty item list with correct totals. Negative or zero values return validation errors.

### 9.7 Public Endpoints

| Method | Path | Success | Purpose |
| --- | --- | --- | --- |
| `GET` | `/runtime-config` | `200` | Product label, base path, footer configuration, and supported features. |
| `GET` | `/catalogs` | `200` | Active and historically referenced academic and taxonomy values. |
| `GET` | `/search/projects` | `200` | Text search, browse, filters, sorting, facets, and pagination. |
| `GET` | `/projects/{project_id}` | `200` | Published Project detail. |
| `GET` | `/people/{person_id}` | `200` | Public Person detail and published Project associations. |
| `GET` | `/artifacts/{artifact_id}/view` | `200` or `206` | Inline PDF response only. |
| `GET` | `/artifacts/{artifact_id}/download` | `200` or `206` | Attachment download for active public Artifact. |
| `GET` | `/import-templates/projects.csv` | `200` | Current CSV import template for administrator preparation. |
| `GET` | `/import-templates/projects.xlsx` | `200` | Current XLSX import template for administrator preparation. |

Import templates contain no private data and may be public. Draft and deleted Project IDs return `404` on public endpoints, including direct UUID requests.

### 9.8 Search Query Parameters

`GET /search/projects` accepts:

| Parameter | Type | Semantics |
| --- | --- | --- |
| `q` | string | Optional trimmed query, maximum 300 code points. Empty means browse. |
| `page` | integer | One-based page. |
| `page_size` | integer | Default 20, maximum 100. |
| `sort` | string | `relevance`, `newest`, `oldest`, or `title_asc`. |
| `year` | integer array | OR within the dimension. |
| `semester` | string array | OR within the dimension. |
| `program` | key array | OR within the dimension. |
| `major` | key array | OR within the dimension. |
| `course` | key array | OR within the dimension. |
| `advisor_id` | UUID array | OR within the dimension. |
| `student_id` | seven-digit string array | OR within the dimension. |
| `category` | key array | OR within the dimension. |
| `platform` | key array | OR within the dimension. |
| `domain` | key array | OR within the dimension. |
| `topic` | key array | OR within the dimension. |
| `technology` | key array | OR within the dimension. |
| `has_report` | boolean | Exact artifact-presence filter. |
| `has_slides` | boolean | Exact artifact-presence filter. |
| `has_source_code` | boolean | Exact artifact-presence filter. |
| `has_dataset` | boolean | Exact artifact-presence filter. |

Repeated query parameters represent arrays. OR applies within one dimension. AND applies across dimensions.

Default sort behavior:

- non-empty `q`: `relevance`;
- empty `q`: `newest`;
- explicit `relevance` with empty `q` normalizes to `newest`.

### 9.9 Public Resource Shapes

#### Project search item

Contains:

- Project ID;
- reference code;
- title;
- matched title alias when relevant;
- concise abstract excerpt;
- academic year and semester;
- Program, Major, and Course labels;
- students with public student IDs;
- advisors and co-advisors;
- selected taxonomy values;
- artifact-presence booleans;
- optional formatted match metadata from Meilisearch.

Search items do not contain `extra_metadata`, audit data, revision, storage keys, deleted records, or administrator notes.

#### Public Project detail

Contains:

- canonical metadata and title aliases;
- all public Project Participations grouped by role;
- all taxonomy assignments grouped by dimension;
- active Artifact summaries;
- publication timestamp;
- no administrator revision or internal timestamps other than publication context.

#### Public Person detail

Contains:

- Person ID;
- display name;
- student ID and staff ID when present;
- published Project associations grouped by role;
- Project title, year, semester, reference code, and Project ID for each association.

### 9.10 Administrator Authentication Endpoints

| Method | Path | Success | Notes |
| --- | --- | --- | --- |
| `POST` | `/admin/auth/login` | `200` | Sets session cookie and returns session view plus CSRF token. |
| `GET` | `/admin/auth/session` | `200` | Returns current user, expiry values, permissions, and CSRF token. |
| `POST` | `/admin/auth/logout` | `204` | Revokes session and clears cookie. |

Login uses username and password JSON fields. Login response and current-session response use `Cache-Control: no-store`.

The frontend keeps the CSRF token in memory. Local storage, session storage, IndexedDB, and URL parameters are prohibited for credentials, session identifiers, and CSRF tokens.

### 9.11 Administrator Project Endpoints

| Method | Path | Success | Purpose |
| --- | --- | --- | --- |
| `GET` | `/admin/projects` | `200` | Paginated filtering across all statuses. |
| `POST` | `/admin/projects` | `201` | Create draft Project. |
| `GET` | `/admin/projects/{project_id}` | `200` | Complete editable Project view. |
| `PATCH` | `/admin/projects/{project_id}` | `200` | Update scalar metadata and aliases. |
| `PUT` | `/admin/projects/{project_id}/participations` | `200` | Replace all participations. |
| `PUT` | `/admin/projects/{project_id}/classifications` | `200` | Replace all taxonomy assignments. |
| `POST` | `/admin/projects/{project_id}/publish` | `200` | Validate and publish. |
| `POST` | `/admin/projects/{project_id}/delete` | `200` | Soft-delete after explicit confirmation. |
| `POST` | `/admin/projects/{project_id}/restore` | `200` | Restore to pre-delete status. |

Delete request fields:

- `expected_revision`;
- `confirmation_value`.

`confirmation_value` must equal the Project reference code when present. When reference code is absent, the value must equal the full canonical Project title after exact trimming. Comparison is case-sensitive.

### 9.12 Administrator Person Endpoints

| Method | Path | Success | Purpose |
| --- | --- | --- | --- |
| `GET` | `/admin/people` | `200` | Search and paginate People. |
| `POST` | `/admin/people` | `201` | Create Person. |
| `GET` | `/admin/people/{person_id}` | `200` | Editable Person and all associations. |
| `PATCH` | `/admin/people/{person_id}` | `200` | Update identity fields with revision. |

Person deletion and automatic Person merge are outside MVP scope. A Person with an incorrect duplicate record may remain unused after Project participation correction.

### 9.13 Administrator Artifact Endpoints

| Method | Path | Success | Purpose |
| --- | --- | --- | --- |
| `POST` | `/admin/projects/{project_id}/artifacts` | `201` | Stream multipart upload. |
| `PATCH` | `/admin/artifacts/{artifact_id}` | `200` | Update display name or type when file type remains compatible. |
| `POST` | `/admin/artifacts/{artifact_id}/delete` | `200` | Soft-delete Artifact. |
| `POST` | `/admin/artifacts/{artifact_id}/restore` | `200` | Restore after quota and compatibility validation. |
| `POST` | `/admin/artifacts/{artifact_id}/replace` | `201` | Create replacement record and soft-delete prior Artifact atomically. |

Upload multipart fields are:

- `expected_project_revision`: decimal integer text;
- `type`: artifact type;
- `display_name`: display label;
- `file`: exactly one binary file.

Browser-provided MIME type is advisory only.

### 9.14 Administrator Import Endpoints

| Method | Path | Success | Purpose |
| --- | --- | --- | --- |
| `POST` | `/admin/imports` | `202` | Store source temporarily and start preview parsing. |
| `GET` | `/admin/imports/{batch_id}` | `200` | Batch status and summary. |
| `GET` | `/admin/imports/{batch_id}/rows` | `200` | Paginated preview rows and issues. |
| `PATCH` | `/admin/imports/{batch_id}/rows` | `200` | Apply selection, acknowledgement, and duplicate resolutions. |
| `POST` | `/admin/imports/{batch_id}/commit` | `200` | Commit selected rows atomically. |
| `GET` | `/admin/imports/{batch_id}/result` | `200` | Stable result after commit. |

Import upload accepts exactly one `file` part. Accepted filename extensions are `.csv` and `.xlsx`. Content detection and parser validation determine the actual format.

Patch-row requests contain `expected_batch_revision` and a bounded array of row updates. Commit requests contain `expected_batch_revision`. A successful repeated commit returns the stored original result.

### 9.15 Administrator Search and Audit Endpoints

| Method | Path | Success | Purpose |
| --- | --- | --- | --- |
| `GET` | `/admin/search/status` | `200` | Counts of pending, failed, and synced Projects plus active rebuild. |
| `POST` | `/admin/search/projects/{project_id}/reindex` | `202` | Mark Project pending immediately. |
| `POST` | `/admin/search/rebuilds` | `202` | Start full versioned rebuild. |
| `GET` | `/admin/search/rebuilds/{operation_id}` | `200` | Progress and failure details. |
| `GET` | `/admin/audit-events` | `200` | Filtered, read-only audit listing. |

Only one full rebuild may run at a time. A concurrent start returns `409 search_rebuild_already_running` and the active operation ID.

### 9.16 Operator CLI Contract

The `ausectl` binary uses the same configuration loader and application services as the API.

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

Passwords are collected through a non-echoing terminal prompt. Password command-line flags and environment variables are prohibited.

## 10. Search Contract

### 10.1 Index Identity

- Stable serving index UID: `projects` by default.
- Rebuild index UID: `projects_rebuild_<schema_version>_<timestamp>`.
- Primary key: `id`.
- Index schema version is a source-controlled integer constant.
- Schema-setting changes require an index schema version increment and full rebuild.

### 10.2 Search Document

Each published Project produces one document with these fields:

```json
{
  "id": "project-uuid",
  "revision": 14,
  "reference_code": "26037",
  "title": "Game Realism through Graphics and Physics",
  "title_aliases": [],
  "abstract": "...",
  "academic_year": 2026,
  "semester": "1",
  "program_keys": ["computer_science"],
  "program_labels": ["Computer Science"],
  "major_keys": [],
  "major_labels": [],
  "course_keys": ["senior_project"],
  "course_codes": ["CS4900"],
  "course_labels": ["Senior Project"],
  "student_person_ids": ["person-uuid"],
  "student_names": ["Student Name"],
  "student_ids": ["0123456"],
  "advisor_person_ids": ["person-uuid"],
  "advisor_names": ["Advisor Name"],
  "co_advisor_person_ids": [],
  "co_advisor_names": [],
  "committee_person_ids": [],
  "committee_names": [],
  "category_keys": ["game"],
  "category_labels": ["Game"],
  "platform_keys": ["game"],
  "platform_labels": ["Game"],
  "domain_keys": ["entertainment"],
  "domain_labels": ["Entertainment"],
  "topic_keys": ["physics_simulation"],
  "topic_labels": ["Physics Simulation"],
  "technology_keys": ["unreal_engine"],
  "technology_labels": ["Unreal Engine"],
  "artifact_types": ["report", "slides"],
  "has_report": true,
  "has_slides": true,
  "has_source_code": false,
  "has_dataset": false,
  "published_at": "2026-09-01T00:00:00Z",
  "updated_at": "2026-09-01T00:00:00Z"
}
```

The search document never contains storage keys, artifact filenames, session data, import data, audit data, `extra_metadata`, deleted Project data, or file contents.

### 10.3 Searchable Attributes Order

Searchable attributes are ordered:

1. `student_ids`;
2. `reference_code`;
3. `title`;
4. `title_aliases`;
5. `student_names`;
6. `advisor_names`;
7. `co_advisor_names`;
8. `committee_names`;
9. taxonomy labels;
10. taxonomy keys;
11. Program, Major, Course codes and labels;
12. `abstract`.

Exact identifier matching is enforced in API query handling. A query consisting of exactly seven ASCII digits first applies the strict expression `student_ids = value OR reference_code = value`. Human-readable text search remains available only when that exact expression has no result. Typo tolerance is disabled for student IDs and reference-code filter paths.

### 10.4 Filterable Attributes

Filterable attributes are:

- `academic_year`;
- `semester`;
- Program, Major, and Course keys;
- advisor Person IDs;
- student IDs;
- all taxonomy key arrays;
- `artifact_types`;
- four artifact-presence booleans.

Public filter values are validated against active or historically referenced catalog values before construction of a Meilisearch filter expression.

### 10.5 Sortable Attributes

Sortable attributes are:

- `academic_year`;
- normalized semester order stored in an internal sortable value;
- `title`;
- `published_at`;
- `updated_at`.

`newest` sorts by academic year descending, semester order descending, title ascending. Semester order is `summer`, `2`, `1` for descending within one academic year. `oldest` reverses academic period order and retains title ascending for ties.

### 10.6 Highlighting and Excerpts

- Only title, aliases, names, taxonomy labels, and abstract may return formatted matches.
- Highlight tags use a fixed safe marker converted to React elements without raw HTML injection.
- Abstract excerpts are bounded to 300 display characters.
- Meilisearch formatted strings never enter `dangerouslySetInnerHTML`.

### 10.7 Incremental Synchronization

Search synchronization sequence:

1. application transaction mutates canonical data;
2. transaction increments Project revision;
3. transaction upserts `project_search_sync` with desired revision and action;
4. transaction commits;
5. an immediate best-effort synchronization attempt runs;
6. the reconciler later retries pending or failed work;
7. success updates indexed revision only when desired revision still matches;
8. an out-of-date attempt returns to pending without marking stale data synced.

Retry delay uses exponential backoff with configured minimum and maximum plus bounded jitter. Failure state remains visible to administrators. No external broker is used.

### 10.8 Search Rebuild Operations

The database includes `search_rebuild_operations` with:

- operation ID;
- initiating Application User;
- state `queued`, `configuring`, `indexing`, `catching_up`, `swapping`, `completed`, or `failed`;
- target physical index name;
- source index name;
- total and processed Project counts;
- starting high-water timestamp;
- safe failure code and message;
- created, started, updated, and completed timestamps.

Rebuild sequence:

1. acquire the database rebuild mutex;
2. create a new physical index;
3. apply complete searchable, filterable, sortable, ranking, typo, pagination, and displayed-attribute settings;
4. scan published Projects in stable UUID order using bounded pages;
5. index generated documents and wait for Meilisearch tasks;
6. catch up Projects changed after the starting high-water timestamp;
7. verify document count and sampled document revisions;
8. acquire the short-lived searchable-mutation advisory lock used by Project, participation, taxonomy, and Artifact mutations;
9. run a final catch-up of every changed or deleted Project while new searchable mutations wait;
10. atomically swap the stable serving UID with the rebuild index UID through the Meilisearch swap-indexes operation;
11. mark matching search-sync revisions synced against the new index;
12. release the searchable-mutation advisory lock;
13. retain the previous index for one successful verification window;
14. delete the previous physical index after verification.

After the swap, the stable UID contains new data and the rebuild UID contains prior serving data. The advisory lock is held only for final catch-up and index cutover, never for the initial full scan. Any failure before the swap leaves the serving index unchanged. Any failure after the swap records the new serving state and does not automatically swap back without verification.

### 10.9 Search Failure Behavior

- Search endpoint returns `503 search_unavailable` when Meilisearch is unavailable.
- Project and Person detail remain available from PostgreSQL.
- Administrator writes continue and leave synchronization pending.
- Search retry failures produce structured logs and status counts.
- Meilisearch data loss is recovered through rebuild.
- Missing search documents are not reconstructed manually.

## 11. Artifact Storage and Serving

### 11.1 Storage Interface

The artifacts module defines a narrow interface with operations equivalent to:

```text
Put(stream, expected_size) -> storage_key, size, sha256
Open(storage_key) -> seekable stream, size
Exists(storage_key) -> boolean
```

Permanent deletion is absent from the MVP interface.

### 11.2 Storage Layout

- Storage root is absolute and configuration-controlled.
- Storage key is a random UUID plus a storage-layout version, not a user filename.
- Physical fan-out uses the first two and next two hexadecimal UUID characters.
- Example relative path: `v1/01/9a/0198...`.
- Final paths are resolved and checked for containment beneath the configured root.
- Symbolic links are prohibited in the storage path.
- Files are created with restrictive permissions.
- Upload temporary files use the same filesystem to support atomic rename.

### 11.3 Upload Sequence

1. authenticate and authorize administrator;
2. validate Project revision and active Project state;
3. parse metadata fields before file streaming;
4. enforce request body limit at Nginx and Go layers;
5. stream to a uniquely created temporary file;
6. count bytes and calculate SHA-256 during streaming;
7. reject zero-byte or oversized files;
8. detect MIME from file signatures and bounded content inspection;
9. validate extension and detected MIME against Artifact type;
10. validate active Project quota inside the database transaction;
11. atomically rename temporary file to final storage key;
12. insert Artifact, increment Project revision, mark search pending, and record audit event;
13. remove final file when the database transaction fails after rename;
14. remove temporary files on every failure path.

### 11.4 Type Allowlist

| Artifact type | Allowed extensions | Required detected MIME family | View |
| --- | --- | --- | --- |
| `report` | `.pdf` | PDF | Yes |
| `slides` | `.pdf`, `.ppt`, `.pptx` | PDF or recognized PowerPoint container | PDF only |
| `source_code` | `.zip`, `.tar.gz`, `.tgz` | ZIP or gzip/tar-compatible archive | No |
| `proposal` | `.pdf`, `.docx` | PDF or recognized DOCX container | PDF only |
| `poster` | `.pdf`, `.png`, `.jpg`, `.jpeg` | PDF, PNG, or JPEG | PDF only in MVP |
| `dataset` | `.zip`, `.csv`, `.tsv`, `.json`, `.xlsx` | Recognized archive, text table, JSON, or XLSX container | No |
| `demo_video` | `.mp4`, `.webm` | MP4 or WebM | No embedded player in MVP |
| `other` | `.pdf`, `.docx`, `.pptx`, `.zip`, `.csv`, `.xlsx`, `.json` | Matching allowlisted type | PDF only |

Double extensions are treated according to the final normalized extension and detected content. Executable formats, HTML, SVG, scripts, disk images, and macro-enabled Office formats are rejected.

### 11.5 Quota Semantics

- File limit applies to the incoming active Artifact.
- Project quota sums active Artifact `size_bytes` only.
- Deleted Artifacts do not consume active quota.
- Artifact restoration rechecks Project quota.
- Replacement checks quota as if the prior Artifact becomes deleted in the same transaction.
- Concurrent uploads lock the Project row before final quota validation.

### 11.6 Replacement Semantics

Artifact replacement never overwrites bytes or reuses identity.

One transaction:

1. validates the existing active Artifact and Project revision;
2. creates a new Artifact record and storage key;
3. marks the prior Artifact deleted;
4. increments Project revision once;
5. records both Artifact IDs in the audit event;
6. marks search synchronization pending when artifact facets change.

### 11.7 View and Download

- View is permitted only for PDF.
- View response uses detected `application/pdf`, `Content-Disposition: inline`, `X-Content-Type-Options: nosniff`, and byte-range support.
- Download response uses `Content-Disposition: attachment` with RFC 5987-safe filename encoding.
- Download uses the detected MIME type when safe, otherwise `application/octet-stream`.
- `Accept-Ranges: bytes` is supported for seekable files.
- Nginx buffering does not load complete large files into memory.
- Missing or corrupt files return `404 artifact_content_unavailable` and emit an error log without failing Project detail.
- Public access checks Project and Artifact status before opening the storage file.

## 12. Import Contract

### 12.1 Global Limits

| Limit | Value |
| --- | --- |
| Upload size | 25 MiB |
| Project rows | 5000 |
| Participation rows | 30000 |
| Classification rows | 50000 |
| Sheets | Exactly the required supported sheets for XLSX |
| Cell text | 10000 Unicode code points |
| JSON array items per cell | 200 |
| XLSX expanded content | 250 MiB |
| Parse time | 60 seconds |

Limit violations fail the batch with a stable import error code. Parsing uses bounded memory and row streaming where supported.

### 12.2 CSV Format

CSV requirements:

- UTF-8 encoding;
- optional UTF-8 BOM;
- comma delimiter;
- RFC 4180 quoting;
- one header row;
- exact case-sensitive headers;
- no unknown or duplicate headers;
- one Project per data row;
- blank trailing rows ignored;
- Student IDs represented as JSON strings, preserving leading zeroes.

Exact headers, in order:

```text
import_key
title
reference_code
abstract
academic_year
semester
program_key
major_key
course_key
title_aliases
students
advisors
co_advisors
committee_members
categories
platforms
domains
topics
technologies
```

JSON-array columns use compact or formatted valid JSON. Examples:

```json
["Alternative Project Title"]
```

```json
[
  {
    "display_name": "Student Name",
    "student_id": "0123456"
  }
]
```

```json
[
  {
    "display_name": "Advisor Name",
    "staff_id": "STAFF-001"
  }
]
```

Taxonomy columns contain JSON arrays of stable keys, such as `["software_application","ai_data"]`.

### 12.3 XLSX Format

The workbook contains exactly these visible worksheets:

1. `Projects`
2. `Participations`
3. `Classifications`

Hidden sheets, macros, external workbook links, formulas, data connections, and unsupported sheet names are rejected.

#### `Projects` columns

```text
import_key
title
reference_code
abstract
academic_year
semester
program_key
major_key
course_key
title_aliases_json
```

#### `Participations` columns

```text
import_key
role
display_name
student_id
staff_id
display_order
```

Role values are `student`, `advisor`, `co_advisor`, or `committee_member`. Student ID cells are parsed as displayed strings. Numeric cells that lose leading zeroes produce an error rather than guessed padding.

#### `Classifications` columns

```text
import_key
dimension
taxonomy_key
display_order
```

Dimension values are `category`, `platform`, `domain`, `topic`, or `technology`.

### 12.4 Canonical Import Draft

Both adapters produce the same versioned internal shape:

```json
{
  "schema_version": 1,
  "import_key": "row-001",
  "project": {
    "title": "...",
    "reference_code": null,
    "abstract": "...",
    "academic_year": 2026,
    "semester": "1",
    "program_key": "computer_science",
    "major_key": null,
    "course_key": "senior_project",
    "title_aliases": []
  },
  "participations": [],
  "classifications": []
}
```

Raw cell coordinates are retained separately for issue reporting and are not part of the committed Project.

### 12.5 Validation Issues

Each issue contains:

- severity: `error`, `warning`, or `info`;
- stable code;
- source sheet or CSV row;
- source column or JSON path;
- human-readable message;
- normalized value when a normalization occurred;
- candidate resource summaries when resolution is required.

Representative issue codes:

- `missing_required_field`
- `invalid_student_id`
- `invalid_academic_year`
- `invalid_semester`
- `unknown_catalog_key`
- `retired_catalog_key`
- `unknown_taxonomy_key`
- `duplicate_participation`
- `person_match_required`
- `project_duplicate_candidate`
- `invalid_json_cell`
- `unsupported_formula`
- `import_limit_exceeded`

### 12.6 Preview State Machine

```text
previewing -> ready -> committing -> committed
     |          |          |
     v          v          v
   failed     expired     failed
```

- `previewing` batches are immutable from the UI.
- `ready` batches accept row selection and resolution changes.
- `committing` batches reject concurrent writes.
- `committed` batches return stable results and reject further changes.
- `expired` batches require a new upload.
- `failed` batches expose safe parse failure details.

### 12.7 Commit Transaction

Commit performs:

1. lock Import Batch;
2. verify state, revision, expiry, and administrator identity;
3. load all selected rows;
4. reject invalid, unacknowledged warning, or unresolved duplicate rows;
5. lock existing matched Projects in stable UUID order;
6. revalidate catalog and duplicate decisions against current database state;
7. create or update People without name-only auto-merge;
8. create or update Projects and child collections;
9. increment Project revisions;
10. mark search synchronization pending for published Projects;
11. store committed Project IDs per row;
12. store immutable batch result summary;
13. record one batch audit event plus Project-level events;
14. mark batch committed;
15. commit once.

Any failure rolls back every selected row. Meilisearch is not called inside the import transaction.

### 12.8 Import Cleanup

- Source upload bytes are deleted immediately after successful or failed parsing.
- Abandoned source bytes are deleted when the batch expires.
- Ready batches expire 24 hours after creation.
- Committed result summaries remain for audit visibility.
- Full canonical draft payloads for expired uncommitted batches are deleted.
- Cleanup is bounded per run and safe to repeat.

## 13. Authentication, Authorization, and Security

### 13.1 Password Handling

Password requirements:

- minimum 14 Unicode code points;
- maximum 128 Unicode code points and 1024 UTF-8 bytes;
- no composition-class requirement;
- no trimming or Unicode normalization;
- reject NUL bytes and invalid UTF-8;
- reject a password equal to the normalized username;
- entry occurs only through a non-echoing operator prompt or HTTPS login form.

Argon2id baseline parameters:

- memory: 19 MiB;
- iterations: 2;
- parallelism: 1;
- salt: 16 cryptographically random bytes;
- derived key: 32 bytes.

The encoded hash uses the standard PHC string format. Authentication rehashes a valid password after login when stored parameters are weaker than current configuration. Rehash failure does not invalidate an otherwise valid login, but emits an error and leaves an operational alert.

### 13.2 Administrator Account Operations

Administrator account CRUD is operator-side CLI only.

- Creation requires unique normalized username, display name, and password confirmation.
- Password reset revokes all active sessions in the same transaction.
- Account disablement revokes all active sessions in the same transaction.
- No self-registration, password-reset email, or browser account-management screen exists.
- The final active administrator may be disabled only with `--allow-zero-admins`, a typed confirmation, and an audit event.

### 13.3 Login Flow

1. validate request origin and content type;
2. normalize username for lookup;
3. apply username and source-address rate limits;
4. perform a constant-work password verification path for existing and missing users;
5. return the same generic failure for missing, disabled, or incorrect credentials;
6. generate a 32-byte random session token;
7. generate a 32-byte random CSRF token;
8. store only SHA-256 hashes;
9. set the session cookie;
10. record success or failure audit event;
11. return Application User, permission set, expiry values, and raw CSRF token on success.

Successful login does not accept or preserve a caller-supplied session identifier.

### 13.4 Session Rules

- Idle timeout: 30 minutes.
- Absolute timeout: 8 hours.
- Expiration is enforced server-side.
- `last_seen_at` and idle expiry update at most once per minute to reduce write volume.
- Expired, revoked, disabled-user, or password-reset sessions are rejected.
- Logout revokes the database session before clearing the cookie.
- Session lookup failure returns `401` without disclosing the failure reason.
- Session identifiers never enter URLs, logs, analytics, or local browser storage.

Cookie attributes in production:

```text
HttpOnly
Secure
SameSite=Lax
Path=/ause-discovery/
host-only cookie
explicit Max-Age no greater than absolute session lifetime
```

Root deployment uses `Path=/`. A `Domain` cookie attribute is prohibited.

### 13.5 CSRF Protection

State-changing browser requests require all of the following:

- valid authenticated session;
- `X-CSRF-Token` containing the raw per-session token;
- constant-time match with stored CSRF-token hash;
- acceptable `Origin` header matching configured public origin;
- Go `http.CrossOriginProtection` checks;
- non-simple expected content type, except controlled multipart upload endpoints.

Safe methods are `GET`, `HEAD`, and `OPTIONS` and MUST NOT mutate state. Login does not require an existing CSRF token but still requires origin protection and rate limiting.

### 13.6 Authorization

The MVP has one effective role, `administrator`, but handlers require explicit permission checks.

| Permission | Protected operations |
| --- | --- |
| `project.read_admin` | Admin Project listing and detail. |
| `project.create` | Create draft Project. |
| `project.edit` | Metadata, participation, and taxonomy updates. |
| `project.publish` | Publication. |
| `project.delete` | Soft deletion and restoration. |
| `person.manage` | Person creation and update. |
| `artifact.upload` | Upload and replacement. |
| `artifact.delete` | Artifact deletion and restoration. |
| `import.preview` | Import upload and preview. |
| `import.execute` | Import selection, resolution, and commit. |
| `search.reindex` | Project reindex. |
| `search.rebuild` | Full index rebuild. |
| `audit.read` | Audit listing. |

Permission checks occur after authentication and before loading sensitive target details where practical.

### 13.7 Login Rate Limiting

Two limits apply:

- normalized username: five failures in 15 minutes;
- source address: 25 failures in 15 minutes.

Blocked attempts return `429` with a generic message and a bounded `Retry-After` header. A successful login clears username-keyed failures but not source-address history. IPv6 source keys use a normalized `/64` network to limit trivial address rotation.

### 13.8 Proxy Trust

- The API listens on the internal Compose network.
- Forwarded client address and scheme headers are trusted only from the configured Nginx service network.
- Direct client-supplied `X-Forwarded-*` headers are removed by Nginx and replaced.
- The API rejects an invalid forwarded host or scheme in production.
- Public URL configuration is authoritative for CSRF origin comparison and external links.

### 13.9 Security Headers

Nginx applies at minimum:

- `Strict-Transport-Security` when TLS is terminated at Nginx;
- `X-Content-Type-Options: nosniff`;
- `Referrer-Policy: strict-origin-when-cross-origin`;
- `Permissions-Policy` disabling unused browser capabilities;
- `Content-Security-Policy` compatible with Vite production assets and without `unsafe-eval`;
- frame restrictions through CSP `frame-ancestors`;
- no server-version disclosure.

The CSP permits same-origin API and artifact access only. Final institutional domains are inserted through deployment configuration.

### 13.10 Input and Output Security

- All SQL is parameterized through sqlc or pgx parameters.
- Dynamic sorting maps validated tokens to static SQL fragments.
- HTML display uses React escaping.
- Search highlights are parsed from fixed markers without raw HTML.
- External redirect parameters are not accepted.
- User filenames appear only in content-disposition encoding and escaped UI text.
- JSON parsing rejects unknown mutation fields.
- Request decompression is disabled unless explicitly required.
- Secrets and credentials are redacted by structured logging middleware.

### 13.11 Audit Integrity

- Application database role may insert and select audit events but may not update or delete them.
- Operator migration role may manage the table.
- Audit list endpoints are administrator-only.
- Metadata keys use a fixed event-specific allowlist.
- Failed login events store normalized username hash rather than raw password or request body.
- Audit retention is not automated in MVP.

## 14. Frontend Implementation Contract

### 14.1 Route Tree

React Router uses the configured basename.

Public routes:

```text
/
/search
/projects/:projectId
/people/:personId
/about
/privacy
/terms
/accessibility
/contact
/*
```

Administrator routes:

```text
/admin/login
/admin
/admin/projects
/admin/projects/new
/admin/projects/:projectId
/admin/people
/admin/people/new
/admin/people/:personId
/admin/imports
/admin/imports/:batchId
/admin/search
/admin/audit
```

The authenticated administrator layout protects every `/admin` route except login. An unauthenticated access redirects to the base-path-safe login route with an internal return path. Return paths are accepted only when relative and inside the configured base path.

### 14.2 Application Providers

Provider order:

1. error boundary;
2. i18n provider;
3. QueryClient provider;
4. session provider;
5. router provider.

The QueryClient default behavior:

- public GET retry: two attempts for network and 5xx failures;
- administrator GET retry: one attempt;
- mutations: no automatic retry;
- `401` clears session state and redirects to login;
- `409 stale_revision` remains on the form and prompts explicit refresh;
- `503 search_unavailable` displays search-specific recovery text;
- query cache never stores raw CSRF or password values.

### 14.3 Generated API Client

- Base URL is constructed from browser origin plus normalized base path plus `api/v1`.
- Credentials mode is `same-origin`.
- CSRF header is injected only for state-changing authenticated calls.
- Request ID from error responses is displayed in technical error details.
- Generated files are never edited manually.
- Feature adapters may transform generated transport types into view models but may not redefine wire contracts.

### 14.4 Search Page State

Search state is URL-owned.

URL parameters include query, page, page size, sort, and every active facet. Behavior:

- form submission updates the URL;
- filter changes reset page to 1;
- sort changes reset page to 1;
- back and forward navigation restore state;
- shareable URLs reproduce the same search;
- invalid URL filter values are removed after a controlled validation message;
- text input may debounce suggestions, but the MVP executes search on submit and explicit filter changes;
- no query history is stored by the application.

Search result states:

- initial browse;
- loading with stable layout;
- results with total and facet counts;
- zero results with filter-reset options;
- search unavailable while navigation and PostgreSQL-backed routes remain usable;
- validation error for malformed URL state.

### 14.5 Project Detail Behavior

- Title is the page heading.
- Reference code is labeled explicitly and never presented as Project ID.
- Academic metadata appears as structured facts.
- Participants are grouped by role and link to Person pages.
- Student ID is visible for student participants.
- Taxonomy values are grouped by dimension.
- Artifact actions are type-aware.
- PDF View opens a new browser tab with `noopener` behavior.
- Download uses a normal link to preserve streaming and browser download behavior.
- Missing Artifact content produces a localized error without hiding Project metadata.

### 14.6 Person Detail Behavior

- Person display name is the page heading.
- Student ID and staff ID appear when present.
- Associated published Projects are grouped by participation role.
- Each association includes Project title, academic year, semester, and reference code.
- No account or authentication language appears on Person pages.

### 14.7 Administrator Forms

Form rules:

- React Hook Form owns field state.
- Zod provides immediate structural validation.
- Server field errors map by stable field path.
- Unsaved-change navigation requires confirmation.
- Submission buttons disable during active mutation.
- Double submission is prevented.
- Successful mutation replaces cached revision with server revision.
- Stale revision preserves unsaved values and offers reload or cancel; no automatic merge occurs.
- Collection editors preserve deterministic order.

Project editor sections:

1. identity and title aliases;
2. academic metadata;
3. abstract;
4. students;
5. advisors, co-advisors, and committee;
6. category, platform, domain, topic, and technology;
7. artifacts;
8. lifecycle and search synchronization status.

### 14.8 Delete and Restore Interaction

Project deletion modal displays:

- Project title;
- reference code when present;
- public-visibility impact;
- search-removal behavior;
- restoration availability;
- required confirmation value input.

The destructive button remains disabled until the exact confirmation value matches. Password re-entry is not required.

Artifact deletion requires a confirmation dialog but not typed text. Artifact restoration displays quota errors when restoration would exceed active Project quota.

### 14.9 Import Interface

Import screens support:

- template downloads;
- drag-and-drop and file picker upload;
- parsing progress polling;
- summary counts;
- row filtering by valid, warning, invalid, selected, or unresolved state;
- field-level issues with CSV row or XLSX sheet/cell references;
- normalized-value comparison;
- candidate Person and Project resolution;
- selection controls that cannot select invalid rows;
- explicit warning acknowledgement;
- atomic commit confirmation;
- stable result summary with created, updated, and skipped rows.

Commit confirmation states that any commit failure rolls back every selected row.

### 14.10 Localization

- English is the only launch locale.
- Every interface string uses a localization key.
- Validation messages map stable server codes to localized text, with server detail as fallback.
- Catalog and taxonomy labels use English database labels in MVP.
- Dates, numbers, and file sizes use locale-aware formatters.
- Project-provided content is never passed through interface translation.
- Translation resources are grouped by common, public, search, project, person, admin, import, auth, and errors namespaces.

### 14.11 Design Tokens

Required semantic tokens include:

```text
color.background
color.surface
color.surface_subtle
color.text
color.text_muted
color.border
color.primary
color.primary_contrast
color.secondary
color.accent
color.focus
color.danger
color.success
color.warning
space.1 through space.12
font.body
font.heading
font.mono
radius.small
radius.medium
shadow.subtle
layout.content_max
layout.reading_max
```

Initial visual direction uses light surfaces, restrained red and purple institutional accents, limited yellow accent, thin rules, clear typography, and minimal shadow. Exact institutional color values remain configuration and design prerequisites.

### 14.12 Accessibility

Required behavior:

- semantic landmarks and heading order;
- labeled inputs and described errors;
- keyboard-operable menus, dialogs, tables, filters, and collection editors;
- visible focus states;
- focus placement after route and dialog transitions;
- focus return after dialog close;
- no color-only status communication;
- screen-reader announcements for search result count, import progress, and mutation outcome;
- minimum WCAG AA contrast target;
- skip link;
- reduced-motion support;
- accessible table fallback or card layout at narrow widths.

### 14.13 Responsive Layout

Reference breakpoints:

- compact: below 640 px;
- medium: 640 px through 1023 px;
- wide: 1024 px and above.

Search filters use an accessible drawer on compact screens and a persistent sidebar on wide screens. Administrator data tables become horizontally scrollable with preserved headers or switch to labeled record cards when tabular comparison is not required.

### 14.14 Legal and Institutional Content

About, Privacy, Terms, Accessibility, Contact, logo, faculty identity, developer credit, and final colors use placeholder development content until approved source material exists.

Production-release acceptance blocks on approved:

- product and faculty naming;
- logo assets and usage permission;
- color values;
- privacy text covering public student IDs;
- terms or acceptable-use text;
- accessibility statement;
- contact details;
- developer credit and GitHub URL.

## 15. Deployment and Operations

### 15.1 Compose Services

`compose.yaml` defines:

| Service | Image or build | Public exposure | Persistent state |
| --- | --- | --- | --- |
| `web` | AUSE Discovery Nginx image | HTTPS/HTTP host ports | None |
| `api` | AUSE Discovery Go image | Internal only | Artifact volume |
| `postgres` | Pinned PostgreSQL image | Internal only | PostgreSQL volume |
| `meilisearch` | Pinned Meilisearch image | Internal only | Meilisearch volume |

`compose.dev.yaml` may expose database and search ports on loopback only and may run source-oriented development commands. Production behavior is validated with the base Compose file.

### 15.2 Volumes

Named volumes:

- `postgres_data`;
- `meilisearch_data`;
- `artifact_data`.

Import temporary data uses a bounded subdirectory of the artifact volume or a dedicated temporary volume with restart-safe preview processing. Source imports are deleted after parsing.

Volume deletion is never part of deployment or normal shutdown instructions.

### 15.3 Health Checks

| Service | Check | Readiness meaning |
| --- | --- | --- |
| PostgreSQL | `pg_isready` plus API connection test | Accepting authenticated database connections. |
| Meilisearch | `/health` | Search service process is healthy. |
| API liveness | `/internal/health/live` | Process event loop is responsive. |
| API readiness | `/internal/health/ready` | Configuration valid, migrations current, PostgreSQL reachable; Meilisearch status reported but not readiness-blocking. |
| Web | internal static health file | Nginx serves built assets. |

Internal health endpoints expose no secrets, DSNs, stack traces, or filesystem paths.

### 15.4 Nginx Routing

For `/ause-discovery/` deployment:

- `/ause-discovery/api/` proxies to the API without accidental path duplication;
- `/ause-discovery/assets/` serves fingerprinted static files with long immutable caching;
- `/ause-discovery/` HTML uses no-cache or short revalidation;
- unknown application routes fall back to `/ause-discovery/index.html`;
- paths outside the configured base path do not fall back to the SPA;
- upload routes use the configured 25 MiB import limit or 250 MiB artifact limit as appropriate;
- response buffering and timeouts support streamed artifact transfer;
- forwarded headers are replaced with trusted values.

Nested-route refresh is a release acceptance test.

### 15.5 Container Security

- API and web containers run as non-root users.
- Root filesystems are read-only where practical.
- API receives a writable artifact mount only at the configured artifact root.
- Linux capabilities are dropped unless explicitly required.
- No Docker socket mount exists.
- PostgreSQL and Meilisearch credentials are not included in image layers.
- Meilisearch master key is available only to the API and Meilisearch services.
- Image builds use multi-stage builds and copy only runtime artifacts.

### 15.6 Database Migrations at Deployment

Deployment sequence:

1. pull pinned images;
2. validate environment configuration;
3. create a PostgreSQL backup according to available operator procedure when schema changes exist;
4. run `ausectl migrations status`;
5. run `ausectl migrations up` as a one-shot operator command;
6. start or replace application services;
7. wait for readiness;
8. run smoke tests;
9. inspect search synchronization status.

The API does not silently apply production migrations at startup.

### 15.7 Logging

- API logs use structured JSON in production and readable text in development.
- Required fields include timestamp, level, service, event, request ID, route template, method, status, duration, and safe actor or target IDs.
- Query strings containing search text are not logged in full by default.
- Passwords, cookies, authorization headers, CSRF tokens, DSNs, API keys, file contents, and import drafts are always redacted.
- Compose log drivers apply size and file-count limits.
- Nginx access logs rotate or use bounded container logging.

### 15.8 Operational Commands

Documented operator procedures include:

- service start, stop, and status;
- migration status and application;
- administrator creation, reset, and disablement;
- catalog validation and synchronization;
- Project reindex and full search rebuild;
- inspection of failed search synchronization;
- artifact-volume capacity inspection;
- PostgreSQL connection and volume checks;
- Meilisearch health and version inspection;
- safe image cleanup without volume deletion;
- log inspection using request IDs.

### 15.9 Deployment Rollback

- Application-only rollback uses the prior immutable image digests.
- Database rollback uses a forward corrective migration or verified restore procedure, not automatic down migration.
- Meilisearch rollback uses rebuild against a compatible image and canonical PostgreSQL data.
- Artifact storage layout changes require backward-readable code until a dedicated migration completes.
- A failed search rebuild leaves the prior index serving.

### 15.10 Backup Boundary

Automated backup implementation is outside MVP scope. Operational documentation identifies critical state:

- PostgreSQL;
- artifact volume;
- production environment configuration stored through approved secret management.

Meilisearch data is excluded from critical backup requirements because full rebuild is mandatory.

## 16. CI, Release, and Supply Chain

### 16.1 Pull Request Workflow

GitHub Actions stages:

1. repository policy and documentation checks;
2. exact dependency and lockfile validation;
3. OpenAPI validation;
4. generated-code drift check;
5. Go formatting, vetting, linting, and unit tests;
6. frontend formatting, linting, type checking, and unit tests;
7. PostgreSQL and Meilisearch integration tests;
8. production image builds without publication;
9. Playwright E2E tests against production-like Compose;
10. vulnerability and dependency audit reporting.

### 16.2 Release Workflow

- Release trigger is a signed or protected Git tag matching `vMAJOR.MINOR.PATCH`.
- Application version is injected at build time and exposed through safe runtime metadata.
- API and web images receive the version tag and immutable Git commit tag.
- Images publish to GHCR.
- Build provenance and software-bill-of-materials attestations are generated where supported.
- Production Compose references release image digests, not mutable application tags.
- Release notes include migrations, configuration changes, search schema changes, and operator actions.

### 16.3 GitHub Action Pinning

Each external action reference uses a full commit SHA. A comment records the human-readable release. Automated dependency updates may propose new SHAs but cannot merge without CI and review.

### 16.4 Secret Handling

- CI uses GitHub environments and least-privilege tokens.
- Pull-request jobs from untrusted forks receive no publication or deployment credentials.
- GHCR publication token receives package-write permission only in release jobs.
- VM SSH deployment is not automated in MVP.
- No production secret is committed, printed, cached in build artifacts, or placed in frontend variables.

## 17. Testing Specification

### 17.1 Test Layers

| Layer | Purpose | External services |
| --- | --- | --- |
| Go unit | Domain validation, normalization, state transitions, projection logic | None |
| Go database integration | SQL, constraints, transactions, migrations, locking | PostgreSQL container |
| Go search integration | Settings, indexing, filtering, retry, rebuild | PostgreSQL and Meilisearch containers |
| Go artifact integration | Filesystem containment, streaming, ranges, cleanup | Temporary filesystem |
| Frontend unit/component | Forms, route state, accessibility, error mapping | MSW |
| API contract | OpenAPI generation and response conformance | API test server |
| Browser E2E | Public and admin flows | Full Compose stack |
| Deployment smoke | Base path, health, persistence, restart | Production-like Compose |

### 17.2 Backend Unit Tests

Required cases:

- seven-digit Student ID accepts leading zeroes and rejects other shapes;
- reference code remains optional and non-unique;
- Project lifecycle allows only documented transitions;
- Restore returns to pre-delete status;
- publication validation reports all missing requirements;
- Person matching never merges on name alone;
- taxonomy keys remain dimension-specific;
- academic version compatibility;
- optimistic-concurrency mismatch;
- Project revision increments once for child collection mutations;
- exact deletion confirmation behavior;
- Artifact allowlist and quota calculations;
- search document excludes internal fields;
- seven-digit query strict path;
- import normalization and stable issue codes;
- Argon2id hash parse, verify, and rehash detection;
- session idle and absolute expiration;
- CSRF token comparison;
- permission mapping.

### 17.3 Database Integration Tests

Required cases:

- migrations apply from empty database;
- migration status rejects a newer unknown schema;
- partial uniqueness for Student ID and staff ID;
- reference-code duplicates are permitted;
- invalid Project status combinations fail constraints;
- overlapping academic versions fail;
- invalid participation roles and duplicates fail;
- concurrent Project edits produce one success and one stale revision;
- concurrent artifact uploads cannot exceed quota;
- import commit rolls back every row after one forced failure;
- repeated successful import commit returns the original result;
- search synchronization pending state commits atomically with Project changes;
- audit events persist with the business mutation;
- disabled user sessions become invalid.

### 17.4 Search Integration Tests

Required cases:

- title, alias, Person, taxonomy, academic, and abstract search;
- exact Student ID lookup with leading zero;
- no near-match Student ID results;
- OR within a facet and AND across facets;
- artifact-presence facets change after upload, delete, and restore;
- Project edit updates search revision;
- deleted Project is removed;
- restored published Project returns;
- restored draft Project remains absent;
- failed Meilisearch update stays pending and retries;
- out-of-order attempt cannot mark stale revision synced;
- full rebuild retains serving index until swap;
- changes during rebuild appear after catch-up;
- index loss recovers entirely from PostgreSQL;
- search outage does not break Project detail.

### 17.5 Artifact Tests

Required cases:

- extension and detected MIME agreement;
- forged browser MIME rejected;
- zero-byte file rejected;
- file-size boundary at exactly 250 MiB and one byte over, using sparse or generated streams;
- Project quota boundary;
- restoration quota conflict;
- path traversal and symbolic-link attempts;
- temporary file cleanup after every failure stage;
- database failure after final rename cleans new bytes;
- range request behavior;
- PDF inline disposition;
- download attachment disposition and safe Unicode filename;
- deleted Artifact or Project returns public `404`;
- missing physical file returns controlled error;
- replacement preserves prior bytes and creates new identity.

### 17.6 Import Tests

Required cases:

- UTF-8 CSV with and without BOM;
- quoted comma, quote, and newline handling;
- unknown, duplicate, missing, and reordered headers;
- invalid JSON arrays;
- leading-zero Student IDs;
- XLSX formulas, hidden sheets, macros, external links, and excess sheets rejected;
- row, cell, expanded-size, and parse-time limits;
- unknown and retired seed keys;
- same-name Person ambiguity;
- duplicate Project candidates;
- invalid rows cannot be selected;
- warning acknowledgement requirement;
- batch revision conflict;
- batch expiry;
- selected valid rows commit atomically;
- selected update requires current Project revision;
- source bytes removed after parse;
- committed result remains idempotent.

### 17.7 Frontend Component Tests

Required cases:

- generated client uses configured base path;
- search state round-trips through URL;
- filters reset pagination;
- zero-results and unavailable states;
- Project and Person links preserve basename;
- admin route guard and return path validation;
- session expiry redirect;
- server field-error mapping;
- stale revision preserves form values;
- typed delete confirmation;
- artifact action visibility by type;
- import row selection restrictions;
- accessible names, focus management, and live announcements;
- localization key coverage.

### 17.8 Browser E2E Tests

Critical E2E flows:

1. browse published Projects without authentication;
2. search by topic and combine facets;
3. search exact public Student ID;
4. open Project, Person, PDF View, and download;
5. login and logout;
6. create draft, assign people and taxonomy, publish, and observe search result;
7. edit published metadata and observe synchronized search;
8. delete published Project, verify public removal, restore, and verify return;
9. upload, download, delete, restore, and replace Artifact;
10. upload CSV and XLSX, resolve warnings, commit, and inspect results;
11. force Meilisearch outage, verify Project detail, restore search, and observe retry;
12. run full rebuild and observe progress;
13. refresh every nested route beneath `/ause-discovery/`;
14. repeat routing smoke tests at `/`.

### 17.9 Accessibility Tests

- Automated Axe checks run on every main public and admin screen.
- Keyboard-only E2E covers search filters, dialogs, forms, collection editors, import table, and artifact actions.
- Automated checks do not replace manual screen-reader and zoom review before release.
- Manual release review covers 200 percent zoom, reduced motion, high contrast where supported, and mobile viewport behavior.

### 17.10 Performance and Resource Checks

MVP release checks use representative data:

- 10000 Projects;
- 30000 People;
- 100000 participation and taxonomy relationships;
- 5000-row import;
- artifacts streamed at the configured file-size boundary.

Checks verify bounded memory, no whole-file buffering, acceptable search response, indexed database queries, bounded log growth, and restart recovery. Hard latency service-level objectives are not introduced without measured institutional requirements.

## 18. Implementation Work Packages

Each work package is independently assignable only after prerequisites pass. An implementation agent receives exclusive ownership of the listed area during active edits. Shared contract changes require root coordination.

### WP-00 Repository Bootstrap

**Prerequisites:** Confirmed GitHub owner and repository URL.

**Ownership:** Root files, existing frontend scaffold manifests, CI skeleton, project manifests.

**Deliverables:** Existing Git and Vite scaffold normalization, npm-to-pnpm migration, removal of npm lock and untracked dependency directories, ignore rules, pnpm workspace, Makefile, pinned tool metadata, baseline README, environment example.

**Verification:** `make doctor` and repository-policy checks.

**Prohibited:** Application feature code.

### WP-01 API and Generation Foundation

**Prerequisites:** WP-00.

**Ownership:** OpenAPI contract, Go and TypeScript generator configuration, generated transport folders.

**Deliverables:** Complete component schemas, shared Problem Details, generation commands, drift check.

**Verification:** OpenAPI validation, deterministic generation, Go and TypeScript compilation.

**Prohibited:** Business behavior in generated handlers.

### WP-02 Database Foundation

**Prerequisites:** WP-00 and domain schema in this document.

**Ownership:** Migrations, sqlc configuration, SQL queries, database test harness.

**Deliverables:** All canonical tables, constraints, indexes, transaction helpers, migration CLI integration.

**Verification:** Empty-database migration, constraint tests, sqlc generation, migration status.

**Prohibited:** Meilisearch calls from migrations.

### WP-03 Catalog Seeds

**Prerequisites:** WP-02 and approved initial vocabulary.

**Ownership:** YAML seed schema, seed files, validation, catalog sync service and CLI.

**Deliverables:** Program, Major, Course, and taxonomy validation and synchronization.

**Verification:** deterministic sync, retirement behavior, historical-version tests.

**Prohibited:** Administrator catalog CRUD UI.

### WP-04 Backend Platform

**Prerequisites:** WP-00 and WP-02.

**Ownership:** configuration, logging, HTTP server, middleware, health, request IDs, shutdown.

**Deliverables:** production-safe startup validation and service skeleton.

**Verification:** configuration table tests, health behavior, trusted-proxy tests, graceful shutdown.

**Prohibited:** Feature-specific SQL.

### WP-05 Authentication and Audit

**Prerequisites:** WP-01, WP-02, WP-04.

**Ownership:** auth and audit modules, admin CLI auth commands, auth endpoints.

**Deliverables:** Argon2id credentials, sessions, CSRF, rate limiting, authorization, audit append service.

**Verification:** unit, database, and HTTP security tests.

**Prohibited:** Browser account-management screens.

### WP-06 Projects, People, and Classifications

**Prerequisites:** WP-01 through WP-05.

**Ownership:** projects, people, and classification application services and endpoints.

**Deliverables:** drafts, validation, publication, delete/restore, participation replacement, taxonomy replacement, public details.

**Verification:** lifecycle, concurrency, authorization, and public-visibility tests.

**Prohibited:** Search engine logic and artifact bytes.

### WP-07 Artifact Storage

**Prerequisites:** WP-05 and WP-06.

**Ownership:** artifact module, filesystem adapter, upload and serving endpoints.

**Deliverables:** streaming upload, type validation, quota, view, download, delete, restore, replacement.

**Verification:** artifact test matrix including path safety and range requests.

**Prohibited:** External-link Artifacts and permanent cleanup.

### WP-08 Search

**Prerequisites:** WP-03, WP-06, WP-07.

**Ownership:** search module, Meilisearch adapter, synchronizer, rebuild operations, public search endpoint.

**Deliverables:** index configuration, projection, filters, ranking, retry, reindex, atomic rebuild.

**Verification:** complete search integration and failure-recovery matrix.

**Prohibited:** Semantic search, report-body indexing, analytics.

### WP-09 Imports

**Prerequisites:** WP-03, WP-05, WP-06.

**Ownership:** import module, templates, parsers, preview persistence, resolution, commit.

**Deliverables:** exact CSV/XLSX workflows and cleanup.

**Verification:** parser limits, validation, ambiguity resolution, atomicity, idempotency.

**Prohibited:** Artifact import and AI extraction.

### WP-10 Frontend Foundation

**Prerequisites:** WP-01 and stable auth/runtime APIs.

**Ownership:** frontend app shell, generated client integration, routing, i18n, tokens, shared components, test harness.

**Deliverables:** base-path-safe application foundation and accessibility primitives.

**Verification:** type checking, component tests, basename tests, Axe baseline.

**Prohibited:** General-purpose component framework.

### WP-11 Public Frontend

**Prerequisites:** WP-06, WP-08, WP-10.

**Ownership:** Home, Search, Project, Person, legal and institutional public routes.

**Deliverables:** complete public discovery and artifact-access experience.

**Verification:** public component and E2E flows, responsive and accessibility review.

**Prohibited:** Recommendations, trending, analytics.

### WP-12 Administrator Frontend

**Prerequisites:** WP-05 through WP-10.

**Ownership:** Login, dashboard, Project, Person, Artifact, Import, Search, and Audit screens.

**Deliverables:** all administrator workflows with concurrency and error handling.

**Verification:** administrator component and E2E flows.

**Prohibited:** Account CRUD and taxonomy CMS.

### WP-13 Containers and Nginx

**Prerequisites:** WP-04, WP-10, stable build outputs.

**Ownership:** Dockerfiles, Compose, Nginx, health wiring, volumes, runtime users.

**Deliverables:** local and production-like Compose stack.

**Verification:** image build, health, persistence, streaming, root and subpath routing.

**Prohibited:** Public PostgreSQL or Meilisearch exposure.

### WP-14 CI and Release

**Prerequisites:** WP-00 and available project checks; completed incrementally.

**Ownership:** GitHub Actions, GHCR publication, provenance, release documentation.

**Deliverables:** pull-request validation and version-tag image release.

**Verification:** protected dry-run release and immutable image references.

**Prohibited:** Automated VM deployment.

### WP-15 Acceptance and Operations

**Prerequisites:** WP-00 through WP-14.

**Ownership:** cross-system verification, operator runbooks, acceptance traceability.

**Deliverables:** complete acceptance evidence and production prerequisite list.

**Verification:** `make test-all`, manual accessibility review, deployment smoke tests, recovery exercises.

**Prohibited:** Waiving failed mandatory criteria without specification change.

## 19. Requirement Traceability

| Product capability | Primary work packages | Mandatory verification |
| --- | --- | --- |
| Public search and filters | WP-08, WP-11 | Search integration and E2E search flows. |
| Project detail | WP-06, WP-11 | Public visibility and detail E2E. |
| Person detail and public Student ID | WP-06, WP-11 | Student-ID unit, API, and E2E tests. |
| PDF View and downloads | WP-07, WP-11 | Range, disposition, and browser tests. |
| Local administrator authentication | WP-05, WP-12 | Auth security and login E2E. |
| Manual Project entry | WP-06, WP-12 | Draft-to-publish E2E. |
| CSV and XLSX import | WP-09, WP-12 | Parser, preview, atomic commit, and E2E tests. |
| Reversible deletion | WP-06, WP-07, WP-12 | Delete/restore lifecycle tests. |
| Project reindex and full rebuild | WP-08, WP-12 | Retry, loss, and atomic rebuild tests. |
| Docker Compose deployment | WP-13 | Persistence, health, restart, and isolation tests. |
| Non-root base path | WP-10, WP-13 | `/ause-discovery/` nested-route and asset tests. |
| Localization readiness | WP-10 through WP-12 | Localization-key coverage. |
| Dark-mode readiness | WP-10 | Semantic-token review; no dark theme required. |
| Audit logging | WP-05 and every mutation package | Database and endpoint audit assertions. |
| Search failure isolation | WP-08 | Detail availability during search outage. |
| Security baseline | WP-05, WP-07, WP-13 | Security test matrix and configuration review. |

## 20. MVP Completion Gates

### 20.1 Contract Gate

- OpenAPI validates and generated outputs are current.
- Database migrations apply from empty state.
- Seed catalogs validate.
- Exact dependency and image pins are present.
- No unresolved implementation decision remains inside a work package.

### 20.2 Functional Gate

All acceptance scenarios in the product specification and Section 17 pass. Draft, publication, search, artifacts, import, deletion, restoration, and maintenance flows operate through public and administrator interfaces.

### 20.3 Security Gate

- HTTPS production configuration documented and tested at the terminating layer.
- Password, session, CSRF, authorization, upload, path, and secret controls pass.
- No public Meilisearch or PostgreSQL exposure exists.
- No critical or high known vulnerability remains without documented risk acceptance.
- Public Student ID policy appears in approved privacy text.

### 20.4 Operations Gate

- Migrations, administrator management, catalog sync, search rebuild, logs, health, restart, and rollback procedures are documented and exercised.
- Persistent volumes survive service recreation.
- Search index can be deleted and rebuilt from PostgreSQL.
- Missing artifact content fails locally without breaking Project metadata.

### 20.5 Accessibility Gate

- Automated accessibility checks pass.
- Keyboard-only critical flows pass.
- Manual zoom, focus, contrast, and screen-reader review completes.
- No critical accessibility defect remains open.

### 20.6 Production Content Gate

Approved institutional name, logos, colors, legal text, privacy policy, accessibility statement, contact information, developer credit, and public GitHub URL replace placeholders before production launch.

## 21. Explicit Deferred Work

The following remain outside MVP implementation:

- Microsoft Entra ID;
- organization-only public access;
- student and lecturer accounts;
- taxonomy or academic catalog CMS;
- Person merge UI;
- recommendation and similar-Project features;
- semantic or vector search;
- report-body, slide-body, source-code, or archive-content search;
- AI extraction and AI tagging;
- analytics and telemetry;
- external-link Artifacts;
- embedded video player;
- custom PDF or Office viewer;
- artifact antivirus or content-disarm service;
- permanent Project or Artifact destruction;
- automated backup, retention, restore, or disaster recovery;
- Redis, message brokers, distributed workers, and microservices;
- automated VM deployment.

Deferred work must not introduce speculative abstractions into MVP modules.

## 22. External Prerequisites

The following facts require confirmation outside source code before the associated implementation or release gate:

- GitHub owner and repository URL;
- GHCR organization or owner;
- approved initial Program, Major, and Course seed vocabulary plus institutional review of the documented initial taxonomy seed;
- university VM CPU architecture, RAM, storage allocation, operating system, and Docker support;
- DNS name and TLS termination ownership;
- production public URL and final base path confirmation;
- outbound registry access from the VM;
- approved artifact-storage host path or volume policy;
- approved institutional branding and legal content;
- operator ownership for database and artifact backups after MVP;
- final privacy approval for public Student IDs.

Unknown prerequisites remain explicit blockers. Implementation agents must not invent institutional values.

## 23. Definition of Done for Each Implementation Pull Request

Every implementation pull request MUST include:

- linked work package and requirement rows;
- only in-scope changes;
- exact interface or migration updates;
- generated outputs when contracts change;
- tests for success, validation, authorization, concurrency, and failure behavior as applicable;
- passing formatting, lint, generation, unit, and relevant integration checks;
- configuration and documentation updates;
- no secret or runtime-state files;
- no unrelated refactor;
- concise verification evidence;
- migration, rollout, and rollback notes when persisted state or deployment changes.

Completion means the documented behavior is implemented and verified. Partial scaffolding does not satisfy a work package completion gate.

## 24. Normative Technical References

Implementation behavior should follow these primary references where this document does not narrow behavior further:

- [OpenAPI Specification 3.1.0](https://spec.openapis.org/oas/v3.1.0)
- [RFC 9457, Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457)
- [RFC 4180, Common Format and MIME Type for CSV Files](https://www.rfc-editor.org/rfc/rfc4180)
- [RFC 6266, Content-Disposition in HTTP](https://www.rfc-editor.org/rfc/rfc6266)
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [OWASP Cross-Site Request Forgery Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)
- [OWASP File Upload Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/File_Upload_Cheat_Sheet.html)
- [Meilisearch swap indexes API](https://www.meilisearch.com/docs/reference/api/indexes/swap-indexes)
- [Go CrossOriginProtection documentation](https://pkg.go.dev/net/http#CrossOriginProtection)

When a reference permits multiple approaches, the narrower requirement in this document controls AUSE Discovery.
