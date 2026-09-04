# CI and operator readiness increment

## Assignment

Implement and commit the CI and operator-readiness increment for AUSE Discovery.

This is one bounded infrastructure and documentation increment. Add reliable GitHub validation, tagged application-image publication logic, local CI parity commands, production image configuration, and an operator runbook. Stop after a coherent commit and handoff.

Do not implement product UI polish, Playwright acceptance paths, broad accessibility work, unrelated application behavior, automated VM deployment, or backup automation. Do not push a Git tag, trigger a release, publish an image, deploy a stack, or contact an external production system.

The accepted baseline is commit `2fa316b` (`Corrected audit actor normalization, retry, and memory routing`). Preserve all accepted application behavior.

## Required reading before coding

Read these files in order:

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/project_context.md`
4. `.memory/architectural_decision_logs.md`
5. `.memory/mvp_implementation_status.md`
6. `.memory/lessons_learned.md`
7. `docs/ause-discovery_specification_v1.md`, especially sections 24 and 37 through 40
8. `docs/ause-discovery_implementation_specification_v1.md`, especially sections 3, 5, 11, 13, 14, 15, 16.7, and 17
9. `README.md`
10. `Makefile`
11. `compose.yaml`
12. `.env.example`, `.gitignore`, and `.dockerignore`
13. `backend/Dockerfile` and `frontend/Dockerfile`
14. `backend/cmd/ausectl/main.go` and its tests
15. `backend/internal/platform/config/config.go` and its tests
16. `deploy/nginx/default.conf.template` and `deploy/nginx/20-base-path.envsh`
17. `package.json`, `frontend/package.json`, `api/generator/package.json`, and `pnpm-workspace.yaml`
18. This brief

Then inspect only nearby files needed to verify commands or understand existing tests. Do not reread feature packages or alter application contracts.

## Starting state

The repository currently has:

- pinned Go, Node.js, pnpm, PostgreSQL, Meilisearch, API runtime, and Nginx baselines;
- deterministic sqlc and OpenAPI generation commands;
- frontend lint, component test, type-check, and production-build scripts;
- backend unit and PostgreSQL integration tests;
- root and configurable subpath support;
- production-like Docker Compose service ordering and persistent named volumes;
- non-root Go API execution;
- operator CLI commands for migration status, catalog validation and synchronization, administrator lifecycle, Project reindex, and search rebuild;
- health endpoints and container health checks;
- no `.github/workflows` directory;
- no operator runbook;
- no production image variables or digest-consumption instructions;
- only a minimal foundation-oriented `README.md`.

Known environmental facts:

- Go may be unavailable on the host. The pinned Go container is the reproducible fallback.
- The host Node.js version may be below the exact repository baseline.
- Containerized pnpm generation against host `node_modules` has failed previously. The direct pure-JavaScript OpenAPI generator shim is a known fallback.
- The persistent development PostgreSQL schema may predate later amendments to the single initial migration. Fresh-schema verification must use a disposable database or disposable Compose project.
- GitHub owner, final repository URL, registry ownership, production domain, VM details, TLS ownership, branding, legal approval, and backup destination remain launch-time inputs.

## Required outcome

The repository must provide:

1. A GitHub validation workflow that exercises the existing reliable quality gates with exact toolchains and integration dependencies.
2. A tagged-release workflow that can build and publish API and web images to GHCR without hardcoded ownership.
3. Local commands that correspond clearly to CI jobs and do not depend on undocumented machine state.
4. Compose configuration that can consume externally published application image references, including digest references, without changing source files.
5. An operator runbook covering bootstrap, configuration, migration, administrator operations, catalog synchronization, health, logs, search recovery, backup targets, restore ordering, upgrades, rollback constraints, and disk hygiene.
6. Focused validation of workflow syntax, action pinning, Compose interpolation, documented commands, and application builds.

## Scope

### Included

- `.github/workflows/validate.yml` or an equivalently named validation workflow.
- `.github/workflows/release-images.yml` or an equivalently named tagged-image workflow.
- Full commit-SHA pins for every GitHub Action reference, with a nearby human-readable release comment.
- Exact repository toolchain versions.
- PostgreSQL-backed Go integration tests in CI.
- Existing frontend lint, component tests, TypeScript checking, and root plus default-subpath production builds.
- Generation-drift checks for sqlc, Go OpenAPI, and TypeScript OpenAPI outputs.
- API and web Docker image builds without publication in ordinary validation.
- Dependency and container vulnerability reporting with explicit, reviewable failure behavior.
- Dynamic GHCR image names derived from GitHub runtime context.
- Release image tags containing release version and commit SHA.
- Digest-oriented operator deployment instructions.
- Compose image overrides or variables for API and web images.
- Production configuration guidance based on `.env.example`.
- `docs/operations/runbook.md` or an equivalently clear operations path.
- README links to CI parity commands and the runbook.
- Small scripts or Make targets when they create clear local-CI parity or enforce action pinning.
- Focused verification and durable status updates.
- One coherent past-tense commit.

### Excluded

- Running or triggering GitHub Actions remotely.
- Pushing Git tags or GitHub releases.
- Logging into GHCR or any other registry during implementation.
- Publishing container images.
- Automated SSH, VM deployment, infrastructure provisioning, DNS, TLS issuance, or reverse-proxy integration outside the repository stack.
- Hardcoded GitHub owner, repository name, domain, registry namespace, VM path, or institutional secrets.
- Automated backup scheduling, retention, encryption, or offsite upload.
- Destructive restore execution against persistent development data.
- Playwright browser acceptance implementation. No Playwright test files currently exist, so CI must not contain a permanently skipped or permanently failing browser job.
- Broad UI, localization, responsive, accessibility, or Nginx security-header changes. Those belong to the next product-coherence increment.
- Dependency upgrades unrelated to making the workflows function.
- Reformatting existing compact application files.

## CI design requirements

### General workflow rules

- Use GitHub Actions YAML under `.github/workflows/`.
- Trigger validation for pull requests and pushes to the default branch. Do not assume the branch is named `main` inside workflow logic when the event context already provides the ref.
- Add concurrency cancellation for superseded validation runs on the same ref.
- Set least-privilege permissions. Validation requires only `contents: read` unless a reporting feature has a documented additional requirement.
- Pin every `uses:` reference to a full 40-character commit SHA. Add a nearby comment naming the intended release, such as `# v4.2.2`.
- Resolve action SHAs from the official upstream action repositories during implementation. Do not invent or shorten SHAs.
- Prefer official GitHub actions and official Docker actions. Limit third-party actions to a justified vulnerability scanner or workflow validator.
- Pin tool versions in commands. Do not use `latest`, `stable`, `node-version: lts/*`, an unversioned Go installer, or floating container tags.
- Avoid repository-specific secrets in validation.
- Avoid duplicated large shell blocks. Use Make targets or small scripts when local and CI execution should remain identical.
- Ensure each job has a descriptive name and a reasonable timeout.
- Do not add placeholder jobs that always skip.

### Validation workflow

Organize the workflow into a small number of understandable jobs. Exact job decomposition is an implementation choice, but the following boundaries must be visible.

#### Source and frontend validation

Use exact Node.js `24.20.0` and pnpm `11.25.0` from the repository baseline.

Required checks:

- frozen-lockfile dependency installation;
- frontend lint command;
- all frontend component tests;
- TypeScript checking, directly or through the existing lint/build commands;
- production frontend build for `/ause-discovery/`;
- production frontend build for `/`;
- repository-specific action-pin validation when such a script is added.

Do not add a repository-wide Prettier gate unless the current repository is first proven clean without rewriting files. Existing compact shared files must not be reformatted as part of this increment. Go formatting can be checked with `gofmt -l`.

#### Backend and integration validation

Use exact Go `1.27.0`.

Provide a PostgreSQL `18.6-alpine` service pinned to the accepted image digest. Configure a healthy service and a test role capable of creating disposable databases because integration tests create isolated databases. Supply `AUSE_TEST_DATABASE_URL` so integration tests cannot silently skip.

Required checks:

- `go mod verify`;
- `gofmt` check for tracked Go files;
- `go vet ./...`;
- `go test ./... -count=1` with the PostgreSQL test URL present;
- a clear failure when the integration database is unavailable.

Start Meilisearch only if a real current test or smoke command consumes it. Do not add an unused service merely to make the workflow look complete.

#### Generated-code drift

Regenerate and fail on a diff for:

- `backend/generated` from sqlc;
- `backend/generated/api/openapi.gen.go` from OpenAPI;
- `frontend/src/api/generated` from OpenAPI.

Use the exact sqlc, oapi-codegen, and OpenAPI TypeScript generator versions already pinned by the repository. Keep source files authoritative and never patch generated files directly.

The existing `make generate-check` may be corrected or split if its containerized TypeScript generation path is unreliable. The final command must work in a clean CI checkout and have a documented local fallback. Avoid installing a second untracked version of a generator.

#### Container builds

Build both production Dockerfiles during ordinary validation with `push: false`.

- Build the API image.
- Build the web image with default base path `/ause-discovery/`.
- Build or otherwise verify the web root-path variant `/` without publishing it.
- Preserve the exact pinned base images.
- Use BuildKit caching only through a least-privilege GitHub cache mechanism if the selected pinned actions support it cleanly.

#### Vulnerability reporting

Provide visible dependency and container vulnerability results without hiding failures behind unconditional success.

At minimum cover:

- Go modules;
- pnpm dependencies;
- the built API image;
- the built web image.

Exact tools are implementation choices. Suitable options include an exact `govulncheck` version for Go, `pnpm audit` for the lockfile, and a pinned Trivy release/action for images. Any non-blocking report must use explicit `continue-on-error` at the narrow reporting step and explain why it is advisory. A confirmed exploitable high-severity issue in shipped runtime code must not be silently ignored.

Do not add broad automated dependency-update tooling in this increment.

### Tagged image publication workflow

Implement publication logic without executing it.

- Trigger only from version tags matching the repository release convention, such as `v*`, and optionally an explicit manual workflow input that defaults to no push. Avoid publication on ordinary branch pushes or pull requests.
- Grant `packages: write` only to the publication job.
- Authenticate with the workflow-provided `GITHUB_TOKEN`. Do not define a custom registry password secret.
- Derive GHCR names from `${{ github.repository }}` or equivalent runtime context. Do not hardcode an owner or repository.
- Publish separate API and web images.
- Include an immutable commit-SHA tag and a release-version tag.
- Do not use `latest` as a deployment contract. Omitting `latest` entirely is preferred.
- Build the web image with the documented default `/ause-discovery/` base path.
- Emit image digests in the workflow summary or outputs so the operator runbook can consume them.
- Enable OCI labels. SBOM and provenance output are encouraged when supported without requiring unapproved external credentials.
- Keep automated VM deployment outside the workflow.

## Compose and configuration requirements

Update `compose.yaml` and `.env.example` only as needed for image consumption and clear production configuration.

- Parameterize API and web image references, retaining local defaults such as `ause-discovery-api:dev` and `ause-discovery-web:dev`.
- Support setting each application image variable to a complete registry digest reference.
- Preserve local `build:` definitions and development behavior.
- Document production startup with `docker compose pull` and `docker compose up -d --no-build` so configured remote images are consumed rather than rebuilt.
- Parameterize `MEILI_ENV` if production operation requires `production` while preserving the local default.
- Keep PostgreSQL and Meilisearch without published host ports.
- Preserve persistent PostgreSQL, Meilisearch, and Artifact/import volumes.
- Do not commit real production secrets.
- Clarify production-required values in `.env.example`, including database credentials, Meilisearch key, session setting, secure cookies, public base path, image references, trusted proxy ranges, and production environment mode.
- Keep launch-time values visibly unresolved rather than inventing placeholders that appear authoritative.

Log rotation may be added through bounded Compose logging options when it does not disrupt local development. If added, document defaults and override behavior. Do not run global Docker prune commands.

## Local CI parity

Improve `Makefile` or add small scripts so the important workflow checks can be run locally.

Desired boundaries:

- a fast source check for frontend lint/tests, Go format/vet/unit behavior, and action pin validation;
- a generated-code drift check;
- an integration check with explicit PostgreSQL availability;
- application image builds;
- Compose configuration validation.

Names are implementation choices. Existing public targets should remain compatible unless they are demonstrably broken. Do not make `make test` unexpectedly perform destructive cleanup or start long-lived services.

If a shell script validates action pins, it must:

- inspect tracked workflow YAML files;
- reject non-SHA `uses:` values, including tag and branch references;
- allow local actions such as `uses: ./path` if any are introduced;
- produce clear file and line diagnostics;
- return a nonzero status on failure;
- have focused fixture-based or temporary-file verification without rewriting repository workflows.

Avoid adding a general-purpose scripting framework for these checks.

## Operator runbook

Create `docs/operations/runbook.md`. Use objective, command-oriented language and distinguish tested commands from launch-time placeholders.

The runbook must cover the following sections.

### 1. Scope and invariants

- PostgreSQL is canonical.
- Meilisearch is derived and rebuildable.
- Artifact bytes and import temporary files share the application persistent storage volume in the current Compose topology.
- PostgreSQL data and Artifact bytes are critical backup targets.
- Meilisearch data does not need authoritative backup.
- Automatic backups and automated disaster recovery are outside MVP scope.

### 2. Host prerequisites

- Supported Docker Engine and Docker Compose plugin expectations without inventing an exact university-managed version.
- Required disk capacity planning for images, PostgreSQL, Artifacts, imports, logs, and backup staging.
- Required network exposure: only the web entry point is host-published by this stack.
- Required ownership or permissions for environment and backup files.

### 3. Configuration

- Copy `.env.example` to an untracked environment file.
- Set production mode, secure cookies, strong unique credentials and keys, public base path, trusted proxy CIDRs, catalog path assumptions, image digest references, and external port as applicable.
- Keep the environment file outside source control with restrictive permissions.
- Explain that the Vite public base path is compiled into the web image. A non-default production base path requires a matching web image build.
- Identify launch-time values that remain institution-owned.

### 4. Initial bootstrap

Document exact order:

1. Validate rendered Compose configuration.
2. Pull digest-pinned application images or build locally for a non-production rehearsal.
3. Start PostgreSQL and Meilisearch.
4. Run forward migrations.
5. Check migration status.
6. Synchronize academic and taxonomy catalogs explicitly.
7. Create the first administrator through the TTY password prompt.
8. Start API and web services.
9. Verify liveness, readiness, public shell, and administrator login.

Use the real `ausectl` entrypoint and Compose service names from the repository. Passwords must never appear in flags, environment variables, shell history examples, or committed files.

### 5. Routine administrator operations

- Create an administrator.
- Reset an administrator password.
- Disable an administrator.
- Validate and synchronize catalogs.
- Check migration status.
- Queue Project reindex.
- Queue full search rebuild.
- Inspect service status and bounded recent logs.

### 6. Health and observability

- Liveness and readiness URLs under the configured base path.
- Meaning of readiness failure when PostgreSQL is unavailable.
- Expected behavior when Meilisearch is unavailable.
- Structured production logs and request IDs.
- Commands for service status and bounded logs without dumping secrets.

### 7. Backup targets and maintenance-window procedure

Document, but do not automate:

- PostgreSQL logical dump as the canonical metadata backup.
- Artifact-volume backup as the canonical byte backup.
- selected non-secret configuration and deployed image digests in a small manifest;
- a maintenance window or write pause for a coordinated PostgreSQL and Artifact snapshot;
- checksum verification and a restore rehearsal requirement;
- external retention, encryption, access control, and off-host storage as institution-owned policy.

Do not present Meilisearch backup as required. Do not claim that an untested archive is a valid backup.

### 8. Restore ordering

Document a destructive-operation warning and require an explicit target check before restoration.

Restore order:

1. Stop API writes.
2. Restore PostgreSQL into the intended empty or approved target.
3. Restore Artifact bytes with expected ownership and permissions.
4. Apply any newer forward migrations.
5. Synchronize catalogs.
6. Start the API.
7. Rebuild Meilisearch from PostgreSQL.
8. verify representative Projects and Artifacts;
9. start or expose the web service fully.

Do not execute this procedure during the increment.

### 9. Upgrade and rollback

- Record current API and web image digests before change.
- Take and verify required backups before schema-changing upgrades.
- Pull replacement digests.
- Run migrations before the new API becomes ready.
- Synchronize catalogs explicitly.
- Verify health and representative behavior.
- Explain that database migrations are forward-only. Application-image rollback is safe only when the previous application remains compatible with the migrated schema.
- Prohibit blind database rollback.

### 10. Recovery scenarios

- Meilisearch unavailable or lost: preserve PostgreSQL, restart Meilisearch, queue full rebuild, observe completion.
- PostgreSQL unavailable: keep API unavailable, diagnose storage or service health, avoid writing elsewhere.
- Missing Artifact bytes: metadata remains canonical but the affected download fails safely; restore bytes from backup and verify digest expectations.
- Disk pressure: inspect named volumes, logs, stopped containers, and old application images; remove only confirmed disposable resources and never remove persistent volumes casually.
- Interrupted import: allow expiry cleanup to remove temporary state; do not delete committed Projects.

### 11. Shutdown and data preservation

- Normal service shutdown.
- Difference between `docker compose down` and volume-deleting commands.
- Explicit warning that `docker compose down -v` destroys named persistent data and is not a normal operational command.

### 12. Manual deployment boundary

- Human-operated deployment only.
- Published image digest inputs.
- TLS and external reverse-proxy ownership remain environment-specific.
- No claim that repository CI deploys automatically.

All examples must avoid real secrets and hardcoded ownership. Potentially destructive commands must carry nearby warnings and explicit target placeholders.

## README requirements

Expand `README.md` only enough to provide a useful entry point:

- project purpose;
- exact toolchain baseline;
- quick local validation commands;
- local Compose bootstrap summary;
- links to the implementation specification and operator runbook;
- statement that production ownership and launch-time values remain external;
- statement that CI publishes images only for the configured tagged-release event and never deploys a VM.

Keep detailed operations in the runbook rather than duplicating it in README.

## Focused verification

Follow `AGENTS.md`. This is an infrastructure boundary, so verify the new automation paths without rerunning final product acceptance.

### Required checks

1. `git diff --check`.
2. Workflow YAML parsing with a pinned workflow-aware validator such as `actionlint`.
3. Repository action-pin check proving all non-local `uses:` references are full commit SHAs.
4. `docker compose config` with local defaults.
5. `docker compose config` with production-like dummy values and digest-shaped API and web image references. Do not contact a registry.
6. Local CI parity fast checks.
7. Generated-code drift check.
8. PostgreSQL-backed Go test job equivalent with `AUSE_TEST_DATABASE_URL` set so integration tests run.
9. Frontend component tests, lint/type-check, and both production base-path builds.
10. API and web Docker image builds with `push: false` or ordinary local Docker builds.
11. Vulnerability commands at least far enough to prove syntax and report behavior. Network-dependent advisory database failures must be distinguished from confirmed findings.
12. Shell syntax checks for new scripts.
13. Runbook command review against current CLI usage and Compose service names.

Do not run the tagged publication path. Do not use registry credentials. Do not run Playwright or full browser acceptance because no browser test suite exists yet.

### Workflow behavior review

Inspect the rendered workflows and confirm:

- validation has no write permission unless justified narrowly;
- release publication is impossible on pull requests and ordinary branch pushes;
- only the release job has package-write permission;
- image names contain no committed owner placeholder;
- published tags include version and SHA;
- `latest` is absent;
- no deployment or SSH step exists;
- integration tests cannot silently skip because the database URL is missing;
- every external action is SHA-pinned.

### Cleanup

Remove only disposable containers, images, databases, networks, and temporary files created during this increment. Preserve active development and verification services and all persistent volumes. Do not use broad Docker prune commands.

## Completion criteria

The increment is complete only when all conditions below hold:

- Validation workflow syntax passes.
- Every external GitHub Action is pinned to a full commit SHA with a release comment.
- Validation covers locked dependency install, frontend lint/tests/builds, Go formatting/vet/tests with PostgreSQL integration enabled, generation drift, and API/web image builds.
- Vulnerability reporting covers Go, pnpm, API image, and web image with explicit gate or advisory semantics.
- Tagged release logic dynamically targets GHCR and emits version, SHA, and digest information without deploying anything.
- Compose accepts digest image references while preserving local builds and defaults.
- Operator runbook covers every required operational section and uses commands that match the repository.
- README points to the runbook and local CI parity commands.
- Focused infrastructure verification passes.
- `.memory/mvp_implementation_status.md` records only verified CI and operations behavior.
- `.memory/MEMORY.md` advances the checkpoint only after verification.
- `.memory/lessons_learned.md` changes only for genuinely durable lessons.
- No unrelated application or formatting changes are present.
- `.DS_Store` remains untracked and unstaged unless separately requested.
- One coherent commit exists with a past-tense message, for example `Added CI and operator readiness`.

## Required handoff for review

Stop after the commit. Do not begin product-coherence or final-acceptance work.

The completion report must include:

- final commit hash and exact commit message;
- concise behavior summary;
- files changed by CI, Compose/configuration, scripts/Make targets, and documentation;
- every external action with release label and pinned SHA;
- validation triggers, permissions, and publication trigger;
- local CI parity commands;
- focused verification commands and outcomes;
- vulnerability findings, advisory failures, or waivers without suppressing material risk;
- any unresolved assumption, skipped check, or environmental limitation;
- confirmation that no tag, image publication, deployment, registry login, or external production mutation occurred;
- final `git status --short` output;
- confirmation that disposable resources were removed and persistent resources were preserved.

The reviewing task will inspect the complete diff from `2fa316b` through the submitted commit, verify workflow permissions and triggers, compare documented commands with actual CLI behavior, rerun focused local checks where useful, and either accept the increment or return a bounded correction list. Do not rewrite accepted commits.
