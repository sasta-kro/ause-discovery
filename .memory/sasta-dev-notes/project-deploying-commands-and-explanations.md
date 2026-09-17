# Project deployment commands and explanations

This note is the current operator reference for local Docker testing, publishing
Linux images to Docker Hub, and deploying the Compose stack on the VM. The
commands assume the repository base path `/ause-discovery/`.

The dated record `records/vm-deployment-2026-09-17-v0.6.md` captures the latest
in-place production upgrade, including image digests, catalog synchronization,
search-settings convergence, preserved data, and timing. The preceding
`records/vm-deployment-2026-09-15-v0.5.md` captures the first in-place upgrade
and its Increment 17 search rebuild. The earlier
`records/vm-deployment-2026-09-13.md` captures the initial fresh reset, imports,
errors, and fixes. This file keeps the reusable command flow.

Project metadata and Project Content remain external source-of-truth files
during development. The `ause-local-test` Compose project is fully disposable:
tear it down, delete its volumes, and rebuild it freely. Production is
different: the hosted `ause-discovery` Compose project and its B2 bucket carry
soft preservation. Everything is rebuildable from the source files, but a full
re-import costs roughly 10 to 15 minutes for about 2 GB, so routine upgrades
preserve the production PostgreSQL volume and B2 objects and never empty them.
A production wipe is a deliberate maintainer decision, used for data-model,
storage-layout, or corruption recovery, and resets PostgreSQL and B2 together.

## Quick copy: fresh local test stack

Create `.env.local-test` once, then set the values described immediately below
this block. Run the remaining commands from the repository root.

```sh
cp .env.example .env.local-test
chmod 600 .env.local-test
```

Deleting:
```
docker compose -p ause-local-test --env-file .env.local-test \
  down -v --remove-orphans
```

Testing config:
```
docker compose -p ause-local-test --env-file .env.local-test \
  config --quiet
```

Deploying:
```
docker compose -p ause-local-test --env-file .env.local-test \
  build api web

docker compose -p ause-local-test --env-file .env.local-test \
  up -d --wait postgres meilisearch

docker compose -p ause-local-test --env-file .env.local-test \
  run --rm migrate

docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api migrations status

docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api catalog validate

docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api catalog sync

docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api admin create

docker compose -p ause-local-test --env-file .env.local-test \
  up -d --no-build --wait

docker compose -p ause-local-test --env-file .env.local-test ps
curl -fsS http://localhost:8088/ause-discovery/health/live
curl -fsS http://localhost:8088/ause-discovery/health/ready

docker compose -p ause-local-test --env-file .env.local-test \
  exec -T api printenv AUSE_ARTIFACT_STORAGE_BACKEND
```

Use these local administrator credentials when the prompt appears:

```text
Username: Test1234567890
Password: Test1234567890
```

Before running the block, edit `.env.local-test` to include:

```dotenv
AUSE_WEB_PORT=8088
AUSE_API_IMAGE=ause-discovery-local-api:test
AUSE_WEB_IMAGE=ause-discovery-local-web:test

AUSE_ARTIFACT_STORAGE_BACKEND=b2
AUSE_ARTIFACT_B2_ENDPOINT=s3.<region>.backblazeb2.com
AUSE_ARTIFACT_B2_BUCKET=<bucket-name>
AUSE_ARTIFACT_B2_KEY_ID=<application-key-id>
AUSE_ARTIFACT_B2_APPLICATION_KEY=<application-key-secret>
```

The local credentials above are disposable. Production requires unique secrets.
The B2 application key needs bucket access with `listFiles`, `writeFiles`, and
`deleteFiles`. Readiness directly verifies `listFiles`; uploads and cleanup use
the other capabilities. The live storage-provider command above must print
`b2` before a Project Content import intended for B2. An environment file does
not change containers that already exist, so a stale `local` result requires a
container recreation after the Compose environment is corrected.

After startup, import Project metadata through the administrator Imports page:

```text
http://localhost:8088/ause-discovery/admin/imports
tools/ausesp-data-extractor/output/ause-discovery-projects-metadata-import.csv
```

Project Logo and Project File commands are in
`project-content-import.md`. Metadata must be committed before Project Content
can resolve `project_import_key` values.

## Quick copy: publish Linux amd64 images to Docker Hub

Replace `<release>` with the selected release, for example `0.4`. Run from the
repository root after the intended commit has been accepted.

```sh
docker login

docker buildx build \
  --platform linux/amd64 \
  --file backend/Dockerfile \
  --tag sastakro/ause-discovery-api:<release> \
  --push \
  .

docker buildx build \
  --platform linux/amd64 \
  --file frontend/Dockerfile \
  --build-arg VITE_PUBLIC_BASE_PATH=/ause-discovery/ \
  --tag sastakro/ause-discovery-web:<release> \
  --push \
  .

docker buildx imagetools inspect sastakro/ause-discovery-api:<release>
docker buildx imagetools inspect sastakro/ause-discovery-web:<release>
```

Record both published digests. A tag is convenient for publication; an
immutable digest is safer in `.env` because it identifies the exact
image that was reviewed.

The 2026-09-13 deployment published API and web `0.4`, then published web
`0.4.1` as a focused reverse-proxy correction. The deployed image index
digests were:

```text
sastakro/ause-discovery-api:0.4
sha256:9251b8c09d938c17a78ba2651db8fca1cad416921f0c5be5f891703432436ad4

sastakro/ause-discovery-web:0.4.1
sha256:1be451f6b0e5d9c93113f0778a3aea72aa69e5ffd14d5fb95c38b6d372623f9e
```

The 2026-09-15 in-place upgrade published API and web `0.5` from commit
`95bd9fc`. The image index digests were:

```text
sastakro/ause-discovery-api:0.5
sha256:a668c3f358960815bf05f6be6189864eab95ae4dbddc4262231513fb525969d1

sastakro/ause-discovery-web:0.5
sha256:9f67b100632631c8ad99651b13d2160e12163f75514feb543fdd3c4cb936c38a
```

The 2026-09-17 in-place upgrade published API and web `0.6` from commit
`2551425`. The image index digests were:

```text
sastakro/ause-discovery-api:0.6
sha256:e8b3fd0c981ec6c7b58e000043ad3a6fbf94424bb2939f4dda444afb498e8e70

sastakro/ause-discovery-web:0.6
sha256:ec26c5d41fc24516e1d1b86439f44f452b3ff6a3c0833ecd236f37b4f730bee8
```

The repository's tagged release workflow currently publishes to GHCR. The
commands above are the separate manual Docker Hub publication path used by the
current VM release.

## Quick copy: routine production image upgrade

Routine upgrades preserve PostgreSQL, B2, Meilisearch, and every named volume.
They do not use `down`, `down -v`, metadata import, Project Content transfer,
or B2 clearing. The successful 0.5 and 0.6 upgrades used this flow:

```sh
cd /home/saiaike/apps/ause-discover
mkdir -p backups

docker compose -p ause-discovery --env-file .env \
  exec -T postgres \
  sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' \
  > "backups/ause-discovery-pre-<release>-$(date +%Y%m%d-%H%M%S).sql"

docker compose -p ause-discovery --env-file .env config --quiet
docker compose -p ause-discovery --env-file .env pull

docker compose -p ause-discovery --env-file .env \
  run --rm migrate

docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api migrations status

docker compose -p ause-discovery --env-file .env \
  up -d --no-build --wait

docker compose -p ause-discovery --env-file .env ps
curl -fsS http://127.0.0.1:8088/ause-discovery/health/live
curl -fsS http://127.0.0.1:8088/ause-discovery/health/ready

docker compose -p ause-discovery --env-file .env \
  exec -T api printenv AUSE_ARTIFACT_STORAGE_BACKEND
```

Update the API and web image references in `.env` before `config` and `pull`.
Verify that the backup file is nonempty. The storage-provider output must remain
`b2`.

When a release changes version-controlled academic or taxonomy catalogs, run
validation and synchronization from the new API image before replacing the
long-running application services:

```sh
docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps \
  --entrypoint /usr/local/bin/ausectl \
  api catalog validate

docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps \
  --entrypoint /usr/local/bin/ausectl \
  api catalog sync
```

Release 0.6 required these commands to retire the `game` Platform catalog value.

When the release changes Meilisearch schema or ranking settings, queue a rebuild
after the new API becomes healthy:

```sh
docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps \
  --entrypoint /usr/local/bin/ausectl \
  api search rebuild
```

The command only queues the rebuild. Confirm completion through the
administrator Search page. Release 0.5 required this operation for Increment
17. Release 0.6 did not require a full rebuild: API startup patched the active
index with search-schema-version-3 facet settings.

## Quick copy: fresh VM deployment

The current VM deployment directory is
`/home/saiaike/apps/ause-discover`. It contains the current `compose.yaml`,
`compose.override.yaml`, and a private `.env`. Run all VM commands from that
directory. The reusable configuration below uses the tags deployed on
2026-09-17. Immutable digests remain preferable for later releases.

```dotenv
AUSE_ENV=production
AUSE_COOKIE_SECURE=true
MEILI_ENV=production

AUSE_API_IMAGE=sastakro/ause-discovery-api:0.6
AUSE_WEB_IMAGE=sastakro/ause-discovery-web:0.6

AUSE_PUBLIC_BASE_PATH=/ause-discovery/
AUSE_WEB_PORT=8088

AUSE_ARTIFACT_STORAGE_BACKEND=b2
AUSE_ARTIFACT_B2_ENDPOINT=s3.<region>.backblazeb2.com
AUSE_ARTIFACT_B2_BUCKET=<bucket-name>
AUSE_ARTIFACT_B2_KEY_ID=<application-key-id>
AUSE_ARTIFACT_B2_APPLICATION_KEY=<application-key-secret>
```

This snippet shows the release and storage values that usually change during
this cycle. The remaining `.env` values must also replace every
development secret: `POSTGRES_PASSWORD` and the matching password inside
`AUSE_DATABASE_URL`, both matching Meilisearch keys, `AUSE_SESSION_SECRET`,
and `AUSE_TRUSTED_PROXY_CIDRS`.

`AUSE_MEILISEARCH_API_KEY` and `MEILI_MASTER_KEY` must contain the same value.
A mismatch was found during the latest deployment and prevented normal search
integration until the services were recreated with matching keys.

The VM override prevents direct public access to the web-container port:

```yaml
services:
  web:
    ports: !override
      - "127.0.0.1:8088:8080"
```

The current `compose.yaml` must pass these values into the API service:

```yaml
AUSE_ARTIFACT_STORAGE_BACKEND: ${AUSE_ARTIFACT_STORAGE_BACKEND:-local}
AUSE_ARTIFACT_B2_ENDPOINT: ${AUSE_ARTIFACT_B2_ENDPOINT:-}
AUSE_ARTIFACT_B2_BUCKET: ${AUSE_ARTIFACT_B2_BUCKET:-}
AUSE_ARTIFACT_B2_KEY_ID: ${AUSE_ARTIFACT_B2_KEY_ID:-}
AUSE_ARTIFACT_B2_APPLICATION_KEY: ${AUSE_ARTIFACT_B2_APPLICATION_KEY:-}
```

```sh
chmod 600 .env

docker compose -p ause-discovery --env-file .env config --quiet
docker compose -p ause-discovery --env-file .env pull

docker compose -p ause-discovery --env-file .env \
  up -d --wait postgres meilisearch

docker compose -p ause-discovery --env-file .env \
  run --rm migrate

docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api migrations status

docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api catalog validate

docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api catalog sync

docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api admin create

docker compose -p ause-discovery --env-file .env \
  up -d --no-build --wait

docker compose -p ause-discovery --env-file .env ps
curl -fsS http://127.0.0.1:8088/ause-discovery/health/live
curl -fsS http://127.0.0.1:8088/ause-discovery/health/ready

docker compose -p ause-discovery --env-file .env \
  exec -T api printenv AUSE_ARTIFACT_STORAGE_BACKEND
```

The `admin create` command is required only for a brand-new database. Omit it
for an upgrade that preserves the existing PostgreSQL volume.

The VM host Nginx includes this application snippet inside the TLS server:

```nginx
location /ause-discovery/ {
    proxy_pass http://127.0.0.1:8088;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

The web image must preserve the incoming `X-Forwarded-Proto` value when it
proxies to the API. Web `0.4` overwrote `https` with the internal HTTP scheme,
causing authenticated writes through the public HTTPS route to fail same-origin
or CSRF validation. Web `0.4.1` contains the correction. DNS and TLS remain
outside the Compose stack.

After startup, import and commit Project metadata through:

```text
https://life.au.edu/ause-discovery/admin/imports
```

The current metadata source is:

```text
tools/ausesp-data-extractor/output/ause-discovery-projects-metadata-import.csv
```

Copy, dry-run, apply, verify, and remove the real Project Content staging
bundle using `project-content-import.md`.

## What runs in the stack

```text
Browser
  -> VM reverse proxy, production only
  -> web container, Nginx on host port 8088
  -> api container, Go Backend API
       -> PostgreSQL, canonical metadata
       -> Meilisearch, rebuildable search index
       -> local or B2 storage provider, Project File and Logo bytes
```

Compose defines five services:

| Service | Role | Lifetime |
|---|---|---|
| `postgres` | Canonical Projects, People, imports, users, audit events, and file metadata | Long-running |
| `meilisearch` | Derived public search documents | Long-running |
| `migrate` | Applies forward database migrations and exits | One-off |
| `api` | Backend API, `ausectl`, search reconciliation, and import cleanup | Long-running |
| `web` | React static files and the proxy to the Backend API | Long-running |

The named volumes hold PostgreSQL data, the rebuildable Meilisearch index, and
local application storage. With B2 selected, Project File and Logo bytes are
stored in B2. The local application volume still holds metadata-import
temporary files and remains available as the local provider for migration or
historical reads.

## Why each local command exists

### Copy and protect the environment file

```sh
cp .env.example .env.local-test
chmod 600 .env.local-test
```

`cp` creates a private working configuration from the tracked template.
`chmod 600` allows only the file owner to read or change it. The file contains
database, session, search, and B2 secrets and is ignored by Git.

### Delete the disposable local stack

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  down -v --remove-orphans
```

`down` removes the selected Compose project's containers and network. `-v`
also deletes that project's named volumes. `--remove-orphans` removes old
same-project services that are no longer declared. The `-p ause-local-test`
scope prevents this command from targeting the `ause-discovery` stack.

This command does not touch B2. When a completely fresh import is required,
the selected B2 bucket must also be emptied with the version-aware B2 CLI flow
in `project-content-import.md`. Otherwise old objects remain orphaned because
the new PostgreSQL database no longer contains their storage keys.

### Validate Compose

```sh
docker compose -p ause-local-test --env-file .env.local-test config --quiet
```

Compose reads `compose.yaml`, substitutes values from `.env.local-test`, and
validates the result. Success is silent. No image, container, volume, database,
or B2 object changes.

### Build the application images

```sh
docker compose -p ause-local-test --env-file .env.local-test build api web
```

The API build compiles the Backend API and `ausectl`, then copies migrations
and `config/` catalogs into the runtime image. The web build compiles the React
application and copies it into Nginx. A catalog YAML change therefore requires
an API rebuild before `catalog sync`.

The `migrate` service uses the API image and has no separate build target.

### Start only the data services

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  up -d --wait postgres meilisearch
```

`up` creates and starts the named services. `-d` keeps them in the background.
`--wait` blocks until their health checks succeed. The Backend API remains
stopped while the schema and catalogs are prepared.

### Apply and inspect migrations

```sh
docker compose -p ause-local-test --env-file .env.local-test run --rm migrate
```

`run` creates a one-off container from the `migrate` service. It executes
`ause-api migrate` from the same API image that will serve requests. `--rm`
removes the one-off container after it exits. Existing migration versions are
not applied twice.

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api migrations status
```

`--entrypoint` replaces the normal API server command with `ausectl`.
`--no-deps` avoids starting another dependency chain because PostgreSQL is
already running. The status command reports the applied schema version.

### Validate and synchronize catalogs

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api catalog validate

docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api catalog sync
```

`validate` checks the baked-in academic and taxonomy YAML without changing the
database. `sync` writes the validated catalog values into PostgreSQL. Repeated
synchronization preserves historical catalog versions.

### Create an administrator

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api admin create
```

The command reads the username and password interactively. The password does
not appear in a command flag, environment variable, shell history, or process
listing. PostgreSQL stores an Argon2id password hash.

### Start the application and wait for readiness

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  up -d --no-build --wait
```

`--no-build` uses the images built earlier. Compose starts migrations before
the Backend API and starts the web service after the API begins. `--wait`
waits for service health instead of returning while startup is still underway.

### Read status and health

```sh
docker compose -p ause-local-test --env-file .env.local-test ps
curl -fsS http://localhost:8088/ause-discovery/health/live
curl -fsS http://localhost:8088/ause-discovery/health/ready
```

`ps` shows container state, health, and port mappings. `curl -f` returns a
failure exit code for HTTP errors, `-s` hides progress, and `-S` still prints
errors.

Liveness proves that the Backend API process accepts HTTP requests. Readiness
checks the supported PostgreSQL schema, the metadata-import temporary
directory, and the configured default storage provider. B2 readiness uses a
read-only `ListObjectsV2` request limited to one result under `v1/`; successful
checks cache for 10 minutes and failures for 1 minute. Meilisearch is omitted
from readiness because search degrades independently and can be rebuilt.

The container healthcheck polls liveness only, on purpose. Readiness is a
deploy-time and on-demand signal: the compose `HEALTHCHECK` calls
`ause-api healthcheck`, which GETs `health/live`. Polling `health/ready` from
inside the container every 10 seconds re-probed B2 roughly 2,880 times per day
per API container, which alone exceeded the 2,500-per-day free B2 Class C
transaction cap and triggered repeated cap alert emails while development
stacks ran beside production. During deploys and troubleshooting, call
`health/ready` explicitly with the curl commands above instead of relying on
continuous polling.

## Environment values that commonly change

| Variable | Purpose |
|---|---|
| `AUSE_ENV` | `development`, `test`, or `production` |
| `AUSE_PUBLIC_BASE_PATH` | Runtime URL prefix used by the Backend API and web container |
| `VITE_PUBLIC_BASE_PATH` | URL prefix compiled into the web image |
| `AUSE_DATABASE_URL` | PostgreSQL connection string used inside Compose |
| `AUSE_MEILISEARCH_API_KEY` | Backend API credential for Meilisearch |
| `MEILI_MASTER_KEY` | Meilisearch server credential; must match the API key |
| `AUSE_SESSION_SECRET` | Secret material for administrator sessions |
| `AUSE_COOKIE_SECURE` | Requires HTTPS cookies when `true` |
| `AUSE_TRUSTED_PROXY_CIDRS` | Proxy networks allowed to supply forwarded headers |
| `AUSE_ARTIFACT_STORAGE_BACKEND` | Current configuration name for the default storage provider: `local` or `b2` |
| `AUSE_ARTIFACT_ROOT` | Local storage-provider root and import migration source |
| `AUSE_IMPORT_TEMP_ROOT` | Temporary directory for metadata import previews |
| `AUSE_ARTIFACT_B2_*` | B2 endpoint, bucket, key ID, and secret |
| `AUSE_API_IMAGE` | API image tag or immutable digest |
| `AUSE_WEB_IMAGE` | Web image tag or immutable digest |
| `AUSE_WEB_PORT` | Host port mapped to web-container Nginx |

`AUSE_PUBLIC_BASE_PATH` and `VITE_PUBLIC_BASE_PATH` must agree. The published
web image currently compiles `/ause-discovery/`; changing that path requires a
new web build.

## Fresh VM release flow

The Docker Hub build commands publish `linux/amd64` images so the VM can pull
them even when the development machine uses another CPU architecture.
`--push` uploads the result directly instead of loading it into the local image
store.

The VM deployment sequence performs these operations:

1. `config --quiet` catches missing or malformed environment substitutions.
2. `pull` downloads the exact API and web images named in `.env`.
3. PostgreSQL and Meilisearch start before schema work.
4. `migrate` advances PostgreSQL to the schema supported by the new API.
5. `migrations status` confirms the applied version.
6. Catalog validation and synchronization load the catalog bundled with the
   new API image.
7. `admin create` creates the initial operator only for a fresh database.
8. `up -d --no-build --wait` replaces changed containers and waits for health.
9. Local health calls verify the VM stack before public reverse-proxy testing.
10. The live API environment confirms that `b2` is the selected storage
    provider.
11. The administrator Imports page commits Project metadata from the reviewed
    CSV.
12. The Project Content bundle is dry-run, applied, rerun for idempotence, and
    removed from VM staging after verification.

For an upgrade, the existing administrator remains in PostgreSQL and
`admin create` is omitted. Upgrades preserve the production PostgreSQL volume
and B2 objects. Rebuilding the VM from empty volumes, with a fresh metadata
import and Project Content import, is a deliberate maintainer decision rather
than the routine path.

## Routine commands

```sh
docker compose -p ause-discovery --env-file .env ps
docker compose -p ause-discovery --env-file .env logs --tail 100 api web

docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api search rebuild

docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api admin reset-password --username <username>
```

`search rebuild` queues a complete Meilisearch rebuild from PostgreSQL. It is
appropriate after the search volume is lost or derived documents need full
replacement. Project detail pages remain PostgreSQL-backed.

## Stop, remove, and reset

```sh
docker compose -p ause-local-test --env-file .env.local-test stop
```

This stops containers while retaining containers, networks, and volumes.

```sh
docker compose -p ause-local-test --env-file .env.local-test down
```

This removes containers and the project network while retaining named volumes.

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  down -v --remove-orphans
```

This deletes the selected local project's containers, network, PostgreSQL,
Meilisearch, and local application volumes. B2 remains unchanged.

For the current development source-of-truth workflow, a fully clean reset is
routine for `ause-local-test` only. For the production `ause-discovery`
project the same sequence is a deliberate maintainer decision, roughly 10 to
15 minutes of re-import for about 2 GB, reserved for data-model changes,
storage-layout changes, or corruption recovery. Production PostgreSQL and B2
must be reset together or not at all:

1. Delete the disposable Compose volumes.
2. Empty every version and unfinished multipart upload in the selected B2
   bucket with the commands in `project-content-import.md`, only when that
   bucket is being deliberately reset.
3. Rebuild or pull the desired images.
4. Start PostgreSQL and Meilisearch.
5. Migrate, validate the migration status, and synchronize catalogs.
6. Create the administrator and start the complete stack.
7. Confirm local readiness and the live `b2` storage-provider value.
8. Import `ause-discovery-projects-metadata-import.csv` through the
   administrator Imports page.
9. Dry-run, apply, and rerun the Project Content bundle.
10. Remove only the VM staging bundle after all checks pass.

If PostgreSQL is reset without emptying B2, the existing objects become
unreferenced. If B2 is emptied without resetting PostgreSQL, Project File and
Logo metadata points to missing bytes.

## Backups when the deployment becomes valuable

The current development workflow deliberately rebuilds from source files. Once
the hosted database or uploaded content becomes authoritative, preserve both
sides of the mapping:

- PostgreSQL contains Project, Person, audit, and storage-key metadata.
- B2 contains Project File and Logo bytes when B2 is the default provider.
- The local `artifact-data` volume contains metadata-import temporary files and
  any locally stored or historical bytes.
- Meilisearch is derived and can be rebuilt.

A PostgreSQL logical dump uses:

```sh
docker compose -p ause-discovery --env-file .env \
  exec -T postgres pg_dump -U <postgres-user> <postgres-database> \
  > <backup-directory>/ause-discovery-<date>.sql
```

B2 backup or replication policy must preserve the bucket objects referenced by
that database snapshot. A database dump without matching storage objects, or
storage objects without the matching database, is not a complete restore set.

## Troubleshooting from the 2026-09-13 deployment

| Symptom | Cause | Correction |
|---|---|---|
| B2 remained empty after a local apply | The running API still had `AUSE_ARTIFACT_STORAGE_BACKEND=local` | Confirm Compose passes all B2 variables, recreate the API, and verify the live environment prints `b2` before importing |
| Metadata import could not complete normally | `AUSE_MEILISEARCH_API_KEY` and `MEILI_MASTER_KEY` differed | Set the same secret for both values and recreate Meilisearch and API |
| Authenticated writes failed only through public HTTPS | Web `0.4` replaced the host proxy's `X-Forwarded-Proto: https` with internal `http` | Deploy web `0.4.1`, which preserves a valid incoming forwarded protocol |
| `couldn't find env file: .../.env.local-test` on the VM | A local Compose command was copied unchanged | Use `-p ause-discovery --env-file .env` on the VM |
| `open /bundle/project-content-manifest.json: no such file or directory` | The local bundle mount path was used on the VM | Run from the VM deployment directory and mount `$PWD/project-content:/bundle:ro` |
| An `ause-local-test` network and volumes appeared on the VM | A wrong-project `docker compose run` created isolated resources | Run `docker compose -p ause-local-test --env-file .env down -v --remove-orphans` after confirming the project name |
| SSH disconnected after stack startup | The remote shell connection reset while containers continued running | Reconnect and inspect `docker ps` and Compose health before treating it as an application failure |

## Recovery notes

- A failed readiness check means the schema, import temporary directory, or
  default storage provider is unavailable. Liveness can remain healthy.
- A lost Meilisearch volume is recovered with `ausectl search rebuild`.
- An interrupted metadata import retains its server-side preview until cleanup
  expiry; the administrator can reopen it or upload the source again.
- An interrupted Project Content apply may leave earlier Projects committed.
  Rerunning the same manifest safely skips matching items and resumes missing
  work.
- Storage-provider failover and automatic second copies are not implemented.
  Changing the configured default controls new writes; existing records retain
  their recorded provider.
