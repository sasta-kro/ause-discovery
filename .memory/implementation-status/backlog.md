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
| 22 | Open | Newest-first academic search ordering |
| 23 | Diagnose first | Classification filter match-all behavior |
| Correction | Partial | Acknowledge all import warnings |
| Correction | Open | Bounded import-preview text layout |
| 24 | Parked | Optional Project Logo payload optimization |

## Current activity

- No technical MVP implementation work remains. Launch readiness begins when the external inputs listed below are available.

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
- **Open:** import preview rows wrap text permanently; browser zoom does not restore layout. Long values need bounded truncation or normal wrapping inside a horizontally scrollable table region.

## Open implementation work

10. **Open.** Storage terminology refactor is due. `Backend` is reserved for the AUSE Backend API, the application service connecting the frontend, PostgreSQL, search, and supporting services. It must not refer generically to a B2 bucket or local filesystem. Use `Artifact store` for the byte-storage concept, `storage provider` for a configured implementation such as `local` or `b2`, and `storage adapter` for the Go implementation. Required rename scope includes the storage interface and implementations, `StorageSet`, `storage_backend` database marker, configuration names and comments, CLI output, internal operator notes, tests, and user-facing wording. Preferred direction: `ArtifactStore` or `StorageProvider` interface, `LocalArtifactStore`, `B2ArtifactStore`, and `storage_provider`. This is not optional polish. The overloaded name obscures architecture and must be removed in a coherent follow-up refactor.

16. **Open, report first and change only after maintainer review.** Conduct a codebase simplification audit after the current publication cycle. Identify duplicated logic, obsolete development compatibility paths, unjustified abstractions, unreachable defensive branches, stale naming, and complexity introduced by agentic implementation. Explain why each candidate exists, its current value, and the risk of removing it. Preserve real replacement boundaries such as storage providers, search projection, versioned catalogs, and malleable taxonomy. Implementation follows only for candidates explicitly accepted after the report.

17. **Open, later product increment.** Redesign the administrator experience for sustained real-world maintenance rather than the current development import workflow. The future pass must cover Project and Person discovery, role assignment, large option sets, validation and conflict recovery, Project Files and Logos, bulk operations, and efficient editing without assuming that administrators will recreate the source CSV workflow. Scope and interaction design require discussion before an implementation brief.

19. **Open, future product increment.** Project File types admit image files only under `poster`. Award and achievement material in the corpus therefore has no importable type: the sp-2039 external material (YRSS 2021 bronze prize, two jpg files and one png file) stays out of the content bundle by maintainer decision on 2026-09-12, and the extractor build reports each skipped image as a warning. A future implementation should let such evidence import. Candidate designs: permit jpg/jpeg/png under the `other` type, or add a dedicated type such as `award` with its own public label and frontend rendering. The full case record is in the extractor's `notes/bundle-building.md`.

20. **Open, future product increment.** The import schema rejects any metadata row without at least one advisor (`missing_advisor` error, `backend/internal/imports/validation.go`). Four corpus projects genuinely print no advisor in any staged document (verified by full-text search on 2026-09-13: sp-1800 report without an approval page, and the slide-only decks sp-2021, sp-2031, sp-2032). The maintainer dropped these four from the import CSV for now (documented in the extractor's `DROPPED_NO_ADVISOR` set) rather than invent names, and the dataset records remain as ground truth. A future implementation should treat a missing advisor as an import warning with an honest public display ("advisor not recorded"), so advisorless legacy documents can join the archive.

22. **Open, MVP search-ordering improvement.** Empty-query discovery should show the newest academic Projects first by default instead of inheriting Meilisearch's current document order or treating an empty query as ordinary relevance. Define newest by academic chronology, primarily `academic_year DESC` and then the accepted semester order, with deterministic tie-breakers such as publication timestamp and stable Project identity. A nonempty text query must keep lexical relevance as the primary ordering, then use the same newest-first chronology to order otherwise equally ranked results. Review the Meilisearch ranking rules because custom `sort` currently precedes `exactness`; the chronological tie-breaker must not displace any intended relevance rule. Update the explicit Newest and Oldest options to use academic chronology rather than only `published_at`, keep Title sorting deterministic, bind the resolved ordering into cursor state, and add service, HTTP, and frontend state tests for empty queries, nonempty equal-relevance results, semester boundaries, and pagination. Artifact or Logo presence may correlate with newer Projects but must not become a hidden ranking boost under this item.

23. **Open, suspected filter-semantics defect requiring diagnosis.** Selecting multiple values in some left-side Classification filters increases the result count, while the expected refinement behavior is to return Projects carrying every selected classification in that dimension. The current backend deliberately emits `attribute IN [a,b]`, which gives OR semantics within one facet, while separate facet expressions are combined with AND. First reproduce the issue against known Projects and record request parameters, result IDs, and counts for Category, Platform, Domain, and Technology independently and in combination. Then decide semantics by field cardinality: multi-valued classification dimensions should support match-all behavior when that matches the maintained taxonomy model, while single-valued dimensions such as Program, Major, Course, Academic Year, and Semester normally require OR within their own dimension. If classification match-all is accepted, emit one filter expression per selected classification value, verify Meilisearch array matching and facet distributions, update UI help or selection wording, and add backend plus rendered interaction tests proving that each additional classification narrows or preserves the result set. Do not change behavior until the reproduction distinguishes a backend filter bug, a facet-count misunderstanding, stale URL state, and the currently documented OR-within-facet contract.

## Parked possibilities

24. **Parked possibility, not planned.** Project Logo payload optimization may be reconsidered only if later evidence shows that full-resolution PNG transfer size causes a meaningful problem after Increment 16. Possible directions include a bounded display derivative, lossless optimization, or responsive variants. No downscaling, cropping, alternate image format, CDN, storage change, or follow-up increment is currently committed.
