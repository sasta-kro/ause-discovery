# VM deployment record, 2026-09-12 to 2026-09-13

This sanitized record captures the local rehearsal, image publication, VM
reset, metadata import, Project Content import, reverse-proxy correction, and
post-import cleanup used for the AUSE Discovery deployment at
`https://life.au.edu/ause-discovery/`.

No passwords, session secrets, B2 application keys, or private environment
file contents are recorded here. The earlier unversioned VM deployment log was
removed from Git history on 2026-09-09 during a secret-leak cleanup. This dated
record replaces its operational purpose without restoring sensitive material.

## Final deployed state

- VM account and host: `saiaike@life.au.edu`
- VM application directory: `/home/saiaike/apps/ause-discover`
- Compose project: `ause-discovery`
- VM environment file: `.env`
- Public base path: `/ause-discovery/`
- Loopback web binding: `127.0.0.1:8088:8080`
- Artifact storage provider: Backblaze B2
- API image: `sastakro/ause-discovery-api:0.4`
- API image index digest:
  `sha256:9251b8c09d938c17a78ba2651db8fca1cad416921f0c5be5f891703432436ad4`
- Web image: `sastakro/ause-discovery-web:0.4.1`
- Web image index digest:
  `sha256:1be451f6b0e5d9c93113f0778a3aea72aa69e5ffd14d5fb95c38b6d372623f9e`
- PostgreSQL migration version: `4`
- Imported metadata Projects: `205`
- Project Content plan: `205` Projects, `68` Logos, `332` Project Files,
  `15` Repository Links across `13` Projects, `1965.9 MiB`

The API remained at `0.4`. The web image advanced from `0.4` to `0.4.1` after
the HTTPS forwarded-protocol bug was found during the administrator metadata
import.

## Local rehearsal

The local stack used the same B2 bucket and Project Content bundle intended for
the VM. The disposable Compose project was `ause-local-test` and the private
environment file was `.env.local-test`.

### Reset local state and B2

The B2 CLI was authenticated from the application environment without copying
secret values into shell history:

```sh
set -a
source .env.local-test
set +a

export B2_APPLICATION_KEY_ID="$AUSE_ARTIFACT_B2_KEY_ID"
export B2_APPLICATION_KEY="$AUSE_ARTIFACT_B2_APPLICATION_KEY"
```

All object versions and unfinished multipart uploads were removed:

```sh
b2 rm \
  --versions \
  --recursive \
  --dry-run \
  "b2://$AUSE_ARTIFACT_B2_BUCKET"

b2 rm \
  --versions \
  --recursive \
  "b2://$AUSE_ARTIFACT_B2_BUCKET"

b2 file large unfinished cancel \
  "b2://$AUSE_ARTIFACT_B2_BUCKET"

b2 ls \
  --versions \
  --recursive \
  --long \
  "b2://$AUSE_ARTIFACT_B2_BUCKET"
```

The final listing was empty. The local Compose data was then removed:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  down -v --remove-orphans
```

### Bootstrap and import

The current application images were built, PostgreSQL and Meilisearch were
started, migrations and catalogs were applied, an administrator was created,
and the complete stack was started. Project metadata was then uploaded and
committed at:

```text
http://localhost:8088/ause-discovery/admin/imports
```

The real Project Content bundle was dry-run and applied from
`resources/REAL_IMPORT_BUNDLE/`:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/resources/REAL_IMPORT_BUNDLE:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content-manifest.json \
  --actor-username Test1234567890 \
  --workers 4
```

The dry-run reported the expected `205`, `68`, `332`, `15`, and `13` counts.
The same command with `--apply` completed all `205` Projects and reported
`1965.9 MiB` planned bytes.

### Local storage-provider error

#### Symptom

The Project Content command appeared to succeed, but the B2 listing remained
empty. The running API reported:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  exec -T api printenv AUSE_ARTIFACT_STORAGE_BACKEND
```

```text
local
```

#### Cause

The B2 values in `.env.local-test` did not prove that the existing API
container had received them. Compose environment files are applied when a
container is created. An already-running container retains its old environment.
The Compose definition must also pass every `AUSE_ARTIFACT_B2_*` variable into
the API service.

#### Solution

The Compose API environment was confirmed to include the storage-provider and
B2 variables, then the stack was recreated from the selected environment file.
The live check then returned:

```text
b2
```

This live-container check is now required before a bulk B2 import.

## Image publication

Linux AMD64 API and web images were published to Docker Hub as `0.4`. The web
proxy correction was then built and published separately as `0.4.1`:

```sh
docker buildx build \
  --platform linux/amd64 \
  --file backend/Dockerfile \
  --tag sastakro/ause-discovery-api:0.4 \
  --push \
  .

docker buildx build \
  --platform linux/amd64 \
  --file frontend/Dockerfile \
  --build-arg VITE_PUBLIC_BASE_PATH=/ause-discovery/ \
  --tag sastakro/ause-discovery-web:0.4.1 \
  --push \
  .
```

Registry inspection confirmed Linux AMD64 manifests and the digests listed in
the final deployed state.

## VM preparation

The deployment files were kept under the existing account-owned application
directory rather than `/srv`:

```text
/home/saiaike/apps/ause-discover
```

The complete bundle was copied to:

```text
/home/saiaike/apps/ause-discover/project-content
```

A reusable transfer command from the local repository root is:

```sh
rsync -avh --progress \
  resources/REAL_IMPORT_BUNDLE/ \
  saiaike@life.au.edu:/home/saiaike/apps/ause-discover/project-content/
```

The VM's Compose file was updated to pass the B2 storage configuration into
the API container. The VM `.env` was updated with production secrets, matching
Meilisearch keys, B2 selection and credentials, base path, port, and image
tags. The private file remained mode `600`.

The VM override bound the web service only to loopback:

```yaml
services:
  web:
    ports: !override
      - "127.0.0.1:8088:8080"
```

## Fresh VM stack bootstrap

The current development deployment intentionally reset PostgreSQL,
Meilisearch, and the application volume together with the B2 bucket. The
following commands were run from `/home/saiaike/apps/ause-discover`:

```sh
docker compose -p ause-discovery --env-file .env \
  down -v --remove-orphans

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
```

Migration output confirmed schema version `4`. Catalog validation and
synchronization succeeded. PostgreSQL, Meilisearch, API, and web reported
healthy after startup.

An SSH connection reset occurred after startup. Reconnecting and running
`docker ps` confirmed that the stack had remained healthy. The disconnect was
not an application failure.

## Administrator metadata-import failures

### Meilisearch key mismatch

The API and Meilisearch initially used different keys. The VM `.env` must keep
these values identical:

```dotenv
AUSE_MEILISEARCH_API_KEY=<same-secret>
MEILI_MASTER_KEY=<same-secret>
```

The values were corrected and the affected services were recreated.

### HTTPS forwarded protocol lost inside the web container

#### Symptom

The administrator Imports page loaded, but its authenticated write failed when
used through `https://life.au.edu`. The VM Nginx correctly sent
`X-Forwarded-Proto: https` to the web container. The web-container Nginx then
replaced it with its own connection scheme, `http`, before proxying to the API.
The API compared the browser's HTTPS `Origin` with the forwarded HTTP scheme
and rejected the request as a same-origin or CSRF failure.

#### Fix

The web Nginx template now preserves a valid incoming forwarded protocol:

```nginx
map $http_x_forwarded_proto $ause_forwarded_proto {
    default $scheme;
    http http;
    https https;
}
```

Both API and health proxy locations now use:

```nginx
proxy_set_header X-Forwarded-Proto $ause_forwarded_proto;
```

The correction was published as
`sastakro/ause-discovery-web:0.4.1`. The VM pulled and recreated the relevant
services:

```sh
docker compose -p ause-discovery --env-file .env pull web

docker compose -p ause-discovery --env-file .env \
  up -d --no-build --force-recreate meilisearch api web
```

The administrator CSV import then completed successfully through the public
HTTPS route.

## VM Project Content import

Two incorrect command variants were attempted before the successful import.

### Wrong Compose project and environment file

The local command was copied to the VM unchanged:

```text
-p ause-local-test --env-file .env.local-test
```

The VM did not contain `.env.local-test`, producing:

```text
couldn't find env file: /home/saiaike/apps/ause-discover/.env.local-test
```

The VM command must use:

```text
-p ause-discovery --env-file .env
```

The failed attempt created an accidental `ause-local-test` network and named
volumes during the later variant that used `.env`. They were removed after the
deployment with:

```sh
docker compose -p ause-local-test --env-file .env \
  down -v --remove-orphans
```

### Wrong bundle mount

The local source path was also copied unchanged:

```text
$PWD/resources/REAL_IMPORT_BUNDLE
```

That directory did not exist on the VM, so the container reported:

```text
open manifest: open /bundle/project-content-manifest.json: no such file or directory
```

The actual VM staging directory was `$PWD/project-content`, so the successful
dry-run was:

```sh
docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps \
  --volume "$PWD/project-content:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content-manifest.json \
  --actor-username Test1234567890 \
  --workers 4
```

It reported:

```text
mode: dry-run
Projects: 205
logo uploads: 68
logo replacements: 0
file uploads: 332
unchanged skips: 0
repository links declared: 15
link sets replaced: 13
link sets unchanged: 0
```

The same command with `--apply` began a `1965.9 MiB` B2 upload with four
workers. Completion required all `205` progress lines, the final `mode: apply`
summary, no failed Projects, and a zero exit status.

## Post-import verification and cleanup

Before removing the staging bundle, the import was checked for completion and
the same manifest was dry-run again to confirm idempotent skips. Stack and
public readiness checks used:

```sh
docker compose -p ause-discovery --env-file .env ps

curl -fsS http://127.0.0.1:8088/ause-discovery/health/live
curl -fsS http://127.0.0.1:8088/ause-discovery/health/ready

curl -fsS https://life.au.edu/ause-discovery/health/live
curl -fsS https://life.au.edu/ause-discovery/health/ready
```

The Project Content directory was a staging copy only. After successful B2 and
database association, it was removed from the VM:

```sh
realpath /home/saiaike/apps/ause-discover/project-content
du -sh /home/saiaike/apps/ause-discover/project-content
rm -rf /home/saiaike/apps/ause-discover/project-content
```

The original local bundle remains the development source of truth. Removing
the VM staging copy does not delete B2 objects or PostgreSQL associations.
Production Compose volumes must not be removed after this point unless a
coordinated PostgreSQL and B2 reset is intended.

## Host Nginx configuration

The existing TLS server includes an AUSE-specific snippet. The working snippet
is:

```nginx
location /ause-discovery/ {
    proxy_pass http://127.0.0.1:8088;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

The host Nginx remains responsible for TLS. The container Nginx must preserve
the incoming `X-Forwarded-Proto` value as described above.

## Remaining operational work

- PostgreSQL and B2 must be backed up as one coherent restore set once hosted
  administrator changes become authoritative.
- Meilisearch is derived and can be rebuilt from PostgreSQL.
- The VM reported pending operating-system updates and a required restart.
  That maintenance is separate from the application deployment.
- Exact image digests are preferred over mutable tags for later releases.
