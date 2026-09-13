# Storage provider readiness increment

## Assignment

Implement and commit readiness checks for the configured Artifact storage
provider.

This is one bounded backend increment. It corrects the current condition where
both readiness endpoints can report ready while the configured Backblaze B2
bucket is unreachable or its credentials are rejected. Liveness must remain
independent of storage availability.

Start from accepted commit `3c487ee`. Preserve unrelated working-tree changes
and all existing behavior unless this brief explicitly changes it.

## Required reading before coding

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`
4. `.memory/architectural_decision_logs.md`, especially AD-011
5. `.memory/implementation-status/completed.md`, especially former items 8, 12, and 13
6. `.memory/docs/increments/10-pluggable-artifact-storage.md`
7. `backend/internal/artifacts/storage.go`
8. `backend/internal/artifacts/storage_b2.go`
9. `backend/internal/artifacts/storage_test.go`
10. `backend/internal/artifacts/storage_b2_test.go`
11. `backend/internal/platform/config/config.go`
12. `backend/internal/platform/httpserver/health.go` and its tests
13. `backend/cmd/api/main.go`
14. `compose.yaml`
15. This brief

## Current defect

`httpserver.CheckReadiness` currently checks the PostgreSQL schema and calls
`Config.CheckStorageDirectories`.

When the configured storage provider is local, that directory check covers the
Artifact root and the metadata-import temporary directory. When B2 is the
configured default, the Artifact-root check is deliberately skipped and only
the local metadata-import temporary directory is probed. The configured B2
endpoint, credentials, and bucket are never contacted.

Consequences:

- `/health/ready` and `/api/v1/health/ready` can return `200` during a B2
  outage or after B2 credentials become invalid;
- the container health check consumes `/health/ready`, so Compose can report a
  healthy API even though Project File and Project Logo reads and writes fail;
- local directory readiness and external storage readiness live in different
  layers instead of following the accepted storage-provider boundary.

## Required outcome

Both readiness endpoints return `200` only when all of these conditions hold:

1. PostgreSQL is reachable and exposes the supported schema version.
2. The metadata-import temporary directory accepts its existing write probe.
3. The configured default storage provider passes its readiness check.

The local provider checks its Artifact root. The B2 provider performs one
bounded, non-persistent bucket-access request. A provider failure produces the
existing generic `503` readiness response without exposing endpoint, bucket,
credential, SDK, or filesystem details.

`/health/live` and `/api/v1/health/live` continue to report process liveness and
must not call PostgreSQL or storage.

## Scope

### Included

- One storage-provider readiness operation on the existing storage interface.
- Local storage readiness through a temporary write probe that is always
  removed.
- B2 readiness through its S3-compatible API without uploading, replacing,
  hiding, or deleting an object.
- Readiness orchestration through the configured `StorageSet` default.
- Shared wiring for the unversioned and versioned readiness endpoints.
- Controlled timeout, concurrent request coalescing, and short result caching
  for the remote B2 probe.
- Focused local-storage, B2-stub, readiness-handler, and API wiring tests.
- One opt-in, non-mutating live B2 check when the existing local credentials
  are available.
- Durable implementation-status and lessons updates when warranted.
- One coherent implementation commit.

### Excluded

- Project Logo, Project Content import, Artifact migration, or frontend work.
- Database migrations or OpenAPI changes.
- Storage-provider renaming or the broader terminology refactor already in the
  backlog.
- Automatic failover, second-copy management, reverse migration, or checking
  every provider registered only for historical reads.
- Writing a permanent health object to B2.
- Upload-and-delete probes against B2.
- Meilisearch readiness. Its existing degraded behavior remains unchanged.
- Deployment-guide consolidation, file moves, archive cleanup, or public
  documentation changes. Those follow after this increment is accepted.
- Stack rebuilds, volume deletion, bulk imports, image publication, or VM
  deployment.

## Storage interface contract

Add a readiness method to the existing storage interface. Use the current
interface and type names from the accepted baseline even though a broader
storage-terminology rename is planned separately.

An acceptable shape is:

```go
CheckReady(ctx context.Context) error
```

Contract:

- success means the provider is sufficiently reachable for the readiness
  boundary;
- cancellation and deadlines must be honored;
- failures must match `ErrStorageUnavailable` while retaining safe internal
  diagnostic context;
- no successful or failed check may leave a new Artifact, Project Logo, hidden
  object version, local probe file, or temporary file behind;
- implementations must be safe for concurrent use;
- tests and adapters implementing the interface must be updated explicitly,
  without weakening the interface through optional type assertions.

Add a `StorageSet` readiness method that checks only `Default()`. A secondary
provider configured for migration or historical reads must not make the whole
application unready when it is not the active write provider.

## Local provider behavior

Local readiness belongs to `LocalStorage`, not to HTTP or configuration code.

The check must:

- reject a canceled context before filesystem work;
- require the configured absolute root to exist as a directory after normal
  application startup;
- retain the existing symlink and containment protections;
- create, close, and remove one bounded temporary probe directly inside the
  Artifact root;
- return an unavailable error if creation, close, or removal fails;
- leave no probe file behind on either success or handled failure where cleanup
  remains possible.

Do not silently recreate a missing runtime mount during readiness. Startup may
continue to create required directories through the existing initialization
path, but a root that disappears after startup must make readiness fail.

The provider's `.tmp` upload directory is created by the existing write path
and may not exist before the first upload. Readiness must not require that
directory or make an empty fresh deployment fail.

The metadata-import temporary directory remains a separate configuration-level
probe because it is not owned by the Artifact storage provider.

## B2 provider behavior

Use one S3 `ListObjectsV2` request against the configured bucket with:

- `MaxKeys=1`;
- prefix `v1/`, matching the application-owned storage-key namespace;
- the readiness request context passed directly to the AWS SDK.

The response contents are irrelevant. An empty bucket is healthy. The request
is read-only and verifies the configured endpoint, request signing,
credentials, bucket access, and `listFiles` capability without requiring the
broader `listAllBucketNames` capability that `HeadBucket` may require for a
bucket-restricted Backblaze application key.

Backblaze documents `ListObjectsV2` as supported by the S3-compatible API and
maps it to the `listFiles` application-key capability. Record that capability
in the later deployment documentation, not in this increment.

Any SDK, HTTP, authentication, authorization, missing-bucket, malformed
response, cancellation, or timeout error makes the provider unavailable.
Never treat an HTTP response as healthy merely because it resembles a missing
object response.

This non-persistent probe cannot prove `writeFiles` or `deleteFiles`
capabilities. Those remain required for the application operations that use
them and continue to fail safely through the existing storage-unavailable
surface when misconfigured. Do not add a mutating readiness request solely to
prove those capabilities.

Do not use `Exists` for readiness. Its boolean contract cannot distinguish a
missing object from an unavailable provider.

## Remote-probe request control

The readiness endpoints are unauthenticated and the Compose container health
check calls them every ten seconds. Arbitrary HTTP traffic must not create one
B2 request per incoming readiness request.

Implement a small concurrency-safe probe controller for B2 with these
properties:

- at most one B2 readiness request is in flight for the shared provider
  instance;
- concurrent callers reuse the in-flight result rather than starting parallel
  bucket requests;
- successful results are cached for a short, explicit duration no longer than
  30 seconds;
- failed results are cached for no longer than 5 seconds so recovery is
  observed promptly;
- waiting callers still honor their own context cancellation;
- tests advance an injected clock or use another deterministic seam rather
  than sleeping;
- cache state contains no credentials, object names, or response bodies.

Keep this helper narrow. Do not introduce a generic job framework, background
poller, new dependency, or global mutable singleton.

## Readiness composition and wiring

Refactor readiness so storage is passed explicitly:

```text
readiness request
  -> supported PostgreSQL schema
  -> metadata-import temporary directory
  -> configured default storage provider
```

`backend/cmd/api/main.go` constructs one `StorageSet`. The same set or one
shared readiness closure must serve:

- the unversioned health handler registered at `<base-path>/health/ready`;
- the versioned controller endpoint at
  `<base-path>/api/v1/health/ready`.

Do not construct a second B2 client solely for health. Sharing the configured
provider instance is required so request coalescing and caching work across
both endpoint forms.

Preserve the existing two-second outer readiness deadline unless focused tests
prove it cannot bound the SDK call. Do not add a longer nested timeout.

The public response remains only:

```json
{"status":"ready"}
```

or:

```json
{"status":"not ready"}
```

The versioned endpoint may retain its equivalent existing generic problem
response. No dependency-specific failure details belong in either response.

## Focused verification

Automated tests must prove:

1. Local readiness succeeds after initialization and removes its probe.
2. A missing, unsafe, or unusable local Artifact root fails readiness.
3. A pre-canceled local check returns an unavailable error and performs no
   probe work.
4. B2 readiness sends one signed `ListObjectsV2` request with `MaxKeys=1` and
   prefix `v1/` and accepts an empty successful result.
5. B2 authentication failure, authorization failure, missing bucket, server
   failure, network failure, deadline, and cancellation match
   `ErrStorageUnavailable`.
6. Repeated successful checks within the success window issue one B2 request.
7. Repeated failed checks within the failure window issue one B2 request and
   return failure.
8. Deterministic clock advancement expires each cache window.
9. Concurrent checks coalesce into one B2 request, and canceled waiters return
   without starting another request or blocking the leader.
10. `StorageSet` checks only the configured default provider.
11. A storage failure makes both readiness endpoints return `503` while both
    liveness endpoints remain `200`.
12. Readiness responses do not contain private provider error text.
13. The application wiring shares the provider check between unversioned and
    versioned readiness paths.

Run focused checks only:

- `backend/internal/artifacts` unit tests, including the S3 HTTP stub;
- `backend/internal/platform/config` focused tests;
- `backend/internal/platform/httpserver` health tests;
- `backend/cmd/api` focused tests or compilation for wiring;
- one focused PostgreSQL-backed readiness integration test if needed to prove
  the real schema plus fake-provider composition;
- race detection for the concurrency-bearing readiness helper;
- `go vet` for affected packages;
- tracked-file `gofmt` and `git diff --check`.

Do not run Playwright, the frontend suite, full product acceptance, image
builds, or unrelated backend integration packages.

After automated checks pass, run one opt-in live B2 readiness check against the
existing `.env.local-test` configuration when its credentials are available.
The check must perform only the read-only list request, must not print secrets,
must not rebuild or restart the stack, and must not create or delete bucket
objects. If sandbox or credential access prevents this check, report it as
deferred rather than weakening the automated evidence.

## Documentation and memory

Update `.memory/implementation-status/completed.md` with:

- the previous readiness gap;
- the implemented default-provider readiness behavior;
- the B2 request type and cache bounds;
- exact focused verification evidence;
- whether the opt-in live B2 check ran.

Add a lesson only if implementation exposes a durable failure pattern that is
not already captured.

Do not update or move the deployment and import guides in this increment. Their
consolidation follows acceptance so the new guide describes verified behavior.

## Suggested implementation order

1. Add the provider readiness contract and update focused test adapters.
2. Implement deterministic local readiness.
3. Implement B2 `ListObjectsV2` readiness and error classification.
4. Add concurrency coalescing and bounded success/failure caching.
5. Add `StorageSet` default-provider readiness.
6. Separate the metadata-import directory probe from provider-owned checks.
7. Pass the shared storage set through both readiness endpoint paths.
8. Run the focused automated checks and race detection.
9. Run the opt-in non-mutating live B2 check when available.
10. Record the accepted behavior and evidence in memory.
11. Commit once and stop.

## Completion criteria

- B2 unavailability makes both readiness endpoints return `503` within the
  bounded cache window.
- Local storage unavailability has the same readiness effect.
- Liveness never depends on PostgreSQL, local storage, or B2.
- B2 readiness is non-persistent, bounded by context, coalesced, and briefly
  cached.
- A bucket-restricted key does not need `listAllBucketNames`; `listFiles` for
  the `v1/` namespace is the documented probe requirement.
- Both readiness endpoints share one configured provider instance.
- Error responses disclose no provider details or credentials.
- Focused tests and race detection pass.
- No schema, OpenAPI, frontend, Project Content, deployment-guide, stack,
  bucket-object, release, or VM changes are present.
- Existing ignored `.env.local-test`, `.DS_Store`, extractor output, and corpus
  files remain unstaged.
- One coherent commit uses a lowercase past-tense message with no prefix and no
  co-author trailer.

## Required handoff

Stop after the implementation commit. Report:

- commit hash and exact message;
- files changed;
- provider readiness interface and `StorageSet` behavior;
- exact local probe behavior;
- exact B2 request, required application-key capability, and error mapping;
- cache durations and concurrent request-coalescing design;
- both readiness and liveness endpoint behavior;
- automated commands and results;
- live B2 check result or explicit reason it was deferred;
- failures encountered and residual limitations;
- confirmation that no stack, volume, B2 object, image, release, or VM state
  changed;
- final `git status --short`.
