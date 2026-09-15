# VM in-place upgrade record, 2026-09-15, release 0.5

This sanitized record captures the first production-preserving AUSE Discovery
image upgrade at `https://life.au.edu/ause-discovery/`. It supersedes the fresh
reset procedure as the normal release path. The initial destructive deployment
and its import incidents remain recorded in `vm-deployment-2026-09-13.md`.

No passwords, session secrets, B2 application keys, private environment values,
or database contents are recorded here.

## Release scope

- Release source: main-repository commit `95bd9fc`
  (`removed intrusive search guidance and aligned controls`).
- Included Project Logo loading experience: Increment 16.
- Included newest-first academic search ordering and search schema version 2:
  Increment 17, commit `07db502`, corrected through `5f4bac7`.
- Included Search Filter Suggestions: Increment 18, commit `9278808`, corrected
  through `7cfc668`, with the post-acceptance visual correction in `95bd9fc`.
- PostgreSQL schema remained version 4. No database migration was added after
  the deployed 0.4 baseline.
- Compose, production environment shape, B2 configuration, storage layout, and
  catalog data were unchanged.
- Increment 17 required one Meilisearch rebuild after the new API started.
- Increment 18 required no backend, database, storage, or search-engine
  operation.

Both API and web were published as 0.5 from the same repository state. Although
Increment 18 was frontend-only, the 0.5 API was required to carry Increment 17
search ordering and schema behavior.

## Published Docker Hub images

Both builds targeted `linux/amd64`, matching the VM release architecture.

### API 0.5

```text
image: sastakro/ause-discovery-api:0.5
OCI index digest: sha256:a668c3f358960815bf05f6be6189864eab95ae4dbddc4262231513fb525969d1
linux/amd64 manifest digest: sha256:31f316ff4ca3523ebacef6c8151d47c96b86f4d4a9d6931a4da86f51afe93891
```

The recorded build and push completed in approximately 69 seconds.

### Web 0.5

```text
image: sastakro/ause-discovery-web:0.5
OCI index digest: sha256:9f67b100632631c8ad99651b13d2160e12163f75514feb543fdd3c4cb936c38a
linux/amd64 manifest digest: sha256:bf78d21b9161dd1fb07316578ac7137db76595827c6540aaac449a7bbd5cd98e
compiled public base path: /ause-discovery/
```

The recorded build and push completed in approximately 29 seconds.

`docker buildx imagetools inspect` confirmed one Linux AMD64 application
manifest plus the normal provenance attestation manifest for each tag.

## VM target

- Account and host: `saiaike@life.au.edu`
- Application directory: `/home/saiaike/apps/ause-discover`
- Compose project: `ause-discovery`
- Environment file: `.env`
- Override file: `compose.override.yaml`
- Loopback web binding: `127.0.0.1:8088:8080`
- Storage provider: Backblaze B2
- Previous images: API 0.4 and web 0.4.1
- New images: API 0.5 and web 0.5

The release used mutable 0.5 tags in the VM environment. The published OCI
index digests above remain the preferred references for a later environment
pinning pass.

## Pre-upgrade database backup

The existing production stack remained online while a PostgreSQL logical dump
was created:

```sh
cd /home/saiaike/apps/ause-discover
mkdir -p backups

docker compose -p ause-discovery --env-file .env \
  exec -T postgres \
  sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' \
  > "backups/ause-discovery-pre-0.5-$(date +%Y%m%d-%H%M%S).sql"
```

The command completed without an error in the captured log. The exact expanded
filename and file size were not captured. Future executions should follow the
dump with `ls -lh backups/` and a nonempty-file check before changing images.

## Successful in-place upgrade

The `.env` image references were changed to API and web 0.5. The existing
containers were not stopped before the new images were pulled:

```sh
docker compose -p ause-discovery --env-file .env config --quiet

docker compose -p ause-discovery --env-file .env pull

docker compose -p ause-discovery --env-file .env \
  run --rm migrate

docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api migrations status

docker compose -p ause-discovery --env-file .env \
  up -d --no-build --wait
```

Observed results:

- image pull completed in approximately 11 seconds for the slowest application
  image;
- migration output reported `no migrations to run. current version: 4`;
- migration status reported version 4 and `applied: true`;
- PostgreSQL and Meilisearch retained container ages of two days;
- only API and web were recreated for release 0.5;
- API became healthy approximately eight seconds after recreation;
- web became healthy approximately seven seconds after recreation;
- the Compose project remained `ause-discovery` throughout.

No `docker compose down` was required. In particular, no `down -v` or volume
removal occurred.

## Post-upgrade verification

```sh
docker compose -p ause-discovery --env-file .env ps

curl -fsS http://127.0.0.1:8088/ause-discovery/health/live
curl -fsS http://127.0.0.1:8088/ause-discovery/health/ready

docker compose -p ause-discovery --env-file .env \
  exec -T api printenv AUSE_ARTIFACT_STORAGE_BACKEND
```

The recorded responses were:

```text
{"status":"live"}
{"status":"ready"}
b2
```

Compose reported healthy PostgreSQL, Meilisearch, API, and web services. The
web service retained the loopback-only host binding.

## Increment 17 search rebuild

Search settings changed from schema version 1 to version 2, so a full rebuild
was queued after API 0.5 became healthy:

```sh
docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps \
  --entrypoint /usr/local/bin/ausectl \
  api search rebuild
```

Recorded operation:

```text
search rebuild 01a0a313-2a02-782d-a2f5-a0e85aa2b1ac queued
```

The CLI command proves queue creation and exits before background processing
finishes. The maintainer subsequently reported that the deployed application
worked correctly. For future releases that require a rebuild, confirm the final
operation state through `/ause-discovery/admin/search` before declaring the
search migration complete.

The rebuild uses PostgreSQL as the canonical source, constructs a replacement
Meilisearch index, and swaps it into service after completion. It does not read,
write, reconnect, or clear B2 objects.

## Data preserved and work deliberately omitted

The successful routine upgrade preserved:

- the production PostgreSQL named volume and every Project-to-storage-key
  association;
- all Backblaze B2 Project File and Project Logo objects;
- administrator accounts, sessions subject to normal expiry, audit history,
  imported metadata, Project Repository Links, and Project revisions;
- the existing Meilisearch service while the replacement index was prepared;
- the local `artifact-data` volume.

The upgrade did not perform:

- metadata CSV import;
- Project Content manifest dry-run or apply;
- Project Content bundle transfer to the VM;
- B2 listing, deletion, or upload;
- administrator creation;
- catalog validation or synchronization;
- Compose project deletion;
- named-volume deletion;
- Nginx, TLS, DNS, Compose, or environment-shape changes.

## Timing outcome

The captured image builds consumed approximately 98 seconds before deployment.
On the VM, the image pull took about 11 seconds and service replacement reached
health in about eight seconds. Backup creation and the asynchronous search
rebuild were not timed in the capture. No 2 GB Project Content transfer or
10-to-15-minute re-import occurred.

The deployment demonstrated that ordinary releases can remain comfortably
inside the desired 10-to-15-minute window when PostgreSQL and B2 are preserved
and release images are published before the VM upgrade begins.

## Reusable conclusions

- Routine production releases replace application images in place.
- Production PostgreSQL and B2 are one preservation boundary.
- `docker compose down -v`, B2 clearing, metadata import, and Project Content
  import belong only to a deliberate full reset.
- A PostgreSQL schema version and a Meilisearch schema version are independent.
  Release 0.5 kept PostgreSQL at version 4 but advanced search to version 2.
- A search schema or ranking-settings change requires a rebuild after the new
  API starts. A frontend-only increment does not.
- `ausectl search rebuild` queues work; completion requires a separate status
  check.
- Pulling while the old containers remain running minimizes interruption.
- Both API and web should use the same release number when one release spans
  backend and frontend increments.
- Linux AMD64 remains the recorded target for this VM.
- Published image index digests should be recorded and preferred over mutable
  tags in the VM environment.

## Rollback boundary

No PostgreSQL migration or storage change occurred, so an application rollback
does not require metadata import, Project Content import, or B2 repair. Restore
the previous API 0.4 and web 0.4.1 image references, pull if needed, and run
`docker compose up -d --no-build --wait`. If the earlier search ordering must
also be restored, queue a search rebuild after the old API is running.
