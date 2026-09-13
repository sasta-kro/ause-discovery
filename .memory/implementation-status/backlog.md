# Implementation backlog

This file is the default implementation-status reading route for planning.
Completed work and historical verification evidence belong in `completed.md`.

## Backlog index

| Item | Status | Topic |
| --- | --- | --- |
| 1 | Recurring | Catalog synchronization on each fresh stack |
| 9 | Partial | Storage correctness, availability distinction, and recovery limits |
| 10 | Open | Storage terminology refactor |
| 16 | Open, review first | Codebase simplification audit |
| 17 | Open | Administrator experience redesign |
| 19 | Open | Award and achievement Project File support |
| 20 | Open | Projects without a recorded advisor |
| Correction | Partial | Acknowledge all import warnings |
| Correction | Open | Bounded import-preview text layout |
| Correction | Open | Explain classification filter combination semantics |
| 24 | Parked | Optional Project Logo payload optimization |
| 25 | Implemented as Increment 18, pending independent review | Search Filter Suggestions |

## Current activity

- No technical MVP implementation work remains. Launch readiness begins when the external inputs listed below are available.
- Increment 18 implemented Search Filter Suggestions on 2026-09-13 per
  `.memory/docs/increments/18-search-filter-suggestions.md`. The public search
  input is now an accessible combobox that suggests exact or prefix matches of
  already-loaded Academic year, Semester, Program, Course, Category, Platform,
  Domain, and Technology controlled values. One Tab action converts exactly one
  nearest-suffix suggestion through the shared filter mutation, Enter keeps
  free-text search, and Escape dismisses then blurs. The item awaits
  independent review.

No status-specific in-progress file exists. Add one only when work must remain
active across sessions without being represented by an implementation brief.

## Current operating constraints

### Disposable-data rebuild policy

Recorded 2026-09-12: no current PostgreSQL, Meilisearch, local Artifact, or B2 data requires preservation. The reviewed metadata CSV and the versioned Project Content manifest plus its referenced local files are the development sources of truth. A fully fresh rehearsal may stop the application, remove the intended Compose project's volumes, empty every object version from the B2 bucket, bootstrap and synchronize catalogs, import Project metadata from the reviewed CSV, then import logos and Project Files from the Project Content bundle. PostgreSQL owns the mapping from stored objects to Projects, so preserving B2 while wiping PostgreSQL leaves unreferenced objects and does not provide an idempotent rebuild. This disposable-data policy remains active until the maintainer explicitly declares a non-disposable production dataset or manual administrative changes that must survive rebuilds.

### Toolchain constraints

- The host Node.js version is below the exact repository baseline; the pinned container toolchain provides the reproducible path.
- Go is absent on the host; Go verification uses the pinned container toolchain.
- `typescript-eslint` 8.69.0 rejects TypeScript 7.0.2. The current lint gate runs ESLint on compatible JavaScript configuration and uses `tsc -b` for TypeScript static checking while preserving the accepted exact pins.
- `@hey-api/openapi-ts` 0.99.0 rejects TypeScript 7.0.2. Reproducible client generation uses the isolated `api/generator` workspace with TypeScript 5.9.3 while application compilation remains on TypeScript 7.0.2.

### Launch-only inputs

GitHub ownership, registry ownership, production URL, VM details, branding, approved legal content, privacy approval, TLS ownership, and final institutional catalog content remain launch-readiness inputs rather than technical MVP blockers.

## Recurring operations

Recorded 2026-09-05 after the SP metadata extraction work in `tools/sp-import/`. The product specification defers exact vocabularies (section 59) and the roadmap (section 41) remain the governing documents; this list tracks concrete accepted follow-ups so any session can pick them up.

Applied to version-controlled catalogs, pending the next `ausectl catalog sync` (`make seed`) to reach PostgreSQL:

- `config/catalogs/academic.yaml` now covers pre-2020 years (`valid_from_year: 1990`) and adds the `information_technology` program; the `senior_project` course version links both program versions. The SP corpus (2016-2025, IT and CS departments) imports against both programs.
- `config/taxonomy/values.yaml` gained nine keys: topic `recommender_system`; technologies `nextjs`, `nodejs`, `mongodb`, `laravel`, `wordpress`, `kotlin`, `mysql`, `unreal_engine`. Extension policy: only abstract-verifiable implementation stacks, no overlap with broader facets (Raspberry Pi and wearables stay under `embedded_hardware`, sentiment analysis under `nlp`, and so on). The extractor's unmapped-vocabulary report lists deliberately rejected candidates.

1. **Operational, recurring.** Run `make seed` or `ausectl catalog sync` on each fresh stack bring-up. Catalog synchronization is implemented, additive, and non-destructive.

## Partially completed work

9. **Partially implemented.** Storage readiness now checks the configured provider per item 14. Two required correctness fixes remain open: advance the B2 ranged reader's logical offset after each successful read, and replace boolean existence checks with a contract that distinguishes missing content from provider unavailability. Changing the configured default still does not provide failover for existing provider-stamped rows. Automatic second copies, reverse migration, and fallback tooling remain deferred. B2 upload validation stages each file in the container operating system temporary directory rather than `AUSE_IMPORT_TEMP_ROOT`; that accepted disk requirement is documented.

- **Partially implemented:** the select-all-valid-rows action is implemented (2026-09-08) as one persisted row update that walks every preview page and selects only rows whose server state is valid. An acknowledge-all-warnings action remains open; error rows still require correction and ambiguous duplicates still require explicit decisions.
- **Open:** import preview rows wrap text permanently. The 200 percent zoom
  behavior is resolved. Long values still need bounded truncation or normal
  wrapping inside a horizontally scrollable table region.
- **Open:** add concise user-facing guidance that multiple selected values
  within one filter dimension match any selected value, while different filter
  dimensions must all match. The accepted search behavior does not change.

## Open implementation work

10. **Open.** Storage terminology refactor is due. `Backend` is reserved for the AUSE Backend API, the application service connecting the frontend, PostgreSQL, search, and supporting services. It must not refer generically to a B2 bucket or local filesystem. Use `Artifact store` for the byte-storage concept, `storage provider` for a configured implementation such as `local` or `b2`, and `storage adapter` for the Go implementation. Required rename scope includes the storage interface and implementations, `StorageSet`, `storage_backend` database marker, configuration names and comments, CLI output, internal operator notes, tests, and user-facing wording. Preferred direction: `ArtifactStore` or `StorageProvider` interface, `LocalArtifactStore`, `B2ArtifactStore`, and `storage_provider`. This is not optional polish. The overloaded name obscures architecture and must be removed in a coherent follow-up refactor.

16. **Open, report first and change only after maintainer review.** Conduct a codebase simplification audit after the current publication cycle. Identify duplicated logic, obsolete development compatibility paths, unjustified abstractions, unreachable defensive branches, stale naming, and complexity introduced by agentic implementation. Explain why each candidate exists, its current value, and the risk of removing it. Preserve real replacement boundaries such as storage providers, search projection, versioned catalogs, and malleable taxonomy. Implementation follows only for candidates explicitly accepted after the report.

17. **Open, later product increment.** Redesign the administrator experience for sustained real-world maintenance rather than the current development import workflow. The future pass must cover Project and Person discovery, role assignment, large option sets, validation and conflict recovery, Project Files and Logos, bulk operations, and efficient editing without assuming that administrators will recreate the source CSV workflow. Scope and interaction design require discussion before an implementation brief.

19. **Open, future product increment.** Project File types admit image files only under `poster`. Award and achievement material in the corpus therefore has no importable type: the sp-2039 external material (YRSS 2021 bronze prize, two jpg files and one png file) stays out of the content bundle by maintainer decision on 2026-09-12, and the extractor build reports each skipped image as a warning. A future implementation should let such evidence import. Candidate designs: permit jpg/jpeg/png under the `other` type, or add a dedicated type such as `award` with its own public label and frontend rendering. The full case record is in the extractor's `notes/bundle-building.md`.

20. **Open, future product increment.** The import schema rejects any metadata row without at least one advisor (`missing_advisor` error, `backend/internal/imports/validation.go`). Four corpus projects genuinely print no advisor in any staged document (verified by full-text search on 2026-09-13: sp-1800 report without an approval page, and the slide-only decks sp-2021, sp-2031, sp-2032). The maintainer dropped these four from the import CSV for now (documented in the extractor's `DROPPED_NO_ADVISOR` set) rather than invent names, and the dataset records remain as ground truth. A future implementation should treat a missing advisor as an import warning with an honest public display ("advisor not recorded"), so advisorless legacy documents can join the archive.

25. **Implemented as Increment 18, pending independent review.** Implemented
2026-09-13 per `.memory/docs/increments/18-search-filter-suggestions.md`. The
public search input is an editable combobox with `aria-autocomplete="list"`
semantics. Suggestions are computed locally from the loaded facet and catalog
choices, use case-insensitive exact or prefix matching with a three-character
threshold (a complete shorter alias and an existing four-digit academic year
remain eligible), never fuzzy-match, exclude already-selected values, label
collisions by dimension, and cap the list at six ranked rows. One Tab action
converts exactly the highlighted suggestion, applies it through the same
filter mutation as the left panel, removes only the consumed trailing range
plus boundary whitespace, resets pagination, and triggers one filtered search.
Enter always submits the draft as free text, and Escape dismisses then blurs
in two stages. Typing alone never requests results.

Implement
`.memory/docs/increments/18-search-filter-suggestions.md`. Add Search Filter
Suggestions to the
public search input. Typing a controlled value such as `2025` or `game` should
offer dimension-labelled completions such as `Academic year: 2025` or
`Category: Game`. Accepting a suggestion adds the same structured URL filter
and selected-filter chip as the left filter panel, removes the recognized text
from the input while preserving every unmatched free-text term, resets
pagination, and triggers one filtered search. Suggestions should initially
cover active academic and classification dimensions already available through
loaded catalogs and search facets and exclude already-selected values.

Only the nearest eligible suffix ending at the caret may become a filter. A
single Tab action converts exactly one highlighted suggestion, never every
controlled term in the input. After acceptance, suggestions immediately
recompute from the remaining text. For example, `machine learning 2025`
followed by Tab selects `Academic year: 2025` and leaves `machine learning`;
only then may `machine learning` become the next suggestion. `best gam`
followed by Tab selects `Category: Game` and leaves `best`. `best game project`
does not scan backward to suggest Game because the active trailing term is
`project`.

Matching is case-insensitive and requires a word boundary. Use normalized exact
or prefix matching with no typo correction, edit distance, or fuzzy dependency
for the initial version. Fuzzy behavior is deliberately provisional and may be
reconsidered only after the first implementation receives a manual interaction
review. A textual fragment normally requires at least three characters unless
it exactly matches a shorter controlled value; an academic year requires four
digits. A space after a completed unmatched term, unrelated trailing text, or
a misspelling closes suggestions rather than guessing intent. Removing or
correcting that text must reopen matching suggestions immediately. Rank exact
matches before prefixes and label every collision with its dimension.

This is filter autocomplete, not live result search, so typing alone must not
request results on every keystroke. Implement an accessible editable combobox
with a labelled listbox, automatic first-option highlight, mouse selection, and
arrow navigation. Enter always submits the free-text search and never accepts a
suggestion. Tab accepts the highlighted suggestion, prevents focus movement for
that action, and keeps focus in the input for continued composition. The active
suggestion must carry a small non-intrusive visual keycap and instruction such
as `<kbd>Tab</kbd> to select filter`; equivalent screen-reader instructions must
describe the same controls.

Escape uses two stages. The first Escape dismisses and suppresses the current
popup without moving the caret or removing input focus. Any subsequent text
change clears that suppression and recomputes suggestions immediately. A second
Escape with the same unchanged input and dismissed popup removes focus from the
search input; later Tab behavior returns to ordinary page navigation. The
implementation must verify that this focus transition remains predictable in a
real browser.

Suggestions should appear only while the caret is at the end so accepting one
can remove the matched suffix safely. Applying a suggestion should use the
remaining normalized text as the submitted query and share filter mutation
logic with the left panel. Prefer a small shared suggestion model over
duplicating that logic. A backend, OpenAPI, or search-engine change is
unnecessary unless later evidence shows that the already-loaded catalog and
facet data is insufficient.

## Parked possibilities

24. **Parked possibility, not planned.** Project Logo payload optimization may be reconsidered only if later evidence shows that full-resolution PNG transfer size causes a meaningful problem after Increment 16. Possible directions include a bounded display derivative, lossless optimization, or responsive variants. No downscaling, cropping, alternate image format, CDN, storage change, or follow-up increment is currently committed.
