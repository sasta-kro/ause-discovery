# Project deployment commands and explanations

This note is the current operator reference for local Docker testing, publishing
Linux images to Docker Hub, and deploying the Compose stack on the VM. The
commands assume the repository base path `/ause-discovery/`.

Project metadata and Project Content remain external source-of-truth files
during development. A completely fresh stack is expected and does not require
preserving the current PostgreSQL, Meilisearch, local storage, or B2 contents.

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
curl -fsS http://localhost:8090/ause-discovery/health/live
curl -fsS http://localhost:8090/ause-discovery/health/ready
```

Use these local administrator credentials when the prompt appears:

```text
Username: Test1234567890
Password: Test1234567890
```

Before running the block, edit `.env.local-test` to include:

```dotenv
AUSE_WEB_PORT=8090
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
the other capabilities.

After startup, import Project metadata through the administrator Imports page:

```text
http://localhost:8090/ause-discovery/admin/imports
tools/ausesp-data-extractor/output/ause-discovery-projects-metadata-import.csv
```

Project Logo and Project File commands are in
`project-content-import.md`. Metadata must be committed before Project Content
can resolve `project_import_key` values.

## Quick copy: publish Linux amd64 images to Docker Hub

Replace `<release>` with the selected release, for example `0.3`. Run from the
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
immutable digest is safer in `.env.production` because it identifies the exact
image that was reviewed.

The repository's tagged release workflow currently publishes to GHCR. The
commands above are the separate manual Docker Hub publication path used by the
current VM release.

## Quick copy: deploy or refresh the VM

The VM must contain the current `compose.yaml` and a private `.env.production`.
Run from that deployment directory. Replace the image placeholders with the
digests reported after publication.

```dotenv
AUSE_ENV=production
AUSE_COOKIE_SECURE=true
MEILI_ENV=production

AUSE_API_IMAGE=sastakro/ause-discovery-api@sha256:<api-digest>
AUSE_WEB_IMAGE=sastakro/ause-discovery-web@sha256:<web-digest>

AUSE_PUBLIC_BASE_PATH=/ause-discovery/
AUSE_WEB_PORT=8090

AUSE_ARTIFACT_STORAGE_BACKEND=b2
AUSE_ARTIFACT_B2_ENDPOINT=s3.<region>.backblazeb2.com
AUSE_ARTIFACT_B2_BUCKET=<bucket-name>
AUSE_ARTIFACT_B2_KEY_ID=<application-key-id>
AUSE_ARTIFACT_B2_APPLICATION_KEY=<application-key-secret>
```

This snippet shows the release and storage values that usually change during
this cycle. The remaining `.env.production` values must also replace every
development secret: `POSTGRES_PASSWORD` and the matching password inside
`AUSE_DATABASE_URL`, both matching Meilisearch keys, `AUSE_SESSION_SECRET`,
and `AUSE_TRUSTED_PROXY_CIDRS`.

```sh
chmod 600 .env.production

docker compose -p ause-discovery --env-file .env.production config --quiet
docker compose -p ause-discovery --env-file .env.production pull

docker compose -p ause-discovery --env-file .env.production \
  up -d --wait postgres meilisearch

docker compose -p ause-discovery --env-file .env.production \
  run --rm migrate

docker compose -p ause-discovery --env-file .env.production \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api migrations status

docker compose -p ause-discovery --env-file .env.production \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api catalog validate

docker compose -p ause-discovery --env-file .env.production \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api catalog sync

docker compose -p ause-discovery --env-file .env.production \
  up -d --no-build --wait

docker compose -p ause-discovery --env-file .env.production ps
curl -fsS http://127.0.0.1:8090/ause-discovery/health/live
curl -fsS http://127.0.0.1:8090/ause-discovery/health/ready
```

For a brand-new database, create the first administrator after `catalog sync`:

```sh
docker compose -p ause-discovery --env-file .env.production \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api admin create
```

The VM reverse proxy continues to forward the public `/ause-discovery/` path
to `127.0.0.1:8090`. DNS and TLS remain outside this Compose stack.

## What runs in the stack

```text
Browser
  -> VM reverse proxy, production only
  -> web container, Nginx on host port 8090
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
the selected B2 bucket must also be emptied through the Backblaze console.
Otherwise old objects remain orphaned because the new PostgreSQL database no
longer contains their storage keys.

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
curl -fsS http://localhost:8090/ause-discovery/health/live
curl -fsS http://localhost:8090/ause-discovery/health/ready
```

`ps` shows container state, health, and port mappings. `curl -f` returns a
failure exit code for HTTP errors, `-s` hides progress, and `-S` still prints
errors.

Liveness proves that the Backend API process accepts HTTP requests. Readiness
checks the supported PostgreSQL schema, the metadata-import temporary
directory, and the configured default storage provider. B2 readiness uses a
read-only `ListObjectsV2` request limited to one result under `v1/`; successful
checks cache for 30 seconds and failures for 5 seconds. Meilisearch is omitted
from readiness because search degrades independently and can be rebuilt.

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

## VM release flow

The Docker Hub build commands publish `linux/amd64` images so the VM can pull
them even when the development machine uses another CPU architecture.
`--push` uploads the result directly instead of loading it into the local image
store.

The VM deployment sequence performs these operations:

1. `config --quiet` catches missing or malformed environment substitutions.
2. `pull` downloads the exact API and web images named in `.env.production`.
3. PostgreSQL and Meilisearch start before schema work.
4. `migrate` advances PostgreSQL to the schema supported by the new API.
5. `migrations status` confirms the applied version.
6. Catalog validation and synchronization load the catalog bundled with the
   new API image.
7. `up -d --no-build --wait` replaces changed containers and waits for health.
8. Local health calls verify the VM stack before public reverse-proxy testing.

For an upgrade, the existing administrator remains in PostgreSQL and
`admin create` is omitted. For the current development reset workflow, the VM
may be rebuilt from empty volumes, metadata imported from the reviewed CSV,
and Project Content imported from its bundle.

## Routine commands

```sh
docker compose -p ause-discovery --env-file .env.production ps
docker compose -p ause-discovery --env-file .env.production logs --tail 100 api web

docker compose -p ause-discovery --env-file .env.production \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api search rebuild

docker compose -p ause-discovery --env-file .env.production \
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

For the current development source-of-truth workflow, a fully clean reset is:

1. Delete the disposable Compose volumes.
2. Empty the selected B2 bucket through the Backblaze console.
3. Rebuild or pull the desired images.
4. Migrate and synchronize catalogs.
5. Import `ause-discovery-projects-metadata-import.csv` through the administrator Imports page.
6. Dry-run and apply the Project Content bundle.

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
docker compose -p ause-discovery --env-file .env.production \
  exec -T postgres pg_dump -U <postgres-user> <postgres-database> \
  > <backup-directory>/ause-discovery-<date>.sql
```

B2 backup or replication policy must preserve the bucket objects referenced by
that database snapshot. A database dump without matching storage objects, or
storage objects without the matching database, is not a complete restore set.

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
