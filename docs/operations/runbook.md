# AUSE Discovery operator runbook

This runbook covers deployment, routine operations, backup targets, restore ordering, upgrades, and recovery for the AUSE Discovery Compose stack. Commands use the repository service names (`postgres`, `meilisearch`, `migrate`, `api`, `web`) and the `ausectl` operator CLI. Values written as `<placeholder>` are environment-specific and must be supplied by the operating institution. No command below requires a secret in a flag, an environment example, or shell history.

## 1. Scope and invariants

- PostgreSQL is the canonical data store. All Project, Person, Artifact, identity, import, and audit records live there.
- Meilisearch holds derived, rebuildable search projections. It can be rebuilt completely from PostgreSQL.
- Artifact bytes and import temporary files share the `artifact-data` application volume in the current Compose topology.
- PostgreSQL data and Artifact bytes are the critical backup targets. Selected non-secret configuration and deployed image digests are recorded alongside them.
- Meilisearch data does not require authoritative backup.
- Automatic backups and automated disaster recovery are outside the MVP scope. Backup execution is a human-operated maintenance-window procedure documented in section 7.

## 2. Host prerequisites

- Docker Engine and the Docker Compose plugin from a currently supported release. The university-managed version is environment-specific; no exact version is mandated here.
- Disk capacity planning must cover, with headroom: container images and BuildKit caches; PostgreSQL data growth; Artifact bytes (bounded by a 250 MiB per-file limit and a 2 GiB active quota per Project); import temporary files; rotated container logs (bounded by the Compose defaults in `compose.yaml`); and backup staging space at least the size of PostgreSQL data plus Artifact bytes.
- Network exposure: only the `web` service publishes a host port (`AUSE_WEB_PORT`, default `8088`). PostgreSQL and Meilisearch are reachable only inside the Compose network. TLS termination and any external reverse proxy are institution-owned.
- The environment file and backup staging directory must be owned by the operator account with restrictive permissions (`chmod 600` for the environment file, `chmod 700` for the backup staging directory).

## 3. Configuration

1. Copy `.env.example` to an environment file named for the deployment, for example `.env.production`, and keep it outside source control. Compose reads `.env` by default; use `docker compose --env-file <file>` for a differently named file.
2. Required production values:
   - `AUSE_ENV=production` and `AUSE_COOKIE_SECURE=true` (enforced together by the API);
   - unique strong `POSTGRES_PASSWORD` and matching `AUSE_DATABASE_URL` credentials;
   - unique strong `AUSE_MEILISEARCH_API_KEY` and `MEILI_MASTER_KEY` (they must match each other);
   - a unique strong `AUSE_SESSION_SECRET`;
   - `AUSE_PUBLIC_BASE_PATH` matching the deployment URL path;
   - `AUSE_TRUSTED_PROXY_CIDRS` listing the reverse-proxy network range in CIDR form;
   - `MEILI_ENV=production`;
   - `AUSE_API_IMAGE` and `AUSE_WEB_IMAGE` set to complete registry references, preferably immutable digests such as `ghcr.io/<owner>/<repository>-api@sha256:<digest>`.
3. The Vite public base path is compiled into the web image. The published web image uses `/ause-discovery/`. A non-default production base path requires a matching web image build with `VITE_PUBLIC_BASE_PATH` set accordingly.
4. Institution-owned launch-time values that remain unresolved here: GitHub repository ownership, production domain, TLS certificates, VM details, branding, approved legal text, and final institutional catalog vocabulary.

## 4. Initial bootstrap

Run from the repository root with the environment file selected. The commands are ordered; do not skip steps.

1. Validate the rendered Compose configuration:

   ```sh
   docker compose --env-file <file> config --quiet
   ```

2. Pull the digest-pinned application images (or build locally only for a non-production rehearsal: `docker compose --env-file <file> build`):

   ```sh
   docker compose --env-file <file> pull
   ```

3. Start PostgreSQL and Meilisearch:

   ```sh
   docker compose --env-file <file> up -d --no-build postgres meilisearch
   ```

4. Apply forward migrations:

   ```sh
   docker compose --env-file <file> run --rm migrate
   ```

5. Check migration status:

   ```sh
   docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api migrations status
   ```

6. Synchronize the academic and taxonomy catalogs explicitly:

   ```sh
   docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api catalog validate
   docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api catalog sync
   ```

7. Create the first administrator. The command must run from a terminal because the password is read through a hidden prompt and is never accepted through flags or environment variables:

   ```sh
   docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api admin create
   ```

8. Start the API and web services:

   ```sh
   docker compose --env-file <file> up -d --no-build
   ```

9. Verify the deployment, substituting the host, port, and base path:

   ```sh
   curl -fsS http://localhost:<port>/<base-path>health/live
   curl -fsS http://localhost:<port>/<base-path>health/ready
   curl -fsSI http://localhost:<port>/<base-path>/
   ```

   Then open the site, confirm the public shell renders, and sign in as the administrator at `/<base-path>admin/login`.

## 5. Routine administrator operations

All commands use the same `--env-file` selection as bootstrap. `ausectl` is invoked through the API image entrypoint.

- Create an administrator: `docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api admin create`
- Reset an administrator password: `docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api admin reset-password --username <username>`
- Disable an administrator: `docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api admin disable --username <username>`
- Validate catalogs: `docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api catalog validate`
- Synchronize catalogs: `docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api catalog sync`
- Check migration status: `docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api migrations status`
- Queue one Project reindex: `docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api search reindex-project --project-id <project-uuid>`
- Queue a full search rebuild: `docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api search rebuild`
- Seed realistic demonstration files or import real Project files in bulk: follow `docs/operations/project-file-import.md`.
- Service status and bounded recent logs:

  ```sh
  docker compose --env-file <file> ps
  docker compose --env-file <file> logs --tail 100 api
  docker compose --env-file <file> logs --since 15m api web
  ```

## 6. Health and observability

- Liveness: `/<base-path>health/live` returns `200` when the API process is serving.
- Readiness: `/<base-path>health/ready` returns `200` only when PostgreSQL exposes the supported applied migration and the Artifact and import directories accept a temporary write probe. A readiness failure with a live process requires investigation of database connectivity, schema compatibility, and storage availability. The container health check calls this HTTP endpoint, so it also detects an unavailable API listener.
- When Meilisearch is unavailable, the API stays available: Project and Person detail pages work, while search returns a controlled service-unavailable response and pending synchronization retries automatically.
- Production logs are structured JSON including timestamp, level, request ID, and a stable code where applicable. Request IDs are also returned in the `X-Request-ID` response header.
- `docker compose logs` output can contain operational detail; do not paste logs from production into public channels. Never print the environment file.

## 7. Backup targets and maintenance-window procedure

Backups are documented, not automated.

- Canonical metadata backup: a PostgreSQL logical dump:

  ```sh
  docker compose --env-file <file> exec -T postgres pg_dump -U <postgres-user> <postgres-db> > <staging-dir>/ause-discovery-<date>.sql
  ```

- Canonical byte backup: the Artifact and import volume, archived with the same pinned PostgreSQL image the stack already uses:

  ```sh
  docker run --rm -v ause-discovery_artifact-data:/data:ro \
    -v <staging-dir>:/backup postgres:18.6-alpine@sha256:d3e1620b530c944afa6e887d22eb899824da68e19c52024bf98f5220c88a65b2 \
    tar -czf /backup/ause-discovery-artifacts-<date>.tar.gz -C /data .
  ```

- Record a small manifest containing the deployment date, `AUSE_PUBLIC_BASE_PATH`, the exact `AUSE_API_IMAGE` and `AUSE_WEB_IMAGE` digests, and the migration version shown by `ausectl migrations status`. Do not include secrets.
- Because PostgreSQL and the Artifact volume are separate filesystems, take a maintenance window or write pause (stop the API service) so the dump and the byte archive describe the same instant.
- Verify checksums of the produced files (`shasum -a 256`) and rehearse a restore into a disposable target at least once before treating any archive as a valid backup. An unverified archive is not a valid backup.
- Retention, encryption, access control, and off-host storage are institution-owned policy. Keep at least one copy where compromise of the application host does not imply destruction of all copies.

## 8. Restore ordering

Restoration is destructive for the target environment. Confirm the target database and volume names before running anything, and prefer rehearsing against a disposable Compose project first.

1. Stop API writes: `docker compose --env-file <file> stop api web`
2. Restore PostgreSQL into the intended empty or explicitly approved target:

   ```sh
   docker compose --env-file <file> exec -T postgres psql -U <postgres-user> -d <target-db> < <staging-dir>/ause-discovery-<date>.sql
   ```

3. Restore Artifact bytes with expected ownership and permissions into the `artifact-data` volume.
4. Apply any newer forward migrations: `docker compose --env-file <file> run --rm migrate`
5. Synchronize catalogs: `docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api catalog sync`
6. Start the API: `docker compose --env-file <file> up -d --no-build api`
7. Rebuild Meilisearch from PostgreSQL: `docker compose --env-file <file> run --rm --no-deps --entrypoint /usr/local/bin/ausectl api search rebuild`
8. Verify representative Projects, People, and Artifact downloads.
9. Start or fully expose the web service: `docker compose --env-file <file> up -d --no-build web`

Do not execute this procedure against persistent development data during normal work.

## 9. Upgrade and rollback

1. Record the currently deployed API and web image digests: `docker images --digests | grep ause-discovery` or `docker compose --env-file <file> images`.
2. Take and verify the section 7 backups before any schema-changing upgrade.
3. Update `AUSE_API_IMAGE` and `AUSE_WEB_IMAGE` to the replacement digest references and pull:

   ```sh
   docker compose --env-file <file> pull
   ```

4. Run migrations before the new API becomes ready: `docker compose --env-file <file> up -d --no-build` starts `migrate` ahead of `api` by service ordering; for an explicit pass run `docker compose --env-file <file> run --rm migrate` first.
5. Synchronize catalogs explicitly, then verify health and representative behavior.
6. Database migrations are forward-only. Rolling an application image back is safe only when the previous application remains compatible with the already-migrated schema.
7. Blind database rollback is prohibited. Recovery from a bad migration uses the section 8 restore procedure with a verified backup, not a down migration.

## 10. Recovery scenarios

- Meilisearch unavailable or lost: keep PostgreSQL running, restart or recreate the Meilisearch service, then queue a full rebuild (`ausectl search rebuild`) and observe completion in the administration search-maintenance view. Public detail pages remain available throughout.
- PostgreSQL unavailable: the API reports not ready and rejects writes. Keep the API stopped or unavailable, diagnose storage and service health, and do not redirect writes anywhere else.
- Missing Artifact bytes: PostgreSQL metadata stays canonical; the affected download fails safely with a controlled response. Restore the bytes from the Artifact backup and re-verify the affected files.
- Disk pressure: inspect named volumes, rotated logs, stopped containers, and old application images (`docker system df -v`). Remove only confirmed disposable resources such as old untagged application image layers. Never remove persistent volumes casually.
- Interrupted import: expired or abandoned import batches are cleaned automatically; temporary upload state is removed by expiry. Do not delete committed Projects to clean up an interrupted import.

## 11. Shutdown and data preservation

- Normal shutdown: `docker compose --env-file <file> down`. Named volumes persist.
- `docker compose down` removes containers and networks only. Volume-deleting commands such as `docker compose down -v` or `docker volume rm` destroy persistent data and are not normal operational commands.
- `docker compose down -v` deletes the PostgreSQL, Meilisearch, and Artifact volumes. Never run it against a production or valued development environment.

## 12. Manual deployment boundary

- Deployment is human-operated through this runbook. Repository CI never deploys to a VM, never runs SSH, and never contacts the production host.
- Deployment inputs are the published image digests emitted by the tagged release workflow.
- TLS termination, DNS, and any institution reverse proxy in front of the `web` service are environment-specific and remain externally owned.
- CI publishes images only for the configured tagged-release event. No automated deployment follows publication.
