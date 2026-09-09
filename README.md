# AUSE Discovery

AUSE Discovery is a searchable institutional archive of historical senior Projects and associated Artifacts. Public visitors search and browse Project metadata; authenticated administrators maintain canonical records, Artifacts, imports, search state, and the audit log.

## Toolchain baseline

The repository uses exact versions: Node.js `24.20.0`, pnpm `11.25.0`, Go `1.27.0`, sqlc `v1.31.1`, oapi-codegen `v2.8.0`, PostgreSQL `18.6`, Meilisearch `v1.53.1`, and Nginx `1.30.4`. Container images are pinned by digest in `compose.yaml`, `backend/Dockerfile`, and `frontend/Dockerfile`.

The current local TypeScript static-analysis gate is `tsc -b`. `typescript-eslint` `8.69.0` does not support the required TypeScript `7.0.2`, so ESLint is limited to compatible JavaScript configuration until the fixed version baseline changes. OpenAPI TypeScript generation uses the isolated `api/generator` workspace with `@hey-api/openapi-ts` `0.99.0` and its compatible TypeScript `5.9.3` tool dependency.

Use `make doctor` to inspect local tooling and available Docker fallbacks. Go commands in the Makefile run through the pinned Go container, so a local Go installation is not required.

## Quick local validation

Local parity commands for the checks in `.github/workflows/validate.yml`. `make vuln-report` covers govulncheck and the pnpm audit; the container image scans run in CI only:

```sh
make check-source       # action pins, frontend lint/tests/builds, Go format/vet/tests
make check-integration  # Go tests with PostgreSQL integration (compose PostgreSQL required)
make generate-check     # regenerates sqlc and OpenAPI output and fails on drift
make check-images       # builds the API and web images, including the root base path variant
make check-compose      # validates the rendered Compose configuration
make vuln-report        # govulncheck plus the pnpm audit
make check-action-pins-test  # fixture test for the action-pin script
make test-integration   # starts dependencies and runs PostgreSQL-backed tests
make test-e2e           # runs browser acceptance against the configured live stack
make test-all           # runs the complete local verification set
```

`make generate-check` requires a prior `make install` so the generator workspace has `node_modules`; the TypeScript generator runs through the pinned Node container.

Browser acceptance defaults to `http://localhost:8088/ause-discovery/`. Set `AUSE_E2E_BASE_URL` for another live stack. Set `AUSE_ACCEPTANCE_PROJECT_TITLE` and `AUSE_ACCEPTANCE_PROJECT_ID` to include the configured public Project and Artifact path; otherwise that fixture-dependent test is skipped.

## Local Compose bootstrap

```sh
make install   # locked dependencies plus the local API image
make seed      # start PostgreSQL, migrate, and synchronize catalogs
make compose-up
```

The web entry point publishes `http://localhost:8088/ause-discovery/`. Create the first administrator with `make seed`'s image through the command shown in the operator runbook.

## Documentation

- Implementation specification and build guide: `docs/ause-discovery_implementation_specification_v1.md`
- Product specification: `docs/ause-discovery_specification_v1.md`
- Operator runbook for deployment, operations, backup, restore, upgrades, and recovery: `docs/operations/runbook.md`
- Bulk Project-file import and demo seeding: `docs/operations/project-file-import.md`

## Ownership and publication boundaries

Production ownership and launch-time values (GitHub repository ownership, production URL, VM details, TLS, branding, approved legal content, and final institutional catalog vocabulary) remain external inputs; see the runbook for the unresolved list.

CI validates every pull request and push to the default branch. Container images are published to GHCR only for the configured tagged-release event, and CI never deploys to a VM or contacts any production host. Deployment is human-operated through the runbook using published image digests.
