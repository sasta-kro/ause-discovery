# AUSE Discovery

![Status](https://img.shields.io/badge/status-demo_deployment-orange)
![React](https://img.shields.io/badge/React_19-61DAFB?logo=react&logoColor=black)
![TypeScript](https://img.shields.io/badge/TypeScript_7-3178C6?logo=typescript&logoColor=white)
![Go](https://img.shields.io/badge/Go_1.27-00ADD8?logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL_18-4169E1?logo=postgresql&logoColor=white)
![Meilisearch](https://img.shields.io/badge/Meilisearch_1.53-005EE0?logo=meilisearch&logoColor=white)
![Docker](https://img.shields.io/badge/Docker_Compose-2496ED?logo=docker&logoColor=white)

AUSE Discovery is a searchable institutional archive of historical senior Projects from Assumption University. Public visitors search and browse Project metadata, people, academic context, and controlled classifications, and download associated Artifacts such as reports, slides, posters, and source archives. Authenticated administrators maintain the canonical records, curate files, run metadata imports, keep the search index healthy, and review a complete audit trail.

## The short version

The application is a modular monolith with a strict data contract. PostgreSQL is the single source of truth; Meilisearch holds derived, fully rebuildable search projections; Artifact bytes live in controlled local storage outside the database. A React single-page application serves the public and administrator experience through an Nginx front container, which also proxies the versioned Go API. Everything is defined by an OpenAPI 3.1 contract with generated Go and TypeScript clients, so the wire between frontend and backend cannot drift silently.

The stack deploys under a configurable base path (`/ause-discovery/` by default, root also supported), which lets it live beside other services on a shared host. A demo deployment is currently serving an initial corpus of senior Projects while institutional content review and remaining interface polish continue.

The stack in one glance:

| Layer | Technology |
| --- | --- |
| Frontend | React 19, TypeScript, Vite, TanStack Query, CSS Modules |
| API | Go 1.27, Chi, pgx, sqlc, Goose migrations |
| Contract | OpenAPI 3.1 with generated Go and TypeScript clients |
| Data | PostgreSQL 18 (canonical), Meilisearch 1.53 (derived search) |
| Runtime | Docker Compose, Nginx front, non-root containers, digest-pinned images |

## Getting started

Prerequisites: Docker with the Compose plugin, and a recent Node.js for the frontend toolchain (a local Go installation is optional; all Go commands run through the pinned container). Use `make doctor` to check what is available.

```sh
git clone https://github.com/sasta-kro/ause-discover.git
cd ause-discover
make install     # install locked dependencies and build the local API image
make seed        # start PostgreSQL, apply migrations, synchronize catalogs
make compose-up  # build and start the full stack
```

The site is then served at `http://localhost:8088/ause-discovery/`. The first administrator account is created through the operator CLI prompt described in the runbook; there is no web signup, and passwords never pass through flags or environment variables.

## Development

The root Makefile is the stable interface. The frequently used targets:

```sh
make dev               # dependencies in Docker plus the Vite dev server
make check-source      # lint, component tests, builds, Go format/vet/tests
make check-integration # PostgreSQL-backed tests against compose services
make generate-check    # regenerate sqlc and OpenAPI output, fail on drift
make lint              # frontend lint gate
make test              # fast unit and component tests
make test-e2e          # browser acceptance against a live stack
```

Configuration is environment-driven. Copy `.env.example` for a local environment file; production values and their constraints are documented there and in the runbook. Catalog and taxonomy vocabularies are version-controlled YAML under `config/` and synchronized into the database with `ausectl catalog sync`, so reference data changes travel through code review like everything else.

## Repository layout

```text
api/         OpenAPI contract and generator workspace
backend/     Go API, CLI (ausectl), migrations, queries
config/      Academic catalog and taxonomy vocabularies
deploy/      Nginx configuration templates
docs/        Specifications, operator runbook, development notes
frontend/    React application and Playwright acceptance tests
scripts/     Repository tooling such as action-pin validation
```

## Testing and quality

Validation runs on every pull request and default-branch push through GitHub Actions: locked dependency install, frontend lint, component tests and both base-path builds, Go format, vet and PostgreSQL-backed tests, generated-code drift checks, API and web image builds, and dependency and container vulnerability reporting. The same checks run locally through the `make check-*` and `make test-*` targets above, so a change that passes locally passes CI.

## Deployment

Deployment is human-operated through the runbook: pull digest-pinned images, run forward migrations, synchronize catalogs, create the administrator, and start the stack behind a reverse proxy. Container images are published to GHCR only by the tagged release workflow; CI never deploys. The full procedure, including backup targets, restore ordering, upgrades, and recovery scenarios, lives in the operator runbook.

## Documentation

- Product specification: `docs/ause-discovery_specification_v1.md`
- Implementation specification and build guide: `docs/ause-discovery_implementation_specification_v1.md`
- Operator runbook: `docs/operations/runbook.md`
- Domain glossary: `CONTEXT.md`
