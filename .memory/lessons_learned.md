# Lessons learned

## Goose procedural statements

Goose splits SQL migrations at semicolons unless procedural function bodies are enclosed by `StatementBegin` and `StatementEnd`. Fresh-database migration tests must cover every migration containing PL/pgSQL.

## Open-ended PostgreSQL ranges

An inclusive `int4range` upper-bound sentinel at the maximum integer overflows during canonicalization. A `NULL` upper bound represents an unbounded historical validity range safely and preserves overlap exclusion behavior.

## Optional audit values

Optional audit UUIDs require nullable PostgreSQL conversion. A zero UUID is a real value and violates foreign keys when no actor exists. Absent audit metadata must serialize as an empty JSON object rather than JSON `null` when the schema requires object metadata.

## Reverse-proxy trust

The original Host header, including a non-default port, must reach same-origin validation. Forwarded client addresses must be evaluated from the immediate trusted peer toward the client, selecting the first untrusted address. Forwarded protocol is trusted only when the immediate peer is configured as trusted.

## Filesystem and metadata transactions

Filesystem finalization and PostgreSQL metadata cannot share one atomic transaction. Artifact uploads therefore require an opaque new storage identity, atomic same-filesystem rename, serialized metadata and quota checks, and compensating removal only for newly finalized bytes when validation or metadata persistence fails. Reversible business deletion must retain previously committed bytes.

## Meilisearch task and filter contracts

Meilisearch task identifiers are zero-based, so task ID `0` is valid and must not be treated as absent. Exact-match query branches must configure every referenced attribute as filterable before the index is populated or swapped. Live post-swap search is the focused boundary check for both contracts.

## Formatting existing files

Do not run repository-wide formatters during a feature increment when existing files are not already formatter-clean. Format new or intentionally reformatted files only, and inspect the diff afterward. Repository-wide formatting belongs in a dedicated formatting change.

## Import review state

Import row responses must expose persisted warning acknowledgement and duplicate-resolution state. A refreshed review interface otherwise presents stale decisions even when the server has stored the correct commit inputs.

Bundled import examples must use version-controlled catalog keys rather than display labels or invented values. Parsing the distributed CSV and XLSX templates through the production adapters provides a focused guard against examples that cannot produce a valid preview.

## Containerized pnpm generation with host node_modules

Running the pinned Node container against the host-installed workspace lets pnpm 11 attempt to purge the host-platform `node_modules` through its dependency-status check, which aborts without a TTY. Pure-JavaScript generator binaries can be invoked directly through their `.bin` shim inside the container, skipping pnpm entirely, when generation is the only requirement.

## Stale persistent development database

The persistent development Compose PostgreSQL was migrated with an early revision of the single initial migration, and goose cannot re-apply amended statements to a version already recorded. Schema-dependent smoke verification must create and migrate a disposable database inside that instance instead of using the shared development database.
