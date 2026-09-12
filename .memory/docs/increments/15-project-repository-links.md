# Project Repository Links increment

## Assignment

Implement and commit import and public display of optional Project Repository
Links.

This is one bounded vertical increment. Repository links join Project Files
and the optional Project Logo in the existing Project Content JSON manifest,
but remain Project metadata rather than Artifacts. No separate CSV or import
command is introduced.

Start only after Increment 14 is independently accepted. Its accepted commit,
plus this planning brief, becomes the baseline. Preserve unrelated
working-tree changes and existing behavior unless this brief explicitly
changes it.

## Required reading before coding

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`, especially Project Repository Link, Artifact, and
   Project Content Import
4. `.memory/architectural_decision_logs.md`, especially AD-001, AD-003,
   AD-004, and AD-007
5. `.memory/mvp_implementation_status.md`, especially backlog item 18 and the
   enrichment-pass record
6. `.memory/docs/increments/12-project-logo-integration.md`
7. `.memory/sasta-dev-notes/project-content-import.md`
8. `backend/internal/projectcontentimport/`
9. `backend/internal/projects/service.go`
10. `backend/internal/audit/`
11. `backend/internal/search/`
12. `backend/internal/platform/httpserver/api.go`
13. `backend/migrations/` and `backend/internal/platform/database/schema.go`
14. `api/openapi.yaml`
15. `frontend/src/app-shell.tsx`, relevant CSS, i18n, and public-page tests
16. Read-only extractor evidence:
    `tools/ausesp-data-extractor/output/enrichment/manifest.json` and
    `tools/ausesp-data-extractor/output/reviewed-import.csv`
17. This brief

## Product and domain boundary

A Project Repository Link identifies a source-control repository belonging to
one Project. It is not:

- a stored Project File;
- a source-code archive;
- an Artifact storage object;
- a third-party library citation;
- a demo-site URL;
- proof that the repository remains publicly accessible.

The accepted extractor evidence currently contains 17 entries classified as
`project_repo` across 14 corpus Projects. One excluded corpus Project,
`sp-1651`, does not exist in the reviewed metadata CSV. The currently
importable evidence is therefore 15 repository links across 13 Projects:
13 last checked as public, one as not found, and one as unknown. These counts
are evidence for the focused import proof, not hardcoded application limits.

Every verified Project Repository Link remains visible regardless of its last
availability result. A private, deleted, rate-limited, or temporarily
unreachable repository can later recover or be accessible to a visitor with
separate authorization.

## Required outcome

- PostgreSQL stores an ordered set of repository links for a Project.
- The existing `project-content import-manifest` command accepts an optional
  authoritative `links` array beside `logo` and `files`.
- Whole-manifest dry-run validates every link before any write.
- Apply replaces the declared Project's repository-link set transactionally,
  advances Project revision, records audit, and remains idempotent.
- Public Project detail responses include the ordered links.
- The public Project page shows a compact Repositories section only when links
  exist, using visible URL text, availability text, and the last check date.
- Link clicks navigate directly to the external HTTPS URL in a new browsing
  context with safe relationship attributes.
- Search results, search documents, Artifact counts, Project File lists,
  storage quotas, B2, and Logo behavior do not change.
- The application performs no live outbound liveness checks.

## Scope

### Included

- Migration `00004` and supported schema version `4`.
- A narrow Project Repository Link model and service.
- Strict HTTPS URL validation, ordering, primary designation, availability,
  and checked timestamp.
- Transactional replace-set behavior with revision, audit, and pending search
  synchronization consistency.
- Optional authoritative `links` in Project Content manifest version `1`.
- Dry-run counts, apply progress, unchanged skips, replacement, and explicit
  empty-set removal.
- OpenAPI response schema and regenerated Go and TypeScript clients.
- Public Project response projection.
- Public Project detail UI consistent with the current application aesthetic.
- Focused migration, service, importer, HTTP, generation, and frontend tests.
- Import guide and implementation-status updates.
- Read-only extractor evidence proving the 15 currently importable links plan.
- One coherent implementation commit.

### Excluded

- Storing repository links as Artifacts or adding an `external_link` Artifact
  type.
- A separate Project Link CSV, metadata-import column, CLI command, or upload
  screen.
- Importing `third_party_reference` entries.
- Importing deployed-site, demo, documentation, issue-tracker, package, or
  social links.
- Repository contents, cloning, mirroring, archiving, source browsing, or ZIP
  generation.
- Runtime HTTP checks, background liveness jobs, GitHub or GitLab APIs, OAuth,
  tokens, webhooks, or scheduled refresh.
- Search indexing, filtering, faceting, keyword matching, or display of links
  in search results or Person pages.
- Administrator link editing UI. That belongs to the later administrator UX
  redesign.
- Changes to the nested extractor repository, its ground truth, classification
  decisions, or liveness script.
- Storage-provider work, B2 objects, Logos, Project File behavior, production
  deployment, or broad acceptance.

## Persistence model

Add `project_repository_links` with a focused shape equivalent to:

```sql
project_repository_links
  id uuid primary key
  project_id uuid not null references projects(id)
  url text not null
  is_primary boolean not null
  availability text not null
  checked_at timestamptz not null
  sort_order integer not null
  actor_id uuid references application_users(id)
  created_at timestamptz not null
  updated_at timestamptz not null
```

Required constraints:

- URL is nonempty and no longer than 2048 characters;
- availability is `accessible`, `not_accessible`, or `unverified`;
- `sort_order` is nonnegative;
- URL is unique within a Project;
- sort order is unique within a Project;
- at most one primary repository exists per Project.

Do not add storage keys, storage-provider markers, MIME types, byte counts,
digests, deletion status, Artifact type, or public serving routes. Repository
links do not own bytes.

The Project Content source is authoritative for a declared link set. Replacing
that set may delete and reinsert rows inside one transaction. Soft-deleted link
history is unnecessary because the audit record captures the mutation and the
development reset model treats generated inputs as source of truth.

## Repository-link service

Place the boundary in a focused package such as
`backend/internal/projectlinks`. Avoid adding link behavior to the Artifact
service or scattering direct SQL through import and HTTP code.

Use a model equivalent to:

```go
type Link struct {
    ID           uuid.UUID
    ProjectID    uuid.UUID
    URL          string
    IsPrimary    bool
    Availability string
    CheckedAt    time.Time
    SortOrder    int
}
```

The service must support:

- validation and normalization of a complete ordered input set;
- retrieval in `sort_order, id` order;
- transactional comparison with the current set;
- an unchanged result without writes, Project revision changes, audit, or
  search synchronization;
- authoritative replacement, including replacement with an empty set;
- Project row locking and expected-revision enforcement;
- rejection of deleted Projects;
- one Project revision increment for a changed set;
- one audit event, `project_repository_links.replaced`, with old count, new
  count, and resulting Project revision but no hidden extractor provenance;
- pending search synchronization using the normal upsert/remove action for the
  Project's status, even though links are not indexed, so the derived document
  revision remains aligned with PostgreSQL.

URL validation must use the standard library:

- trim surrounding whitespace;
- require an absolute `https` URL with a nonempty hostname;
- reject embedded username or password credentials;
- reject control characters;
- discard or reject fragments consistently, with rejection preferred for a
  strict import contract;
- preserve meaningful path and query case;
- reject exact duplicate URLs after normalization;
- enforce one primary link for a nonempty set.

Do not create a broad generic URL-management framework. Validation exists for
this Project-owned repository-link boundary only.

## Project Content manifest contract

Extend the existing strict manifest version `1` additively. Do not revive the
removed Artifact CSV importer and do not add a legacy parser branch.

```json
{
  "version": 1,
  "projects": [
    {
      "project_import_key": "sp-1925",
      "logo": {
        "file_path": "logos/1925.png"
      },
      "files": [
        {
          "artifact_type": "report",
          "display_name": "Final report",
          "file_path": "projects/sp-1925/report.pdf"
        }
      ],
      "links": [
        {
          "url": "https://github.com/williampoch/sp1",
          "primary": true,
          "availability": "accessible",
          "checked_at": "2026-09-11T07:32:27Z"
        },
        {
          "url": "https://github.com/williampoch/sp1website",
          "primary": false,
          "availability": "accessible",
          "checked_at": "2026-09-11T07:32:30Z"
        }
      ]
    }
  ]
}
```

Manifest semantics:

- `links` is optional.
- Omitted `links` leaves existing repository links unchanged.
- Present `links` is the complete desired set for that Project.
- Present `links: []` removes all repository links for that Project.
- `null` is rejected. The loader must retain field presence so omitted and
  empty have distinct meanings.
- A Project entry may contain only a present `links` field, including an empty
  array, or may combine links with Logo and Project Files.
- Each link requires `url`, `primary`, `availability`, and `checked_at`.
- Unknown fields, duplicate URLs, invalid timestamps, invalid availability,
  non-HTTPS URLs, multiple or missing primary links in a nonempty set, and
  unresolved Projects fail whole-manifest planning before writes.
- Project order and link order remain deterministic.

Map extractor liveness only at bundle-generation time:

```text
project_repo + public     -> accessible
project_repo + not_found  -> not_accessible
project_repo + unknown    -> unverified
```

Ignore `final_url`, HTTP status, source file, source page, extraction method,
and all `third_party_reference` entries. The original normalized Project URL
is the application value.

Update Project Content planning and reporting with clear counts for declared
links, changed Project link sets, and unchanged link sets. Apply keeps the
existing one-line-per-Project output. A representative outcome can say
`links replaced (2 repositories)` or `links unchanged (2 repositories)`.
Links are not uploads and must not be included in byte totals.

Within one Project, preserve existing Logo-first and sequential Project File
behavior, then apply the repository-link set using a fresh Project revision.
A link-set failure marks that Project failed without rolling back already
committed Logo or Project File work. A rerun skips identical committed content
and resumes the remaining link set.

## Extractor evidence and operator preparation

The nested extractor repository remains read-only. Add a documented `jq`
preparation example to the main repository's Project Content operator note.
The example must:

- retain only metadata CSV import keys;
- select only `kind == "project_repo"`;
- use each entry's normalized original URL;
- map liveness to the three application availability values;
- retain `primary` and `checked_at`;
- produce manifest version `1` accepted by the real loader;
- produce 13 Project entries and 15 links against the current reviewed CSV;
- permit a link-only run through the same `project-content import-manifest`
  command;
- explain that the same arrays can be included in the final combined Logo and
  Project File bundle.

Use an opt-in guarded evidence test or an explicit temporary-manifest command
to run the real loader and planner against those 15 links and a disposable
database containing matching import keys. Do not commit generated extractor
output or alter the nested repository.

## OpenAPI and public response

Add a schema equivalent to:

```yaml
ProjectRepositoryLink:
  type: object
  additionalProperties: false
  required: [url, primary, availability, checked_at]
  properties:
    url:
      type: string
      format: uri
      maxLength: 2048
    primary:
      type: boolean
    availability:
      type: string
      enum: [accessible, not_accessible, unverified]
    checked_at:
      $ref: '#/components/schemas/Timestamp'
```

Add required `repository_links`, always an array, to `PublicProject`. Do not
add links to `ProjectSummary`, `SearchResult`, `PublicPersonProject`, Artifact,
or Project Logo schemas. `AdminProject` may remain unchanged because no
administrator link UI is included.

Regenerate Go and TypeScript clients from OpenAPI. Never hand-edit generated
files.

The public Project controller loads links only after the parent Project passes
its existing published-only lookup. Draft and deleted Projects retain the
existing Project-level 404 behavior. No separate link endpoint is needed.

## Frontend behavior

Add a Repositories section to the public Project detail page only when
`repository_links` is nonempty. Keep the existing typography, panels, spacing,
responsive behavior, and localization pattern.

For each repository:

- use the URL or a safely shortened host/path form as visible link text;
- keep the full URL available through the anchor destination;
- open in a new tab with `target="_blank"` and
  `rel="noopener noreferrer"`;
- label the primary repository in text when more than one exists;
- show one non-color-only availability label:
  `Accessible`, `Not accessible (private or deleted)`, or `Unverified`;
- show the localized exact check date rather than a continuously changing
  relative-time timer;
- use the existing semantic success color for Accessible only as a supplement
  to the text;
- prevent long URLs from causing page-level horizontal overflow.

Availability is historical evidence, not click gating. Every link remains
clickable. The application must not fetch the external URL from the browser or
Backend API merely to render the page.

## Focused verification

### Migration and service

- Fresh migration and rollback create and remove the table cleanly.
- Constraints reject duplicate URLs, duplicate order, multiple primary rows,
  invalid availability, and negative order.
- Validation rejects HTTP, credentials, fragments, controls, overlong URLs,
  invalid timestamps, duplicate normalized URLs, and invalid primary sets.
- Replace inserts an ordered set, advances Project revision once, writes one
  audit event, and maintains pending search revision.
- Exact rerun is unchanged and produces no writes, revision change, audit, or
  search update.
- Changed status, timestamp, order, primary choice, URL, or cardinality
  replaces the set once.
- Explicit empty set removes all rows; omitted links do nothing.
- Deleted or revision-stale Project mutation fails safely.

### Project Content importer

- Old valid Logo and Project File manifests without `links` remain accepted by
  the same version-1 parser with no compatibility branch.
- A link-only entry and a combined Logo, files, and links entry both plan and
  apply.
- Invalid link data prevents every write in whole-manifest planning.
- One Project progress line reports replaced or unchanged links.
- A rerun skips an unchanged link set.
- A failure after earlier Project Content commits recovers on rerun.
- Current extractor evidence resolves 13 Projects and plans 15 links while
  importing zero third-party references.

### API and frontend

- Published Project response returns ordered links and an empty array when
  absent.
- Draft and deleted Projects remain unavailable publicly.
- No repository link appears in search documents, facets, Artifact counts, or
  Project File lists.
- Generated clients match OpenAPI.
- The Repositories section is absent for no links and renders all links when
  present.
- Primary, availability, and exact checked date are visible.
- External link security attributes are present.
- Long URLs do not create page-level overflow.
- Availability does not disable navigation.

Run focused checks only:

- migration and PostgreSQL-backed service tests for the new package;
- `backend/internal/projectcontentimport` and `backend/cmd/ausectl`;
- focused `backend/internal/platform/httpserver` response tests;
- OpenAPI generation-drift checks;
- frontend lint, TypeScript build, and affected component tests;
- one default-subpath frontend production build only if generated client or
  routing changes require it;
- `go vet` for affected packages, tracked `gofmt`, and `git diff --check`.

Do not run Playwright, full product acceptance, live URL checks, live B2 bulk
imports, stack rebuilds, image publication, or VM deployment.

## Documentation and memory

After focused verification:

- extend `.memory/sasta-dev-notes/project-content-import.md` with the `links`
  manifest contract, extractor mapping, current 13-Project and 15-link evidence,
  dry-run, apply, rerun, and combined-bundle behavior;
- update backlog item 18 in `.memory/mvp_implementation_status.md` to
  implemented with concise verification evidence;
- update `.memory/CONTEXT.md` only if implementation materially changes the
  term already recorded;
- add a lesson only for a new durable failure pattern.

No public documentation change is required.

## Suggested implementation order

1. Add migration 00004 and schema-version support.
2. Implement link validation, ordered retrieval, comparison, and transactional
   replacement.
3. Add focused migration and service integration tests.
4. Extend the strict Project Content manifest, planner, apply path, results,
   and progress output.
5. Add extractor-derived guarded planning evidence.
6. Extend OpenAPI and regenerate clients.
7. Project the links into the public Project response.
8. Add the public detail Repositories section and focused component tests.
9. Run the bounded verification set once.
10. Update operator guidance and implementation status.
11. Commit once and stop for independent review.

## Completion criteria

- One Project owns zero or more ordered Project Repository Links.
- Repository links use PostgreSQL metadata and never create Artifact or B2
  records.
- The unified Project Content manifest handles files, Logo, and links without
  another CSV or command.
- Omitted, empty, changed, and unchanged link sets have explicit safe behavior.
- Only verified `project_repo` evidence is eligible.
- Public Project details display every link with textual last-checked status.
- Search and Project File behavior remain unchanged.
- No runtime external liveness request is introduced.
- Focused verification passes without broad acceptance or deployment.
- One coherent commit uses a lowercase past-tense message with no prefix and no
  co-author trailer.

## Required handoff

Stop after the implementation commit. Report:

- accepted Increment 14 baseline and final commit hash;
- migration and supported schema version;
- table constraints and service transaction behavior;
- URL and availability validation contract;
- exact Project Content manifest extension and omitted versus empty semantics;
- dry-run, apply, progress, idempotency, and removal behavior;
- extractor evidence counts and confirmation that third-party references were
  excluded;
- public OpenAPI and UI behavior;
- focused commands and results;
- failures encountered and residual limitations;
- confirmation that no separate CSV/importer, Artifact storage, runtime URL
  checks, nested extractor changes, live B2 work, stack rebuild, image, or VM
  work occurred;
- final `git status --short`.
