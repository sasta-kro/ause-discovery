# Architectural decision log

## AD-001 Canonical and derived data

**Status:** Accepted

**Decision:** PostgreSQL is canonical. Meilisearch stores rebuildable search projections only.

**Reason:** Canonical relational integrity and independent search evolution are required.

**Consequence:** Every searchable mutation records pending synchronization in the PostgreSQL transaction. Full index rebuild from PostgreSQL is mandatory.

## AD-002 Modular monolith

**Status:** Accepted

**Decision:** The Go backend is one modular monolith.

**Reason:** Expected scale and single-maintainer constraints do not justify distributed services.

**Consequence:** No broker, Redis, or microservice boundary enters the MVP. In-process background reconciliation is permitted.

## AD-003 Contract-controlled API

**Status:** Accepted

**Decision:** OpenAPI 3.1 is the HTTP source of truth, with generated Go transport types and TypeScript client/types.

**Reason:** Shared generated contracts reduce interface drift and implementation-agent invention.

**Consequence:** Generated outputs are tracked and verified for drift in CI. Business handlers remain handwritten.

## AD-004 Explicit SQL data access

**Status:** Accepted

**Decision:** PostgreSQL access uses pgx, sqlc, and goose.

**Reason:** Explicit SQL and generated query types provide control without a generic ORM abstraction.

**Consequence:** Schema changes use versioned migrations. Generic repository layers are prohibited without a real boundary.

## AD-005 Reversible deletion

**Status:** Accepted

**Decision:** Product wording uses Delete and Restore while database records and Artifact bytes remain preserved.

**Reason:** Archival records require deliberate removal without immediate destruction.

**Consequence:** Project status is draft, published, or deleted. Restore returns to the pre-delete status. Permanent cleanup is deferred.

## AD-006 Versioned developer-controlled catalogs

**Status:** Accepted

**Decision:** Taxonomy and academic catalogs originate from version-controlled YAML and synchronize to PostgreSQL.

**Reason:** Reproducibility and historical accuracy outweigh MVP catalog-CMS flexibility.

**Consequence:** Administrators assign existing values only. Historical academic labels use version records.

## AD-007 Persistent metadata import

**Status:** Accepted

**Decision:** CSV and XLSX imports create persistent metadata previews. Selected valid or acknowledged-warning rows commit atomically.

**Reason:** Stable review, explicit duplicate resolution, and tamper-resistant commit behavior are required.

**Consequence:** Artifact import is excluded. Invalid rows cannot commit. Batch state and result are stored in PostgreSQL.

## AD-008 Local controlled Artifact storage

**Status:** Accepted

**Decision:** MVP Artifacts are stored files on persistent local storage behind an application storage boundary.

**Reason:** The university VM target does not require object storage for MVP.

**Consequence:** Opaque storage keys, streamed access, path containment, strict allowlists, 250 MiB file limit, and 2 GiB active Project quota are mandatory. External links are deferred.

## AD-009 Base-path deployment

**Status:** Accepted

**Decision:** Default production base path is `/ause-discovery/`; root deployment remains supported.

**Reason:** University hosting may place the application beneath a shared domain path.

**Consequence:** Vite assets, routing, API URLs, cookies, Nginx fallback, redirects, and Artifact links share one normalized base-path contract.

## AD-010 Exact version pinning

**Status:** Accepted

**Decision:** Direct dependencies, toolchains, service versions, actions, and production images use exact pins.

**Reason:** Reproducible agent implementation and production operation require known versions.

**Consequence:** Lockfiles and generated outputs are tracked. Container images use exact tags and manifest digests. Upgrades require explicit review.
