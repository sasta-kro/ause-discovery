# VM in-place deployment record, 2026-09-17, release 0.6

This sanitized record captures the AUSE Discovery 0.6 publication and
production-preserving VM upgrade at
`https://life.au.edu/ause-discovery/`. It follows the routine image-replacement
path proven by release 0.5 and preserves PostgreSQL, Backblaze B2, Meilisearch,
and every named Compose volume.

No passwords, session secrets, B2 application keys, private environment values,
or database contents are recorded here.

## Release source and scope

- Release source: main-repository commit
  `255142539f533b8f03577795483d1d9d540a6912`.
- Short commit: `2551425` (`my tweaks on ui (deployment 0.6)`).
- Local branch: `main`, clean at publication time and 35 commits ahead of the
  recorded remote tracking branch.
- Previous deployed release: API and web 0.5 from commit `95bd9fc`.
- New deployed release: API and web 0.6.
- PostgreSQL schema remained version 4.
- Meilisearch search schema advanced from version 2 to version 3 through the
  complete-Person-facet correction.
- The version-controlled `game` Platform value was retired while the separate
  Game Category remained active.
- No Compose, environment-shape, storage-layout, OpenAPI-shape, or PostgreSQL
  migration change was required.

Release 0.6 includes the accepted work after 0.5:

- Increment 19, Person-name Search Filter Suggestions;
- Increment 20, filter ordering, sort controls, and expansion focus;
- Increment 21, route-transition scroll reset;
- Increment 22, complete People and Advisor facet distributions with search
  schema version 3;
- Increment 23, developer attribution, administrator cookie disclosure, and
  current public policy copy;
- the direct Platform `game` retirement;
- B2 readiness-traffic reduction and liveness-only container health checks;
- public landing, branding, favicon, faculty crest, base-path, and final UI
  adjustments through the release commit.

## Published Docker Hub images

Both application images target `linux/amd64`, matching the VM architecture.
PostgreSQL and Meilisearch remained pinned third-party images. The Compose
`migrate` service continued to reuse the API image.

### API 0.6

```text
image: sastakro/ause-discovery-api:0.6
OCI index digest: sha256:e8b3fd0c981ec6c7b58e000043ad3a6fbf94424bb2939f4dda444afb498e8e70
linux/amd64 manifest digest: sha256:a0787d45c4bb5a28d9c9ff6c5545c238e003fcc43e6549d5852da87b52e581a8
```

The captured API build and push completed in approximately 85 seconds.

### Web 0.6

```text
image: sastakro/ause-discovery-web:0.6
OCI index digest: sha256:ec26c5d41fc24516e1d1b86439f44f452b3ff6a3c0833ecd236f37b4f730bee8
linux/amd64 manifest digest: sha256:ac02b9d8edcf871611012526570ad3a7cfcedaf748a90e8bd8bcb933a111c06c
compiled public base path: /ause-discovery/
```

The web build and push output was not included in the captured publication log.
Registry inspection nevertheless confirmed the 0.6 OCI index, Linux AMD64
application manifest, and provenance attestation manifest.

The VM used mutable 0.6 tags. The index digests above remain preferable for a
future environment-pinning pass.

## VM target

- Account and host: `saiaike@life.au.edu`
- Application directory: `/home/saiaike/apps/ause-discover`
- Compose project: `ause-discovery`
- Environment file: `.env`
- Override file: `compose.override.yaml`
- Loopback web binding: `127.0.0.1:8088:8080`
- Artifact storage provider: Backblaze B2
- Previous images: API 0.5 and web 0.5
- New images: API 0.6 and web 0.6

## Pre-upgrade PostgreSQL backup

The existing production stack stayed online while a logical dump was created:

```sh
docker compose -p ause-discovery --env-file .env \
  exec -T postgres \
  sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' \
  > "backups/ause-discovery-pre-0.6-$(date +%Y%m%d-%H%M%S).sql"
```

The captured backup directory contained:

```text
ause-discovery-pre-0.5-20260915-101540.sql
ause-discovery-pre-0.6-20260917-112548.sql
```

This proves creation of the named 0.6 dump file. The capture did not include
its byte size. Future runs should still use `ls -lh backups/` and verify that
the newest dump is nonempty before changing images.

## Image pull and schema verification

After the VM `.env` was updated to API and web 0.6:

```sh
docker compose -p ause-discovery --env-file .env config --quiet
docker compose -p ause-discovery --env-file .env pull

docker compose -p ause-discovery --env-file .env \
  run --rm migrate

docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps \
  --entrypoint /usr/local/bin/ausectl \
  api migrations status
```

Observed results:

- API 0.6 pulled in approximately 11.4 seconds;
- web 0.6 pulled in approximately 10.5 seconds;
- the pinned PostgreSQL and Meilisearch images were checked or pulled by the
  ordinary Compose pull;
- Goose reported no migrations to run and current version 4;
- `ausectl migrations status` reported version 4 and `applied: true`.

No PostgreSQL migration existed between releases 0.5 and 0.6.

## Catalog validation and synchronization

Release 0.6 changed the version-controlled taxonomy catalog, so the new API
image was used to validate and synchronize it before application replacement:

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

Both commands succeeded. Synchronization marks the Platform value `game` as
retired and keeps its stable database identity. The distinct Game Category is
unchanged. The current reviewed 205-row metadata CSV contains no Project using
`game` as a Platform, so no Project metadata re-import or assignment cleanup was
required.

Catalog retirement changes catalog availability. It does not delete Projects,
Project assignments, B2 objects, or Project Content associations.

## Successful in-place service replacement

```sh
docker compose -p ause-discovery --env-file .env \
  up -d --no-build --wait
```

Compose reported all five service outcomes successfully:

- PostgreSQL healthy, existing container age four days;
- Meilisearch healthy, existing container age four days;
- migration service exited successfully;
- API 0.6 healthy approximately seven seconds after recreation;
- web 0.6 running approximately 37 seconds after recreation when inspected.

The retained PostgreSQL and Meilisearch ages prove that the data services were
not recreated. API and web were the changed long-running application services.
No `docker compose down`, `down -v`, or volume deletion occurred.

## Health and storage verification

```sh
docker compose -p ause-discovery --env-file .env ps

curl -fsS http://127.0.0.1:8088/ause-discovery/health/live
curl -fsS http://127.0.0.1:8088/ause-discovery/health/ready

docker compose -p ause-discovery --env-file .env \
  exec -T api printenv AUSE_ARTIFACT_STORAGE_BACKEND
```

Recorded responses:

```text
{"status":"live"}
{"status":"ready"}
b2
```

The web service retained the loopback-only host binding. The explicit readiness
request proved PostgreSQL schema compatibility, the import temporary directory,
and B2 availability. Release 0.6 changed the frequent container health check to
liveness only and extended the B2 readiness success cache to ten minutes, so
routine container polling no longer spends B2 Class C requests.

## Search settings convergence

Increment 22 advanced the source-controlled search schema to version 3 and
raised the facet-distribution bound to 1,000 with count-first truncation for
People and Advisor facets. A full search rebuild was not required. API startup
patches settings on the active Meilisearch index and waits for the settings task.

No `ausectl search rebuild` command was run during this deployment. Existing
search documents and the Meilisearch container were preserved. Old version-2
search cursors become invalid through the controlled cursor-version contract.

## Data preserved and operations omitted

The deployment preserved:

- the production PostgreSQL volume and all canonical metadata;
- every PostgreSQL Project-to-B2 storage-key association;
- all B2 Project File and Project Logo objects;
- the Meilisearch container and existing search documents;
- the local `artifact-data` volume;
- administrators, audit history, repository links, and Project revisions.

The deployment did not perform:

- `docker compose down` or `down -v`;
- metadata CSV import;
- Project Content manifest dry-run or apply;
- bundle transfer to the VM;
- B2 listing, clearing, or content upload;
- administrator creation;
- full Meilisearch rebuild;
- Nginx, TLS, DNS, Compose, or environment-shape changes.

## Timing outcome

The captured API image build and push took approximately 85 seconds. Web build
time was not captured. On the VM, application image pulls took about 10 to 12
seconds and API replacement reached health in about seven seconds. Database
backup and catalog synchronization were not timed in the capture.

No 2 GB Project Content transfer or 10-to-15-minute re-import occurred. The
second routine in-place release remained well inside the desired deployment
window.

## Reusable conclusions

- A release that changes both backend behavior and frontend assets requires two
  project-owned images, API and web, not four custom images.
- PostgreSQL and Meilisearch images are pinned third-party inputs. The migrate
  service reuses the API image.
- A version-controlled catalog change requires `catalog validate` and
  `catalog sync` from the new API image.
- Catalog retirement is distinct from PostgreSQL migration and data deletion.
- Search schema version 3 converges through API startup settings patching. A
  full search rebuild is not required when Search Document fields are unchanged.
- The container health check now proves liveness only. The explicit readiness
  curl remains a required deployment check.
- Production PostgreSQL and B2 remain one preservation boundary.
- Mutable tags are convenient, but recorded index digests provide the exact
  rollback and audit identities.

## Rollback boundary

No PostgreSQL migration, Search Document field, or storage-layout change
occurred. An application rollback therefore requires only restoring the API and
web 0.5 image references, pulling if necessary, and running
`docker compose up -d --no-build --wait`. The retired catalog timestamp is
non-destructive and intentionally sticky under the current additive catalog
sync contract. Restoring the prior active `game` Platform would require an
explicit catalog decision rather than omission from an older source file.
