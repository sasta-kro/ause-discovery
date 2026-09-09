## Quick path: paste one block, then import and test

Fixed credentials for every fresh local test stack:

- Username: `Test1234567890`
- Password: `Test1234567890`

These credentials are for the local test stack only. Never use this password on a public or production system.

Paste the complete block below into a terminal at the repository root. The commands run in order. Wait for each command to finish before the next one starts. The `build` command is the slow step.

```sh
# One time only: create the local test configuration.
# Skip the next two commands when .env.local-test already exists.

cp .env.example .env.local-test

chmod 600 .env.local-test

# Validate the rendered Compose configuration.

docker compose -p ause-local-test --env-file .env.local-test config --quiet

# Build both images. This step copies the current config/ YAML into the API image.

docker compose -p ause-local-test --env-file .env.local-test build

# Start the data services and wait until both are healthy.

docker compose -p ause-local-test --env-file .env.local-test up -d --wait postgres meilisearch

# Create the database schema.

docker compose -p ause-local-test --env-file .env.local-test run --rm migrate

# Copy programs, courses, and taxonomy values from the YAML files into PostgreSQL.

docker compose -p ause-local-test --env-file .env.local-test run --rm --no-deps --entrypoint /usr/local/bin/ausectl api catalog sync

# Create the administrator. The next two lines answer the username and password prompts.

docker compose -p ause-local-test --env-file .env.local-test run --rm --no-deps --entrypoint /usr/local/bin/ausectl api admin create

Test1234567890

Test1234567890

# Start the complete stack and wait for health.

docker compose -p ause-local-test --env-file .env.local-test up -d --no-build --wait

# Confirm health. Both commands must print a status JSON.

curl -fsS http://localhost:8090/ause-discovery/health/live

curl -fsS http://localhost:8090/ause-discovery/health/ready
```

Notes on the block:

- The two `Test1234567890` lines feed the prompts of `admin create`. When the terminal does not forward the pasted password, type it by hand. The characters stay hidden while you type.
- When the administrator already exists, `admin create` prints an error. The rest of the block still runs. This is harmless.
- When you change a file inside `config/`, run the `build` command again before `catalog sync`. The image keeps a copy of the YAML files from build time. A sync without a rebuild uses the old copy.

## After the block completes

1. Open [http://localhost:8090/ause-discovery/admin/login](http://localhost:8090/ause-discovery/admin/login).
2. Sign in with `Test1234567890` / `Test1234567890`.
3. Go to Imports. Upload `tools/sp-import/output/import.csv`. This file holds 40 extracted senior projects.
4. Review the preview. Acknowledge the warnings. Select the rows. Commit.
5. Open the public home page and search. The committed projects appear in search after reconciliation.

## Manual test checklist

After signing in:

1. Create a Person.
2. Create a Project using that Person.
3. Upload a PDF Artifact.
4. Publish the Project.
5. Confirm it appears in public search.
6. Open the public Project and Person pages.
7. Test Artifact View and Download.
8. Open the audit log and confirm the mutations appear.

## Stop or delete the test stack

Stop while preserving test data:

```sh
docker compose -p ause-local-test --env-file .env.local-test stop
```

Remove containers while preserving volumes:

```sh
docker compose -p ause-local-test --env-file .env.local-test down
```

Permanently delete the isolated test data:

```sh
docker compose -p ause-local-test --env-file .env.local-test down -v --remove-orphans
```

The last command deletes only the `ause-local-test` deployment volumes.




---
## Deeper explanation of these test deploy steps
### Project architecture 

```text
Browser
  |
  v
web container, Nginx, host port 8090
  | serves React files
  | proxies /api/v1/ and /health/
  v
api container, Go application
  |             |                 |
  v             v                 v
PostgreSQL   Meilisearch     artifact-data volume
canonical    derived search  PDFs and import files
data         index
```

Five Compose services exist:

| Service | Purpose | Lifetime |
|---|---|---|
| `postgres` | Canonical records, users, sessions, audits, imports, and Artifact metadata | Long-running |
| `meilisearch` | Rebuildable search index derived from PostgreSQL | Long-running |
| `migrate` | Applies database migrations, then exits | One-off |
| `api` | Go HTTP API, background search reconciliation, and import cleanup | Long-running |
| `web` | Nginx serving the React application and proxying API requests | Long-running |

Three named volumes preserve data:

- `postgres-data`: PostgreSQL database files
- `meilisearch-data`: search index files
- `artifact-data`: uploaded Artifacts and temporary import files

PostgreSQL and Artifact storage are authoritative. Meilisearch can be rebuilt.

## Important corrections to the previous exchange

- Port `8088` being occupied was session-specific. Port `8090` is simply a separate local choice.
- The current `.env.example` already specifies port `8090` and the two `:test` image tags. Editing those three values may therefore be unnecessary.
- `.env.local-test` is not currently matched by `.gitignore`. It must not be committed, especially after adding secrets.
- `up -d --no-build` does not wait for health. A health request may fail briefly during startup. Adding `--wait` makes the command wait for healthy services.

## 1. Local configuration

```sh
cp .env.example .env.local-test
```

- `cp` copies a file.
- `.env.example` is the documented configuration template.
- `.env.local-test` becomes the configuration for this isolated deployment.
- Compose reads values from it through `--env-file .env.local-test`.

```sh
chmod 600 .env.local-test
```

Permission `600` means:

- File owner: read and write
- Group: no access
- Everyone else: no access

This matters because deployment environment files contain database passwords and session secrets.

### Important environment values

| Variable | Meaning |
|---|---|
| `AUSE_ENV` | Application mode: `development`, `test`, or `production` |
| `AUSE_LISTEN_ADDR` | Internal API listening address, normally port `8080` |
| `AUSE_PUBLIC_BASE_PATH` | URL prefix, normally `/ause-discovery/` |
| `AUSE_DATABASE_URL` | PostgreSQL connection string |
| `AUSE_MEILISEARCH_URL` | Internal Meilisearch address |
| `AUSE_MEILISEARCH_API_KEY` | API credential used by the Go service |
| `AUSE_MEILISEARCH_INDEX` | Logical search index name |
| `AUSE_ARTIFACT_ROOT` | Uploaded-file directory inside the API container |
| `AUSE_IMPORT_TEMP_ROOT` | Temporary import-file directory |
| `AUSE_MAX_ARTIFACT_BYTES` | Maximum size per file, 250 MiB |
| `AUSE_MAX_PROJECT_ARTIFACT_BYTES` | Maximum active Artifact storage per Project, 2 GiB |
| `AUSE_SESSION_IDLE_TTL` | Session expires after this much inactivity |
| `AUSE_SESSION_ABSOLUTE_TTL` | Maximum session lifetime |
| `AUSE_SESSION_SECRET` | Protects session-related security state |
| `AUSE_COOKIE_SECURE` | Restricts cookies to HTTPS when `true` |
| `AUSE_TRUSTED_PROXY_CIDRS` | Reverse-proxy networks whose forwarded headers are trusted |
| `AUSE_FEATURED_PROJECT_IDS` | Optional comma-separated featured Project UUIDs |
| `VITE_PUBLIC_BASE_PATH` | URL prefix compiled into the React build |
| `VITE_API_PATH` | API path compiled into the frontend |
| `POSTGRES_*` | Initial PostgreSQL database and credentials |
| `MEILI_MASTER_KEY` | Meilisearch server credential, matching the API key |
| `AUSE_WEB_PORT` | Host port mapped to Nginx |
| `AUSE_API_IMAGE` | Local API image name and tag |
| `AUSE_WEB_IMAGE` | Local web image name and tag |

`AUSE_PUBLIC_BASE_PATH` is runtime configuration. `VITE_PUBLIC_BASE_PATH` is compiled into the frontend. Both must agree.

The hostname `postgres` in `AUSE_DATABASE_URL` is a Compose service name. It resolves inside the Compose network, not from the host.

## 2. Compose command structure

Every command starts with:

```sh
docker compose -p ause-local-test --env-file .env.local-test
```

- `docker compose`: operates the multi-container application.
- `-p ause-local-test`: assigns an isolated Compose project name.
- `--env-file .env.local-test`: loads configuration from that file.

The project name scopes containers, networks, and volumes. Resulting resources resemble:

```text
ause-local-test-postgres-1
ause-local-test-api-1
ause-local-test_default
ause-local-test_postgres-data
ause-local-test_artifact-data
```

This isolation prevents interference with stacks using project names such as `ause-discovery` or `ause-discovery-verification`.

A trailing `\` means the shell command continues on the next line. It has no Docker-specific meaning.

## 3. Validate configuration

```sh
docker compose -p ause-local-test --env-file .env.local-test config --quiet
```

- Reads `compose.yaml`.
- substitutes environment variables.
- validates the rendered Compose structure.
- `--quiet` prints nothing on success.
- A nonzero exit status indicates invalid configuration.

This does not build or start anything.

## 4. Build images

```sh
docker compose -p ause-local-test --env-file .env.local-test build
```

This builds local images declared by Compose.

### API image

The backend Dockerfile:

1. Uses a pinned Go toolchain image.
2. Compiles `ause-api`.
3. Compiles the operator CLI, `ausectl`.
4. Copies both binaries into a small pinned Alpine runtime.
5. Copies migrations and catalog configuration.
6. Runs as the unprivileged `ause` user.

The `migrate` and `api` services use this same image.

### Web image

The frontend Dockerfile:

1. Uses pinned Node and pnpm versions.
2. installs exact lockfile dependencies.
3. builds the React/Vite application.
4. copies the result into pinned Nginx.
5. runs Nginx as the unprivileged `nginx` user on internal port `8080`.

`ause-discovery-local-api:test` and `ause-discovery-local-web:test` are local image tags. They are labels, not registry downloads.

## 5. Start infrastructure

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  up -d --wait postgres meilisearch
```

- `up`: creates and starts services.
- `-d`: runs them in the background.
- `--wait`: waits until health checks pass.
- `postgres meilisearch`: starts only those services.

PostgreSQL health uses `pg_isready`. Meilisearch health calls its `/health` endpoint.

The API and web are deliberately not started yet because database setup must happen first.

## 6. Apply migrations

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm migrate
```

- `run`: creates a temporary container for one service.
- `migrate`: selects the Compose `migrate` service.
- `--rm`: deletes that temporary container after completion.

The service starts the API binary with:

```text
ause-api migrate
```

That command connects to PostgreSQL and applies forward Goose migrations. Migrations create and update the database schema.

Running migrations repeatedly is safe when no new migration exists. Already-applied versions are not reapplied.

## 7. Inspect migration status

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api migrations status
```

Breakdown:

- `run api`: creates a temporary container using the API service configuration.
- `--entrypoint /usr/local/bin/ausectl`: runs the operator CLI instead of the normal API server.
- `migrations status`: arguments passed to `ausectl`.
- `--no-deps`: does not automatically start API dependencies.
- `--rm`: deletes the temporary container afterward.

PostgreSQL is already running, so automatic dependency startup is unnecessary.

Expected output resembles:

```text
migration version: 1
migration applied: true
```

This proves that the expected migration is registered as applied.

## 8. Validate catalogs

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api catalog validate
```

This loads the version-controlled academic and taxonomy YAML files from the API image and validates their structure and internal rules.

No database changes occur.

## 9. Synchronize catalogs

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api catalog sync
```

This writes version-controlled catalog values into PostgreSQL.

Catalogs include data such as:

- programs
- majors
- courses
- academic validity periods
- taxonomy values

Synchronization is non-destructive. Existing historical versions remain preserved.

## 10. Create an administrator

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps --entrypoint /usr/local/bin/ausectl \
  api admin create
```

The CLI prompts for:

```text
Username:
Password:
```

The password prompt requires an interactive terminal and hides input. Passwords are not accepted in flags or environment variables, which keeps them out of shell history and process listings.

For the local test stack, this document fixes the credentials in advance:

- Username: `Test1234567890`
- Password: `Test1234567890`

The quick block feeds these two values through the prompts. The username is stored in lowercase form. Login accepts the same value in any letter case because both sides pass through the same normalization.

The command stores an Argon2id password hash in PostgreSQL. It does not store the plaintext password.

## 11. Start the complete application

Original command:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  up -d --no-build
```

Recommended local form:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  up -d --no-build --wait
```

- Starts all services.
- `--no-build` requires the locally built images and prevents an accidental rebuild.
- `--wait` waits for health checks.
- The `migrate` service runs before the API.
- The API starts after migration succeeds.
- Nginx starts and exposes host port `8090`.

The API also starts two background processes:

- Search reconciliation, which updates Meilisearch from PostgreSQL changes
- Import cleanup, which removes expired temporary import state

## 12. Request routing

The browser connects only to Nginx:

```text
http://localhost:8090/ause-discovery/
```

Nginx handles paths as follows:

| Path | Destination |
|---|---|
| `/ause-discovery/assets/*` | Static compiled files |
| `/ause-discovery/api/v1/*` | Proxied to the Go API |
| `/ause-discovery/health/*` | Proxied to the Go API |
| Other `/ause-discovery/*` routes | React `index.html` for client-side routing |

PostgreSQL and Meilisearch publish no host ports. They remain reachable only inside the Compose network.

## 13. Inspect status

```sh
docker compose -p ause-local-test --env-file .env.local-test ps
```

This lists containers, states, ports, and health status.

A healthy deployment should show PostgreSQL, Meilisearch, API, and web running. The migration container should have completed successfully.

Useful logs:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  logs --tail 100 api web
```

This prints the most recent 100 log lines from the API and web services.

## 14. Health checks

```sh
curl -fsS http://localhost:8090/ause-discovery/health/live
```

Flags:

- `-f`: fail on HTTP 400 or 500 responses
- `-s`: suppress progress output
- `-S`: still print errors

Expected response:

```json
{"status":"live"}
```

Liveness means the API process is accepting HTTP requests.

```sh
curl -fsS http://localhost:8090/ause-discovery/health/ready
```

Expected response:

```json
{"status":"ready"}
```

Readiness additionally verifies:

- PostgreSQL is reachable.
- The applied schema version is supported.
- Artifact storage is writable.
- Import temporary storage is writable.

Readiness intentionally does not require Meilisearch. Search may be temporarily unavailable while PostgreSQL-backed detail pages remain usable.

## 15. Manual product test

The suggested workflow crosses the major system boundaries:

1. Create a Person, proving authenticated administration and PostgreSQL writes.
2. Create a Project linked to that Person, proving aggregate relationships.
3. Upload a PDF, proving validation and persistent Artifact storage.
4. Publish the Project, making it publicly visible.
5. Search for it, proving PostgreSQL-to-Meilisearch reconciliation.
6. Open public Project and Person pages, proving public projections.
7. View and download the Artifact, proving safe serving and range support.
8. Inspect the audit log, proving append-only mutation auditing.
9. Test CSV or XLSX import, proving persistent preview and atomic commit behavior.

## 16. Browser acceptance

```sh
AUSE_E2E_BASE_URL=http://localhost:8090/ause-discovery/ make test-e2e
```

The prefix:

```sh
AUSE_E2E_BASE_URL=...
```

sets an environment variable only for this command.

`make test-e2e` runs:

```sh
pnpm --filter @ause-discovery/frontend exec playwright test
```

The browser suite checks:

- Public shell and nested routing
- Search page title and content
- Axe accessibility results
- Narrow 360-pixel layout without horizontal overflow
- Artifact actions when fixture variables are supplied

The Artifact test requires:

```text
AUSE_ACCEPTANCE_PROJECT_ID
AUSE_ACCEPTANCE_PROJECT_TITLE
```

Without those values, that one test is skipped. This is a narrow browser acceptance suite, not the full backend or administrator test suite.

## 17. Stop while preserving everything

```sh
docker compose -p ause-local-test --env-file .env.local-test stop
```

This stops containers but keeps:

- containers
- network
- volumes
- database data
- search data
- uploaded files

Restart with:

```sh
docker compose -p ause-local-test --env-file .env.local-test start
```

## 18. Remove containers but preserve data

```sh
docker compose -p ause-local-test --env-file .env.local-test down
```

This removes:

- project containers
- project network

It preserves named volumes. A later `up` recreates containers and reconnects the existing data.

## 19. Permanently delete the test deployment

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  down -v --remove-orphans
```

- `down`: removes containers and network.
- `-v`: deletes the project’s PostgreSQL, Meilisearch, and Artifact volumes.
- `--remove-orphans`: removes same-project containers no longer declared in the current Compose file.

This permanently deletes all data belonging to `ause-local-test`. The `-p ause-local-test` scope is the main safeguard against deleting another stack.