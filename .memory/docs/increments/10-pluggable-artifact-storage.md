# Pluggable Artifact storage increment

## Assignment

Implement and commit the pluggable Artifact storage vertical for AUSE Discovery per AD-011.

This is one bounded increment. Extract the storage interface, add the per-Artifact backend marker, implement the Backblaze B2 backend through the S3-compatible API, wire configuration, add the storage migration command, and verify. Stop after a coherent commit. Do not continue into release tagging, VM deployment, or publication.

The accepted baseline is commit `5d89b71`. Treat that commit as working software. Preserve all existing behavior unless this brief explicitly requires a change.

Note: the split author-review workflow is suspended for this increment by the maintainer's instruction (2026-09-10). This session wrote the brief and implements it. Acceptance returns to the maintainer.

## Required reading before coding

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/project_context.md`
4. `.memory/architectural_decision_logs.md`, especially AD-011 (supersedes AD-008) and AD-010
5. `.memory/mvp_implementation_status.md`, backlog item 8
6. This brief

## Repository starting state

Verified directly before this brief:

- `backend/internal/artifacts/storage.go` defines the concrete `Storage{Root string, MaxBytes int64}` with `Put(ctx, reader, expectedSize) (StoredContent, error)`, `Open(storageKey) (*os.File, int64, error)`, `Exists(storageKey) bool`, and unexported `removeNew(storageKey) error`. Keys are opaque `v1/xx/yy/<uuid>` paths.
- `backend/internal/artifacts/service.go` defines `Service{Pool, Storage Storage, MaxProjectBytes}`. Storage is used at five sites: `Upload` (Put, removeNew), `transition` restore path (`Exists`), `Replace` (Put, removeNew), and `OpenPublic` (`Open`, mapping `os.ErrNotExist` and `ErrUnsafeStoragePath` to `ErrContentUnavailable`). All artifact SQL is handwritten in this file: two INSERTs, three UPDATE ... RETURNINGs, three SELECTs, and `scanArtifact`, every one listing the same 16 columns ending at `deleted_at`.
- `PublicContent{Artifact, File *os.File, Size}` is consumed by `serveArtifact` in `backend/internal/platform/httpserver/api.go` (~line 1546), which ends with `http.ServeContent(writer, request, content.Artifact.OriginalFilename, content.Artifact.UpdatedAt, content.File)`. `http.ServeContent` is the source of byte-range support.
- `artifacts.Storage` is constructed at exactly two sites: `backend/internal/platform/httpserver/api.go` `NewAPIHandler` (~line 62) and `backend/cmd/ausectl/main.go` `runArtifacts` (~line 153).
- `backend/internal/platform/config/config.go` holds typed configuration. `ArtifactRoot` defaults to `/var/lib/ause-discovery/artifacts`, is validated absolute, and is created and probed by `EnsureStorageDirectories` and `CheckStorageDirectories` (the latter feeds startup and container readiness).
- `backend/migrations/00001_initial_schema.sql` defines `artifacts.storage_key text NOT NULL UNIQUE`. There is exactly one migration; a second has never existed.
- `backend/cmd/ausectl/main.go` dispatches `artifacts seed-demo | artifacts import-manifest` through `parseArtifactCommand` with a flag set, dry-run by default, `--apply` to write, and a printed result summary.
- `backend/go.mod` has no S3 client. Pin candidates verified current on the Go module proxy (2026-09-10): `github.com/aws/aws-sdk-go-v2` v1.47.0, `.../service/s3` v1.113.0, `.../credentials` v1.20.4.
- `compose.yaml` forwards each `AUSE_` variable explicitly into the `api` service environment and mounts `artifact-data:/var/lib/ause-discovery`. The `migrate` service forwards only migration-relevant variables.
- The live bucket `ause-discovery-artifacts` verified on 2026-09-10: credentials work with no list capability, AES256 server-side encryption is active, and the S3 endpoint honors `Range` requests correctly (`Content-Range: bytes 10-15/36` round trip). The application key lives only in deployment environment files.

## Required outcome

An operator sets `AUSE_ARTIFACT_STORAGE_BACKEND=b2` (with the four B2 variables) and uploads, previews with byte ranges, downloads, replaces, and restores Artifacts exactly as today, with bytes landing in and served from B2 through the existing API endpoints. Visitors never learn the storage provider exists. Existing `local` deployments behave byte-for-byte identically to the current build. A migration command copies existing bytes to B2, verifies digests, and stamps records so mixed-backend serving works during and after migration. Upstream B2 failures produce controlled problem responses, never crashes or hangs.

## Scope

### Included

- Storage interface extraction and local backend adaptation in `backend/internal/artifacts`.
- B2 backend implementation over the pinned aws-sdk-go-v2 modules with ranged reads.
- Goose migration `00002` adding `artifacts.storage_backend`.
- Service and SQL changes to write and honor the marker.
- Configuration, including relaxation of artifact-directory checks when the backend is `b2`.
- One shared construction path for both current construction sites.
- `ausectl artifacts migrate` command (dry-run default, `--apply`) with per-artifact digest verification and idempotent re-runs.
- Upstream-failure mapping to a new 503 problem code.
- `compose.yaml` variable forwarding.
- Unit and PostgreSQL integration tests per this brief. No live B2 dependency in any test.
- `.env.example` comment correction (the block stops being "not yet consumed").
- Durable memory updates after verification.
- One coherent past-tense commit.

### Excluded

- OpenAPI contract changes (endpoints and wire types are untouched).
- Reverse migration (B2 to local) tooling. Rollback is covered by both backends remaining registered.
- B2 in CI: no workflow changes, no secrets in CI, no live-network tests.
- Bucket lifecycle rules, versioning, or CORS configuration.
- Frontend changes of any kind.
- Release tagging, image publication, VM deployment.
- Hard-deleting bytes of soft-deleted Artifacts (unchanged archival behavior).

## Contract decisions

1. `Backend` interface, in package `artifacts`:

   ```go
   type Backend interface {
       Name() string
       Put(ctx context.Context, reader io.Reader, expectedSize int64) (StoredContent, error)
       PutAt(ctx context.Context, storageKey string, reader io.Reader, expectedSize int64) (StoredContent, error)
       Open(ctx context.Context, storageKey string) (io.ReadSeekCloser, int64, error)
       Exists(ctx context.Context, storageKey string) bool
       RemoveNew(ctx context.Context, storageKey string) error
   }
   ```

   Context parameters reach the interface because B2 calls are network operations. The existing filesystem type becomes `LocalStorage` implementing `Backend`; its methods gain `ctx` (checked on the copy path as today through `contextReader`), `Open` returns `*os.File` as the `io.ReadSeekCloser`, and `removeNew` is exported as `RemoveNew`. `Put` generates a fresh key; `PutAt` writes under an explicit key, which migration uses so a record's storage key stays stable across backends (added during implementation: key-churning copies would orphan records against their stored `storage_key`).

2. `StorageSet` replaces the `Storage` field on `Service`:

   ```go
   type StorageSet struct {
       DefaultName string
       Backends    map[string]Backend
   }
   ```

   Every backend whose configuration is present is registered (local always, B2 when configured), regardless of which is default. Writes go through the default backend. Reads look up the backend named by the record's `storage_backend`; an unknown or missing name yields `ErrContentUnavailable`. This is what makes the per-record marker safe during migration and lets rollback be a default flip.

3. `PublicContent.File` becomes `io.ReadSeekCloser`. `http.ServeContent` accepts it unchanged; range support is preserved because the B2 reader below is seekable.

4. B2 `Open` returns a lazy ranged reader: a `HeadObject` supplies the size, and each `Seek` followed by reads issues at most one ranged `GetObject` using the `Range` header (verified against the live endpoint). `http.ServeContent` performs one seek per range response, so a ranged request costs one ranged GET and a full download costs one sequential GET. `Put` streams through `manager.Uploader` or a single `PutObject` with the same `MaxBytes` guard and digest computation as the local backend, and rejects `expectedSize` mismatches identically. `Exists` is a `HeadObject`. `RemoveNew` is a `DeleteObject` tolerating absence.

5. Error mapping: S3 `NoSuchKey` / HTTP 404 on read maps to `ErrContentUnavailable` (existing 404 `artifact_content_unavailable` behavior). Every other upstream failure (403, 429, 5xx, network, malformed response) maps to a new sentinel `ErrStorageUnavailable`. `OpenPublic` and the mutation paths translate it to HTTP `503` with problem code `artifact_storage_unavailable` through `writeArtifactError`. Upload failures roll back exactly as local failures do today, without leaving partial objects (B2 uploads are atomic per PUT).

6. Migration `00002_add_artifact_storage_backend.sql`:

   ```sql
   ALTER TABLE artifacts
       ADD COLUMN storage_backend text NOT NULL DEFAULT 'local'
       CHECK (storage_backend IN ('local', 'b2'));
   ```

   `scanArtifact` and every INSERT/UPDATE/RETURNING/SELECT column list in `service.go` gains `storage_backend`. The `Artifact` struct gains `StorageBackend string`. INSERTs write the default backend name.

7. Configuration: `Config` gains `ArtifactStorageBackend string` and `ArtifactB2Endpoint`, `ArtifactB2Bucket`, `ArtifactB2KeyID`, `ArtifactB2ApplicationKey` from the reserved `AUSE_ARTIFACT_*` names. Validation: backend must be `local` or `b2`; `b2` requires all four B2 values non-empty, in every environment. `ArtifactRoot` stays required absolute in all modes (it is the registered local backend's root and the second copy), but `EnsureStorageDirectories` and `CheckStorageDirectories` create and probe the artifact directory only when the backend is `local`. Readiness therefore does not fail a B2 deployment whose artifact root does not exist. `validateProductionSecrets` additionally requires `AUSE_ARTIFACT_B2_APPLICATION_KEY` in production when the backend is `b2`.

8. Construction: `artifacts.NewStorageSet(StorageOptions)` with plain fields (backend name, root, max bytes, B2 endpoint, bucket, key ID, application key). Both call sites map `config.Config` to `StorageOptions` in three lines. The B2 client is built from `aws.Config` with static credentials, a path-style custom endpoint resolver, and the region parsed from the endpoint host (`s3.us-west-004.backblazeb2.com` becomes `us-west-004`), so no `config` SDK module and no ambient credential chain is involved. The connection is created once per process.

9. Migration command: `ausectl artifacts migrate [--apply]`. Source is the local backend, target is the configured default backend; when the default is already `local`, the command exits with an explanatory error. Work list: `SELECT id, storage_key, byte_count, sha256 FROM artifacts WHERE storage_backend='local' ORDER BY created_at, id`. For each row: `LocalStorage.Open`, `PutAt` into the target under the same storage key with `expectedSize = byte_count`, compare the returned digest against the stored `sha256` (mismatch aborts without stamping), then `UPDATE artifacts SET storage_backend=$target WHERE id=$1 AND storage_backend='local'` (the guard makes concurrent re-runs safe). Dry-run prints the plan; apply prints scanned, copied, skipped-already-target, digest-verified, and failed counts. The loop lives in the `artifacts` package as an exported function so it is testable without the CLI.

10. `compose.yaml`: the `api` service environment forwards `AUSE_ARTIFACT_STORAGE_BACKEND`, `AUSE_ARTIFACT_B2_ENDPOINT`, `AUSE_ARTIFACT_B2_BUCKET`, `AUSE_ARTIFACT_B2_KEY_ID`, and `AUSE_ARTIFACT_B2_APPLICATION_KEY` with empty defaults. The `migrate` service is untouched.

## Implementation directives

Files in creation or modification order:

1. `backend/migrations/00002_add_artifact_storage_backend.sql` - contract decision 6.
2. `backend/internal/artifacts/storage.go` - interface, `StorageSet`, `LocalStorage` (same file, exported renames only where the interface requires).
3. `backend/internal/artifacts/storage_b2.go` - B2 backend per contract decisions 4 and 5, including the ranged reader.
4. `backend/internal/artifacts/service.go` - `StorageSet` usage, `StorageBackend` on the struct, all SQL column lists, `ErrStorageUnavailable` translation in `OpenPublic`.
5. `backend/internal/artifacts/migrate.go` - the exported migration loop.
6. `backend/internal/platform/config/config.go` - contract decision 7.
7. `backend/internal/platform/httpserver/api.go` - construction site, `PublicContent.File` type, `writeArtifactError` 503 mapping.
8. `backend/cmd/ausectl/main.go` - construction site, `migrate` command kind, usage string.
9. `compose.yaml` - contract decision 10.
10. `.env.example` - replace the "current build still serves from AUSE_ARTIFACT_ROOT" comment with a statement that the selection is live.

Dependency additions, exactly pinned: `github.com/aws/aws-sdk-go-v2 v1.47.0`, `github.com/aws/aws-sdk-go-v2/credentials v1.20.4`, `github.com/aws/aws-sdk-go-v2/service/s3 v1.113.0`. Resolve modules through the pinned Go container. Run `go mod tidy` and `go mod verify` there.

## Verification requirements

All through the pinned containers (`make` targets):

1. `go-test` green: adapted local-storage tests, new unit tests for the ranged reader (seek then read issues the expected ranged request; 404 maps to `ErrContentUnavailable`; 5xx maps to `ErrStorageUnavailable`) against an `httptest` S3 stub, `StorageSet` lookup semantics including unknown names, and the migration loop against a fake target backend (copies bytes, verifies digest, stamps, idempotent second run, digest-mismatch aborts unstamped).
2. `check-integration` green: existing artifact integration tests unchanged in behavior, plus one new PostgreSQL integration test covering: upload stamps the marker, serving reads through the marker-named backend (fake registered under `b2`), unknown marker yields the 404 problem, and the migration loop against the real schema.
3. `check-source`, `generate-check` (no drift), `check-compose` all green.
4. One live boundary check outside CI, performed by hand with `.env.local-test` against the real bucket: `AUSE_ARTIFACT_STORAGE_BACKEND=b2` stack, upload one small PDF through the API, `curl -r` the view URL for a 206, full download, then `ausectl artifacts migrate --apply` rehearsal on a locally seeded row set, then clean up the created objects through the B2 dashboard. This check is recorded in the handoff, not in the test suite.
5. `git diff --check` clean. Gofmt clean across modified files.

## Completion

After verification: update `.memory/mvp_implementation_status.md` backlog item 8 (implemented state), record any reusable lesson in `lessons_learned.md`, commit everything as one past-tense commit referencing AD-011, and hand off with the commit hash, the verification evidence, and the residual risks (notably: none of the automated tests exercise the live endpoint; the live check in item 4 is the only end-to-end proof).

Increment 10 ends here. Tagging `v*` and VM migration are subsequent maintainer-driven steps tracked as separate tasks.
