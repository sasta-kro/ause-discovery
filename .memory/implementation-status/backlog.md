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
| 24 | Parked | Optional Project Logo payload optimization |
| 26 | Public copy implemented; private agreement remains | Ownership, institutional license, public legal text, and developer credit |
| 27 | Deferred | Privacy-preserving telemetry and administrator analytics |
| 31 | External policy decision | Public or organization-only Project File access |

## Current activity

- No technical MVP implementation work remains. Launch readiness begins when the external inputs listed below are available.
- Increment 19, Person-name Search Filter Suggestions, was implemented at
  commit `eb84ae7` on 2026-09-15 and independently accepted; its record moved
  to `completed.md`.
- Increment 20 was implemented at commit `eedeecd` and independently accepted
  on 2026-09-15. Its record moved to `completed.md`.
- Increment 21 was implemented at `0591de0`, corrected at `170f5a8`, and
  independently accepted on 2026-09-15. Its record moved to `completed.md`.
- Increment 22 was implemented at `afb6864`, corrected at `7388627`, and
  independently accepted on 2026-09-15. Its record moved to `completed.md`.
- Increment 23 implemented the factual developer attribution portion on
  2026-09-16 per
  `.memory/docs/increments/23-developer-attribution-and-session-cookie-disclosure.md`.
  The shared footer and About page credit Sai Aike Shwe Tun Aung as Developer
  and maintainer with a GitHub profile link, and the administrator sign-in
  carries the compact strictly-necessary authentication-cookie disclosure. A
  direct product-owner correction on 2026-09-16 replaced public
  pending-approval placeholders with current Terms, Privacy, Accessibility,
  and Contact copy. The implementation commit `7af6120` and direct correction
  `5b48ccb` are accepted. The private institutional operating agreement remains
  separate.

No status-specific in-progress file exists. Add one only when work must remain
active across sessions without being represented by an implementation brief.

## Current operating constraints

### Production data preservation policy

Superseded 2026-09-14: the earlier disposable-data rebuild policy no longer
applies to the hosted `ause-discovery` Compose project or its Backblaze B2
bucket. Production data carries soft preservation: everything is rebuildable
from the reviewed metadata CSV and Project Content bundle, but a full
re-import costs roughly 10 to 15 minutes for about 2 GB of Project File and
Logo bytes, so routine work avoids it. Routine image updates, container
recreation, migration, and search rebuild preserve the production PostgreSQL
volume and B2 objects, omit metadata and Project Content re-import, and never
use `docker compose down -v`. A deliberate production wipe is acceptable when
the maintainer intends it, for example after a data-model or storage-layout
change or corruption, and is executed as a coordinated reset: PostgreSQL and
B2 together, since PostgreSQL owns every Project-to-object association and
opaque storage key. Meilisearch remains disposable derived state. Separately
named local test Compose projects remain fully disposable and may use
deliberately selected non-production storage targets.

### Toolchain constraints

- The host Node.js version, 24.14.1, is below the exact repository baseline, 24.20.0. Routine frontend lint, tests, and builds run on host pnpm regardless; the pinned Node container is used for generated-client regeneration.
- Host Go 1.27.1 exists but differs from the pinned Go 1.27.0 baseline. Makefile Go verification still runs through the pinned `golang:1.27.0-alpine3.23` container.
- No `typescript-eslint` release supports TypeScript 7.0.2, the native compiler generation (upstream tracking: typescript-eslint issue 10940). The inert `typescript-eslint` pin was removed on 2026-09-14. The lint gate runs ESLint on compatible JavaScript configuration only, and `tsc -b` provides all TypeScript static checking. Real TypeScript linting requires either a TypeScript 6 side-by-side setup or upstream tsgo support.
- `@hey-api/openapi-ts` 0.99.0 rejects TypeScript 7.0.2. Reproducible client generation uses the isolated `api/generator` workspace with TypeScript 5.9.3 while application compilation remains on TypeScript 7.0.2.

### Launch-only inputs

GitHub ownership, registry ownership, production URL, VM details, branding, TLS ownership, and final institutional catalog content remain launch-readiness inputs rather than technical MVP blockers.

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

## Open implementation work

10. **Open.** Storage terminology refactor is due. `Backend` is reserved for the AUSE Backend API, the application service connecting the frontend, PostgreSQL, search, and supporting services. It must not refer generically to a B2 bucket or local filesystem. Use `Artifact store` for the byte-storage concept, `storage provider` for a configured implementation such as `local` or `b2`, and `storage adapter` for the Go implementation. Required rename scope includes the storage interface and implementations, `StorageSet`, `storage_backend` database marker, configuration names and comments, CLI output, internal operator notes, tests, and user-facing wording. Preferred direction: `ArtifactStore` or `StorageProvider` interface, `LocalArtifactStore`, `B2ArtifactStore`, and `storage_provider`. This is not optional polish. The overloaded name obscures architecture and must be removed in a coherent follow-up refactor.

16. **Open, report first and change only after maintainer review.** Conduct a codebase simplification audit after the current publication cycle. Identify duplicated logic, obsolete development compatibility paths, unjustified abstractions, unreachable defensive branches, stale naming, and complexity introduced by agentic implementation. Explain why each candidate exists, its current value, and the risk of removing it. Preserve real replacement boundaries such as storage providers, search projection, versioned catalogs, and malleable taxonomy. Implementation follows only for candidates explicitly accepted after the report.

17. **Open, later product increment.** Redesign the administrator experience for sustained real-world maintenance rather than the current development import workflow. The future pass must cover Project and Person discovery, role assignment, large option sets, validation and conflict recovery, Project Files and Logos, bulk operations, and efficient editing without assuming that administrators will recreate the source CSV workflow. Scope and interaction design require discussion before an implementation brief.

19. **Open, future product increment.** Project File types admit image files only under `poster`. Award and achievement material in the corpus therefore has no importable type: the sp-2039 external material (YRSS 2021 bronze prize, two jpg files and one png file) stays out of the content bundle by maintainer decision on 2026-09-12, and the extractor build reports each skipped image as a warning. A future implementation should let such evidence import. Candidate designs: permit jpg/jpeg/png under the `other` type, or add a dedicated type such as `award` with its own public label and frontend rendering. The full case record is in the extractor's `notes/bundle-building.md`.

20. **Open, future product increment.** The import schema rejects any metadata row without at least one advisor (`missing_advisor` error, `backend/internal/imports/validation.go`). Four corpus projects genuinely print no advisor in any staged document (verified by full-text search on 2026-09-13: sp-1800 report without an approval page, and the slide-only decks sp-2021, sp-2031, sp-2032). The maintainer dropped these four from the import CSV for now (documented in the extractor's `DROPPED_NO_ADVISOR` set) rather than invent names, and the dataset records remain as ground truth. A future implementation should treat a missing advisor as an import warning with an honest public display ("advisor not recorded"), so advisorless legacy documents can join the archive.

26. **Public copy implemented in Increment 23; the private institutional
operating agreement remains open.** The factual attribution portion shipped
2026-09-16 per
`.memory/docs/increments/23-developer-attribution-and-session-cookie-disclosure.md`:
the shared footer credits Sai Aike Shwe Tun Aung as developer and maintainer
with a profile link to `https://github.com/sasta-kro`, the About page
separates software development from institutional content stewardship, and
the administrator sign-in carries the compact strictly-necessary
authentication-cookie disclosure. The public Terms, Privacy, Accessibility,
and Contact copy records current behavior and the settled educational,
non-commercial, content-rights, anti-abuse, correction, and contact boundaries
without exposing internal approval status. No public software copyright notice,
executed software license, personal email address, or repository link was
published.

AUSE Discovery was independently initiated and developed by bachelor Computer
Science student Sai Aike Shwe Tun Aung, without an employee role, official
university position, formal commission, or current written ownership
agreement. The selected private-agreement direction remains developer ownership
of the software plus a written institutional operating license that lets
Assumption University host, operate, reproduce, back up, and maintain it,
including reasonable security and continuity work, without creating an
absolute modification prohibition. Exact modification, successor-maintainer,
redistribution, commercialization, termination, and attribution rights remain
for a written agreement and qualified Thai legal review. This private agreement
is not a public-page placeholder or a blocker for the current site copy.

Public attribution is settled for Increment 23: use the accurate role `Developer and
maintainer`, the name `Sai Aike Shwe Tun Aung`, and the GitHub profile
`https://github.com/sasta-kro`. Do not invent a university title. Direct email
publication is deferred to avoid spam; use GitHub as the initial public contact
route. A public source-repository link is also deferred. The intended UI is a
short footer credit and a fuller About section that distinguishes software
development and maintenance from institutional content stewardship. Increment
23 implements this attribution plus a compact factual disclosure beside the
administrator sign-in form for the existing strictly necessary authentication
cookies.

Public copy states neutrally that Project metadata is transcribed and processed
from institutionally supplied source material, while corrections and
content-policy decisions are handled through authorized university
administrators. It does not make the developer the guarantor of report or
metadata accuracy, the owner of submitted Project content, or the institutional
contact for content decisions. The future public-versus-organization-only
Project File access decision remains item 31 and will require corresponding
policy-copy updates when resolved.

27. **Deferred, far-future product work separate from item 26.**
Add first-party telemetry for institutional insight and an administrator-only
analytics area with appropriate tables, graphs, and charts. Candidate measures
include search phrases, zero-result searches, selected filters, traffic by
time, Project views, Person views, and Project File opens or downloads. Design
must precede implementation and cover event definitions, aggregation, search
term sensitivity, bots, retention, deletion, lawful basis, privacy notice,
cookie or consent requirements, administrator authorization, and operational
cost. The preferred first stage is cookieless, privacy-minimized aggregate
telemetry without cross-site tracking, advertising identifiers, fingerprinting,
or persistent public visitor identities. Unique-visitor or session analytics
remain a separate decision. This work requires backend persistence and APIs as
well as frontend dashboards and is not a frontend-only addition. No telemetry,
analytics cookie, visitor identifier, or implementation brief is planned until
the maintainer explicitly resumes this item.




31. **External policy decision; no implementation brief yet. Public or
organization-only Project File access.** The university administrator has not
yet decided whether Project Files remain publicly viewable and downloadable or
become available only to authenticated Assumption University organization
members. Preserve the current public Artifact contract until that decision is
authorized. The candidate restricted model uses Microsoft Entra ID for any
organization member without role differentiation for file access, while local
administrator authorization remains a separate privileged concern unless later
decided otherwise. Public Project metadata and search are expected to remain.
If restricted access is selected, the future increment must cover the public
`/artifacts/{artifact_id}/view` and `/artifacts/{artifact_id}/download`
endpoints, inline PDF viewing, public View and Download actions, Artifact
availability search facets, tenant and application registration, OIDC sessions,
cookies, CSRF, rate limiting, public locked-file presentation, Project Logos,
Repository Links, and administrator previews. Confirm the university's rights
and policy for submitted Project content before choosing either public or
organization-only delivery.

## Parked possibilities

24. **Parked possibility, not planned.** Project Logo payload optimization may be reconsidered only if later evidence shows that full-resolution PNG transfer size causes a meaningful problem after Increment 16. Possible directions include a bounded display derivative, lossless optimization, or responsive variants. No downscaling, cropping, alternate image format, CDN, storage change, or follow-up increment is currently committed.
