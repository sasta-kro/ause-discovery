# Project Logo and unified content import increment

## Assignment

Implement and commit Project Logo storage, unified Project Content import, public delivery, and frontend presentation.

This is one bounded vertical increment. A Project Logo is presentation metadata for one Project. Its bytes use the configured object store, but it is not an Artifact and must never appear as a downloadable Project File, Artifact count, Artifact type, Artifact facet, or Project File administration row. Project Files and the optional Project Logo are described together by one Project Content manifest and dispatched to their separate domain services during import.

Start only after increment 11 is accepted. Treat that accepted commit as the baseline. Preserve unrelated working-tree changes and all existing behavior unless this brief explicitly changes it.

## Required reading before coding

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`
4. `.memory/architectural_decision_logs.md`, especially AD-011
5. `.memory/mvp_implementation_status.md`, especially the completed enrichment pass
6. `.memory/docs/increments/10-pluggable-artifact-storage.md`
7. `.memory/docs/increments/11-project-file-import-concurrency.md`
8. `api/openapi.yaml`
9. `backend/internal/artifacts/`, `backend/internal/artifactimport/`, `backend/internal/search/`, and nearby focused tests
10. `frontend/src/app-shell.tsx`, `frontend/src/App.module.css`, and focused Project/search tests
11. `tools/ausesp-data-extractor/README.md` enrichment section and `tools/ausesp-data-extractor/enrichment/manifest.json`
12. This brief

The extractor repository is nested, separately versioned, and ignored by the main repository. Treat its files as operator input, not application source code.

## Repository starting state

- The extractor repository is at commit `59fced5` and contains 71 verified Project-specific PNGs in `enrichment/logos/<id>.png`.
- `enrichment/manifest.json` is a top-level object keyed by corpus identifier. A usable logo has `logo.has_logo=true`, `logo.extracted=true`, `logo.verified=true`, and a relative `logo.output` path.
- The corpus identifier maps to the metadata import key as `sp-<identifier>`. It is an internal import mapping, not a public Project identity.
- The 71 PNGs total about 14.8 MB. The largest is 1,430,125 bytes. Width and height do not exceed 1600 pixels.
- Most Projects have no extracted logo. Search results currently use title initials in a fixed red or purple square.
- Project detail pages currently have no representative image.
- The existing storage interface can persist and open opaque bytes through local or B2 providers. Artifacts store their own provider marker.
- The current `ausectl artifacts import-manifest` command consumes a flat five-column CSV containing only Project Files. This command and format are replaced by the unified Project Content importer in this increment. Backward compatibility is not required.
- The specialized `ausectl artifacts seed-demo` helper remains available for copying the four shared mock files across published Projects. It is not the long-term real-content manifest format.
- `ProjectSummary` is shared by search results, public Person Project lists, and import duplicate candidates. Public Project and Admin Project have separate response schemas.
- Search documents are derived from PostgreSQL and use Project revision plus `project_search_sync`.
- Repository-link enrichment exists in the same extractor manifest but is not part of this increment.

## Domain decision

A **Project Logo** is an optional, Project-owned representative image used to identify the Project visually. It is not submitted academic material.

Consequences:

- Project Logo metadata has its own table and service boundary.
- Logo bytes reuse the configured object-store infrastructure.
- A Project has at most one active logo.
- Logo replacement and removal are audited and revisioned.
- A logo never enters `artifacts`, Artifact quotas, Artifact search facets, Project File lists, or download controls.
- Initials remain the public fallback when no active logo exists or image loading fails.

No new ADR is required. This is a bounded domain distinction inside the existing Project aggregate and storage-provider decision.

## Required outcome

An administrator can describe each Project's optional logo and Project Files in one versioned JSON manifest and import the complete bundle through one dry-run-first CLI command. The importer resolves each Project through its stable internal `project_import_key` or Project ID, validates all content before writes, uploads with bounded Project-level concurrency, safely skips unchanged content, replaces changed logos, prints one line per completed Project, and reports a nonzero exit on failures.

Public search results and Project detail pages display a Project Logo when present and fall back to the existing initials treatment when absent or unavailable. Logo bytes are served through the application from either configured storage provider. No logo is presented as a downloadable Project File.

## Scope

### Included

- `project_logos` database migration and schema-version update.
- Project Logo domain/service package.
- Local and B2 storage through the existing configured store set.
- PNG validation, digesting, size limits, dimension limits, replacement, soft removal, and audit events.
- Public inline logo endpoint.
- Administrator upload/replace/remove endpoints and a compact control on the Project edit screen.
- Nullable `logo_url` on Project summary, public Project, and administrator Project responses.
- Search-document projection of logo availability and version.
- Search result and Project detail rendering with initials fallback.
- Versioned JSON Project Content manifest containing optional Project Logo and Project File entries.
- Dry-run-first unified Project Content importer using Project IDs or import keys.
- Removal of the superseded flat Project File CSV manifest command, loader, tests, and operator instructions.
- Bounded concurrent Project processing and one progress line per completed Project, consistent with increment 11.
- Focused backend, PostgreSQL, HTTP, search, frontend, and accessibility tests.
- Internal operator notes for the new CLI, manifest contract, and current extractor export.
- Durable memory updates and one coherent main-repository commit.

### Excluded

- Treating a logo as a new Artifact type.
- A public Download button or inclusion in Project File lists.
- Repository links, deployed-site links, or link liveness UI.
- Logo extraction, cropping, visual re-review, or changes to ground truth.
- Importing `third_party_reference` URLs.
- Preserving compatibility with `ausectl artifacts import-manifest` or its five-column CSV contract.
- Multiple active logos, galleries, project screenshots, hero banners, or image editing.
- SVG, animated images, WebP conversion, CDN work, direct public B2 URLs, or browser access to B2 credentials.
- Automatic second copies, storage failover, reverse migration, and unrelated storage review corrections.
- Full acceptance, release publication, or VM deployment.

## Data contract

Add a migration after the current schema version with a dedicated table shaped around the existing Artifact metadata pattern:

```sql
project_logos
  id uuid primary key
  project_id uuid not null references projects(id)
  storage_key text not null unique
  storage_backend text not null
  mime_type text not null
  extension text not null
  byte_count bigint not null
  sha256 bytea not null
  status text not null
  revision bigint not null default 1
  actor_id uuid references application_users(id)
  created_at timestamptz not null
  updated_at timestamptz not null
  deleted_at timestamptz null
```

Apply constraints equivalent to:

- provider value follows the same `local` or `b2` constraint as the accepted Artifact marker;
- `mime_type = 'image/png'` and `extension = 'png'`;
- positive byte count;
- 32-byte SHA-256 digest;
- status is `active` or `deleted`;
- one partial unique index on `project_id` where status is active.

Use the current database column name required by the accepted baseline. If the recorded storage-terminology refactor lands first, use `storage_provider` instead of reintroducing `storage_backend`. Do not combine that broad rename into this increment.

Old logo rows and bytes remain after replacement or removal, consistent with the current archival approach for Project Files. Only the active row is public.

## Logo validation and mutation behavior

- Accept PNG only for this increment.
- Maximum upload size is 2 MiB. This covers the verified corpus maximum with margin while preventing Artifact-sized image uploads.
- Decode PNG configuration before persistence and reject invalid or truncated images.
- Maximum width and height are 1600 pixels, matching the extractor contract.
- Empty content, declared-size mismatch, MIME mismatch, invalid dimensions, and deleted Projects return controlled validation errors.
- Upload or replacement writes bytes first, then performs the database mutation transaction. A failed transaction removes only the newly written object.
- Upload requires the current Project revision. The transaction locks the Project, verifies revision and state, inserts the new active row, soft-deletes any previous active row for replacement, advances Project revision, schedules search synchronization, and appends one audit event.
- Removal locks the same state, soft-deletes the active logo, advances Project revision, schedules search synchronization, and appends an audit event.
- Audit event names are `project_logo.uploaded`, `project_logo.replaced`, and `project_logo.removed`. Metadata may contain byte count, digest, Project revision, and replaced logo ID. It must not contain storage keys or source paths.
- An identical active digest is an idempotent skip in the bulk importer, not a replacement.
- Storage-provider missing-content and unavailable errors use controlled Project Logo problem codes and never expose provider details.

Place the Project Logo service in its own package, such as `backend/internal/projectlogos`. Reuse the store interface and storage set. Do not call the Artifact mutation service and do not duplicate local or B2 adapters.

## HTTP and OpenAPI contract

Add:

- `GET /projects/{project_id}/logo`
- `PUT /admin/projects/{project_id}/logo`
- `DELETE /admin/projects/{project_id}/logo`

Administrator upload is multipart and includes `expected_project_revision` plus one PNG file. Removal uses the expected Project revision. Existing authentication and CSRF rules apply.

Add nullable `logo_url` with `format: uri-reference` to:

- `ProjectSummary`;
- `PublicProject`;
- `AdminProject`.

An active logo URL uses the configured public base path and includes the logo revision as a cache-busting query value:

```text
<public-base-path>/api/v1/projects/<project-id>/logo?v=<logo-revision>
```

The public logo endpoint:

- returns 404 when the Project is unpublished, deleted, or has no active logo;
- streams only validated PNG content from the row-selected storage provider;
- verifies stored size against metadata;
- sets `Content-Type: image/png`;
- sets an inline content disposition, never attachment;
- supports conditional or byte-range behavior through the existing seekable store contract where practical;
- uses a long immutable cache only because the response URL contains the logo revision;
- maps provider unavailability to a controlled 503;
- never exposes a storage key or direct B2 URL.

Regenerate Go and TypeScript API clients from `api/openapi.yaml`. Do not hand-edit generated files.

## Search projection

Add nullable logo revision data to `search.Document`. The search index does not need raw logo bytes, digest, storage key, or provider.

`BuildProjectDocument` reads the active Project Logo revision. `searchResponse` turns it into `logo_url` using the configured public base path. The public Person Project mapper and direct Project response query the same active-logo summary helper.

Logo upload, replacement, and removal advance the Project revision and update `project_search_sync`, so search results converge through the existing reconciler. A full search rebuild must also project the logo correctly.

Logo state must not alter `artifact_count`, `artifact_types`, `has_artifacts`, or any Artifact availability flag.

## Unified Project Content import contract

Add the preferred command:

```text
ausectl project-content import-manifest --manifest <path> --actor-username <username> [--workers 1..8] [--apply]
```

The command is dry-run by default. Use the same default and maximum worker bounds as increment 11. Remove `ausectl artifacts import-manifest` and the old five-column CSV loader. No compatibility alias or legacy format branch is required. Retain `ausectl artifacts seed-demo` and `ausectl artifacts migrate` because they perform separate demonstration and storage-migration jobs.

The application-owned manifest is strict, versioned JSON. Its shape is:

```json
{
  "version": 1,
  "projects": [
    {
      "project_import_key": "sp-1703",
      "logo": {
        "file_path": "logos/1703.png"
      },
      "files": [
        {
          "artifact_type": "report",
          "display_name": "Final report",
          "original_filename": "final-report.pdf",
          "file_path": "files/1703/final-report.pdf"
        },
        {
          "artifact_type": "slides",
          "display_name": "Presentation slides",
          "file_path": "files/1703/presentation-slides.pdf"
        }
      ]
    }
  ]
}
```

Manifest rules:

- `version` is required and must equal `1`.
- `projects` is required, nonempty, and contains each target Project at most once.
- Each Project entry sets exactly one of `project_id` or `project_import_key`.
- Each Project entry contains a nonempty `files` array, a `logo`, or both.
- `logo` is optional and contains exactly one `file_path`.
- `files` defaults to an empty array. Each file requires `artifact_type`, `display_name`, and `file_path`; `original_filename` is optional and defaults to the source basename.
- Every `file_path` is relative to the manifest directory and uses the existing symlink and path-escape protection.
- Unknown fields, unsupported versions, duplicate Projects, blank strings, and trailing JSON data are rejected.
- Project and file order is preserved for deterministic planning and sequential work within each Project.

The operational bundle layout is:

```text
project-content/
  manifest.json
  files/
    1703/
      final-report.pdf
      presentation-slides.pdf
  logos/
    1703.png
```

One manifest is the operational source of mapping. The importer dispatches `files` entries through the Artifact service and `logo` through the Project Logo service. This preserves the domain distinction without forcing operators to maintain a second manifest containing only a Project key and image path.

For the current extractor output, operator preparation maps each usable `enrichment/manifest.json` entry to a Project entry as:

```text
project_import_key = "sp-" + top-level corpus identifier
logo.file_path     = logo.output
files              = existing Project File entries when available, otherwise empty
```

Only entries with all of these conditions belong in the generated Project Content manifest:

- `logo.has_logo` is true;
- `logo.extracted` is true;
- `logo.verified` is true;
- `logo.output` is a nonempty relative path.

Document a short `jq` export command in the internal operator note so a valid logo-only Project Content manifest can be generated from the current 71 usable extractor entries without editing Projects manually. The same manifest can later add real Project File entries under each Project. Runtime application code consumes the Project Content JSON contract and does not depend on the extractor's larger provenance and link schema.

Dry-run resolves every Project and validates every logo and Project File before writes. It reports Project count, planned logo uploads, planned logo replacements, planned Project File uploads, unchanged skips, total bytes, and errors. Invalid content or any planning error prevents the entire apply phase.

Apply processes Projects concurrently. Within one Project, process the optional logo and Project Files sequentially while reading the current Project revision before every actual mutation. A failure stops the remaining content for that Project; safe reruns skip already-committed identical content and resume the remainder. No cross-Project rollback is attempted.

Reuse the accepted Project completion output behavior from increment 11. If that implementation is scoped inside `artifactimport`, extract only the smallest scheduling and serialization helper justified by this second caller. Do not create a general job framework.

Each progress line summarizes the complete Project outcome on one physical line, including `logo uploaded`, `logo unchanged`, Project File types uploaded or skipped, and any failed item. An apply rerun must skip all unchanged logos and Project Files. If a source PNG changes, only that Project Logo is replaced.

Implement the new boundary in a package such as `backend/internal/projectcontentimport`. It coordinates Project-level planning and calls the Project Logo and Artifact services without merging their records or validation rules. Remove the superseded CSV manifest loader from `backend/internal/artifactimport`; retain only code still required by `seed-demo`, moving shared Project scheduling code when that produces the clearest dependency direction.

## Frontend behavior

### Search results

Keep the current fixed square identity area and color treatment as the visual fallback.

- When `logo_url` is present, render the logo inside the same footprint with `object-fit: contain` and a neutral surface.
- Keep dimensions fixed to prevent layout shift.
- If loading fails, replace the image with the existing title initials in the same position.
- Mark the image decorative with `alt=""` because the linked Project title immediately supplies the accessible name.
- Preserve current narrow-layout dimensions and overall application aesthetic.

### Project detail

Place the logo beside the Project title in the existing page header, using a bounded square or near-square container and `object-fit: contain`. On narrow screens it may stack above the title without horizontal overflow. Use the same initials fallback and decorative-image rule.

Do not add the logo to the Project Files section.

### Administrator Project page

Show the current logo preview or initials fallback near a compact `Upload or replace Project Logo` control and a `Remove Project Logo` action when present. Preserve form state and refresh the Project revision after a successful mutation. Error, pending, revision-conflict, and invalid-file states must be visible.

Do not add Project Logo or Project File fields to CSV/XLSX metadata import. Project metadata remains a separate import stage; the Project Content manifest runs after target Projects exist.

## Suggested implementation order

1. Add the migration and supported schema version.
2. Add Project Logo model, validation, storage, mutation, open, and active-summary behavior.
3. Add PostgreSQL and storage-adapter focused tests.
4. Add OpenAPI endpoints and nullable response field, then regenerate clients.
5. Add HTTP handlers, error mapping, base-path-safe URLs, and cache behavior.
6. Add search-document projection and synchronization tests.
7. Add the strict Project Content JSON loader, whole-bundle planner, importer, command parser, progress output, and internal operator note.
8. Remove the superseded `artifacts import-manifest` command, CSV loader, tests, and instructions.
9. Add search result, Project detail, and administrator Project UI.
10. Run focused verification once.
11. Update `.memory/CONTEXT.md`, `.memory/mvp_implementation_status.md`, and any verified reusable lesson.
12. Commit the main repository once and stop.

## Focused verification

### Backend and CLI

- PNG validation accepts representative extractor files and rejects wrong MIME, malformed PNG, oversize content, and over-dimension images.
- Upload creates one active row, stamps the selected provider, advances Project revision, schedules search synchronization, and audits.
- Replacement soft-deletes the prior row and keeps one active row.
- Removal hides the logo and schedules search synchronization.
- Identical Project Content rerun skips logos and Project Files without new rows or Project revision changes.
- Local and fake B2 adapters serve identical bytes.
- Missing content maps to 404; unavailable storage maps to 503.
- Generation from the extractor manifest produces exactly 71 Project entries, and every relative PNG path resolves and passes validation.
- A focused disposable PostgreSQL test covers at least two Projects containing both logos and multiple Project Files, an unchanged rerun, and one logo replacement without loading the full corpus.
- Worker bounds, duplicate Projects, unsupported manifest versions, unknown fields, malformed JSON, path escapes, missing Projects, deleted Projects, and partial failures are covered.
- The old Project File CSV command and parser no longer exist.
- One progress line is emitted per completed Project, not per file.

### API and search

- Public logo access is allowed without authentication only for published Projects.
- Draft, deleted, absent, and removed logos return 404.
- Admin mutations require authentication and CSRF.
- `logo_url` is null when absent and base-path-correct when present.
- Search indexing and rebuilding preserve logo revision.
- Logo changes do not affect Artifact counts or facets.

### Frontend

- Search result uses a logo when present and initials when absent.
- An image error restores initials without leaving a broken-image icon.
- Project detail renders the same behavior.
- Administrator upload, replace, remove, validation error, and revision conflict are covered by focused component tests.
- Decorative images have empty alternative text and fixed dimensions.
- Narrow viewport styles do not overflow.

Run focused Go, integration, generation-drift, frontend lint/type, and affected component tests. Build the frontend once for the default subpath if generated-client or asset URL handling requires it. Do not run full Playwright, full product acceptance, live B2 bulk import, image publication, or VM deployment.

One optional live boundary check may upload and display a single logo in the existing local test stack only after all automated checks pass. It must not erase or rebuild the stack. The 71-logo live import remains a maintainer-run acceptance step.

## Completion criteria

- Project Logo is a distinct Project concept backed by the shared store and one active database row.
- One versioned Project Content manifest can map logos and Project Files without a second logo-only import format.
- The 71 extracted logos can be prepared and imported without manual row-by-row UI work.
- Dry-run and idempotent rerun behavior are clear.
- Public search and Project detail use logos with reliable initials fallback.
- Administrators can replace and remove one logo without using Project File controls.
- Logos do not appear in Artifact records, counts, facets, lists, or download actions.
- Public URLs remain application-owned and work under root or configured subpath deployment.
- Focused verification passes without broad acceptance or live bulk B2 work.
- Existing untracked `.env.local-test` and `.DS_Store` files remain unstaged.
- One coherent main-repository commit uses a lowercase past-tense message with no prefix and no co-author trailer.

## Required handoff

Stop after the implementation commit. Report:

- accepted increment 11 baseline and final commit hash;
- migration and schema version;
- Project Logo table, service, mutation, and storage design;
- exact manifest contract and 71-entry generation evidence;
- confirmation that the old flat Project File CSV importer was removed;
- public endpoint and cache behavior;
- search synchronization behavior;
- UI locations and initials fallback behavior;
- focused tests and exact results;
- any failed test, skipped check, or residual risk;
- confirmation that repository-link integration, live B2 bulk import, image publication, and VM deployment did not occur;
- final `git status --short`.
