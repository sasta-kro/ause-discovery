# AUSE Discovery project context

## Product

AUSE Discovery is a public searchable institutional archive of historical senior projects. Public visitors search and browse Project metadata, people, academic context, controlled classifications, and available Artifacts. Authenticated administrators maintain canonical records and search state.

## Canonical boundaries

- PostgreSQL is canonical.
- Meilisearch is disposable derived state.
- Artifact bytes use controlled local persistent storage outside PostgreSQL.
- Project is the search-result unit.
- Project ID is application-owned UUID identity.
- Reference Code is optional external data without assumed uniqueness or encoded semantics.
- Person records remain separate from Application Users.
- Student ID is public, searchable, and exactly seven digits.
- Delete is reversible soft deletion. Permanent destruction is outside MVP scope.

## Repository state

As of 2026-09-01:

- Git is initialized on `main` with an initial commit;
- `frontend/` contains a default npm-based Vite React scaffold that does not yet match the accepted pnpm and exact-pin baseline;
- `backend/` is empty;
- root workspace manifests, migrations, backend source, application tests, Compose, and CI do not exist;
- the implementation specification is `docs/ause-discovery_implementation_specification_v1.md`;
- the domain glossary is `CONTEXT.md`.

## Accepted implementation baseline

- Go modular monolith.
- React and Vite SPA.
- PostgreSQL, Meilisearch, Nginx, and Docker Compose.
- pnpm workspace plus root Makefile.
- OpenAPI 3.1 with generated Go transport types and TypeScript client/types.
- pgx, sqlc, and goose for PostgreSQL access and migrations.
- TanStack Query, React Hook Form, Zod, CSS Modules, i18next, and semantic design tokens.
- GitHub Actions and GHCR for validation and image publication.
- Default base path `/ause-discovery/`, with root deployment also tested.
- Version-controlled YAML catalogs and taxonomy.
- Persistent metadata-only CSV/XLSX import preview with atomic selected-row commit.
- Automatic in-process search synchronization retry plus administrator UI and CLI controls.
- Stored-file Artifacts only, limited to 250 MiB per file and 2 GiB active bytes per Project.

## Explicit implementation exclusions

Recommendations, analytics, semantic search, report-body indexing, AI extraction, public accounts, taxonomy CMS, custom document viewers, Redis, message brokers, microservices, external-link Artifacts, automatic backup, and automated VM deployment remain outside MVP scope.

## External prerequisites

GitHub ownership, initial approved academic catalog vocabulary, institutional taxonomy review, VM capacity and architecture, production URL, TLS ownership, approved branding, legal text, contact information, and final public Student ID privacy approval require external confirmation.
