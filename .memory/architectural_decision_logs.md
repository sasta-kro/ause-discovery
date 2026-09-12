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

**Reason:** Shared generated contracts reduce interface drift and unsupported implementation invention.

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

**Status:** Superseded 2026-09-10 by AD-011

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

**Reason:** Reproducible implementation and production operation require known versions.

**Consequence:** Lockfiles and generated outputs are tracked. Container images use exact tags and manifest digests. Upgrades require explicit review.

## AD-011 Pluggable Artifact storage

**Status:** Accepted 2026-09-10 (supersedes AD-008). Provider: Backblaze B2.

**Decision:** Artifact bytes move behind a storage interface with a per-Artifact backend marker, so implementations are exchanged by configuration. The external backend is Backblaze B2 through its S3-compatible API. Public serving always proxies through the API and never exposes storage URLs to visitors.

**Reason:** The institution's other administrator rejected VM-local storage as the primary backend and S3 as a service. Google Drive was evaluated and rejected: consumer-account lock exposure for a public archive, OAuth refresh-token fragility, per-user API rate limits, and no institutional second copy. Requirements are zero running cost and no storage outage that permanently breaks downloads. B2 over Cloudflare R2: no payment method is required, the first 10 GB are always free, and the free monthly egress allowance of 3x average stored bytes is sufficient at current scale and grows with the corpus. R2's distinguishing advantage, unmetered egress, is unusable at current scale and comes with a payment method on file. If traffic grows beyond the free allowances, the provider can change later without a rewrite.

**Consequence:** The filesystem implementation remains for development and tests. The B2 backend must support streamed put and range-capable open, a one-time migration of existing bytes, an institutional second copy, upstream-unavailability mapping to controlled 503 responses, and backend selection without rewrite. Operators monitor monthly egress against the 3x allowance and stored bytes against 10 GB; crossing either is a trigger to re-evaluate the provider, not an outage.

## AD-012 Project Repository Links in the unified Project Content manifest

**Status:** Accepted 2026-09-12

**Decision:** Verified Project Repository Links are ordered Project metadata, persisted separately from Artifacts and imported through the existing versioned Project Content manifest alongside optional Project Logos and Project Files. Only extractor links classified as `project_repo` are eligible. Third-party references remain extractor provenance and do not enter application data.

**Reason:** One Project can have multiple repositories, each with last-checked availability. A separate CSV or CLI would duplicate Project mapping, while treating URLs as Artifacts would introduce fake storage, MIME, byte-count, quota, migration, and download semantics for content that has no stored bytes. A dedicated Project metadata relation preserves the Artifact boundary and permits public presentation without leaking extractor internals.

**Consequence:** The Project Content manifest gains an optional links collection. Public Project details may display imported repositories and their last-checked availability, but search documents, Artifact counts and facets, Artifact storage, B2 migration, direct-download behavior, and third-party-reference URLs remain unaffected. Links open externally with normal external-link safeguards. Liveness is evidence at its recorded check time, not a runtime availability guarantee.
