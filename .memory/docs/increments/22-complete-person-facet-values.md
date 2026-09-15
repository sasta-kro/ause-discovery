# Complete Person facet values increment

## Assignment

Correct the public People facet and Person-name Search Filter Suggestions so
every Person participating in the current matching Project set remains
available at the archive's present scale. Remove the accidental dependence on
Meilisearch's default 100-value facet cap while preserving contextual counts,
filter semantics, local autocomplete, and the accepted frontend interactions.

This is a bounded search-settings bug fix. Start from accepted Increment 21
baseline `f455971`, including the intervening taxonomy change and documentation
cleanup. Preserve unrelated work.

## Required reading

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`, especially Person, Project Participation, Search
   Document, and Search Filter Suggestion
4. `.memory/implementation-status/README.md`
5. `.memory/implementation-status/backlog.md`, especially item 33
6. `.memory/implementation-status/completed.md`, especially Increments 17
   through 21
7. `.memory/docs/increments/17-newest-first-academic-search-ordering.md`
8. `.memory/docs/increments/19-person-filter-suggestions.md`
9. `.memory/docs/increments/20-filter-panel-ordering-and-focus.md`
10. `backend/internal/search/client.go`
11. `backend/internal/search/client_test.go`
12. `backend/internal/search/document.go`
13. `backend/internal/search/query.go`
14. `backend/internal/search/ordering_integration_test.go`
15. `frontend/src/app-shell.tsx`, limited to People and Advisor facet choices
    and suggestion construction
16. relevant SearchPage and browser tests
17. This brief

## Confirmed diagnosis

The defect is not a missing Person, import error, role error, or frontend label
problem.

Current local-test evidence from the 205-Project archive:

```text
People rows in PostgreSQL: 383
People participating in published Projects: 383
Empty-search People facet values returned: 100
Empty-search Advisor facet values returned: 23

Person: Phyo Min Tun
Person ID: 01a0a53f-ab5b-7a75-b514-d03ade6077a8
Published Project participations: 11
Roles: advisor, committee_member
Empty-search People facet: absent
Empty-search Advisor facet: present, count 6
Text search q=Phyo People facet: present, count 11
```

The active Meilisearch index reports:

```text
faceting.maxValuesPerFacet: 100
faceting.sortFacetValuesBy.*: alpha
```

The public People facet is built from the `person_ids` facet distribution.
Those facet values are UUID strings. Meilisearch truncates each distribution to
`maxValuesPerFacet` before the backend labels UUIDs from PostgreSQL. With the
default alphabetical facet ordering, the empty search keeps 100 UUIDs by their
string order and omits the other 283. Phyo Min Tun happens to be outside that
UUID subset despite appearing in 11 matching Projects.

The Advisor facet currently appears complete only because its distinct value
count is below 100. The same latent cap applies there.

Increment 19 intentionally reuses the already-loaded People and Advisor facets
for local Person-name suggestions. The missing People facet value therefore
also removes the `People: Phyo Min Tun` autocomplete row. Only the Advisor row
survives on the empty search.

## Domain boundary

The accepted meanings remain unchanged:

- Person is the canonical academic person record.
- Project Participation relates a Person to a Project as student, advisor,
  co-advisor, or committee member.
- People filters `person_ids` across every Project Participation role.
- Advisor filters the narrower `advisor_person_ids` projection.

Phyo Min Tun correctly belongs in both current filters. The People occurrence
count is 11 because it includes advisor and committee participation. The
Advisor count is 6 because it includes only the narrower advisor projection.
Different counts are expected and must not be merged.

No terminology or domain-model change is required. Item 28 remains separate.

## Selected fix

### Bounded complete facet distribution

Add explicit source-controlled Meilisearch faceting settings in
`MeilisearchClient.EnsureIndex`:

```text
maxValuesPerFacet: 1000

sortFacetValuesBy:
  *: alpha
  person_ids: count
  advisor_person_ids: count
```

Exact Go representation remains an implementation choice. Use one named
constant for the bound rather than an unexplained literal repeated in tests.

At the present 383-Person cardinality, the 1,000-value bound returns every
People value. The engine returns only actual values, not 1,000 placeholders, so
the current response grows to the real contextual cardinality. The bound leaves
room for archive growth without choosing an effectively unlimited value.

Count-first ordering for `person_ids` and `advisor_person_ids` is a defensive
truncation policy if either dimension eventually exceeds the bound. It ensures
the most frequent matching People survive rather than selecting UUIDs by
lexicographic accident. The backend and frontend may still apply their accepted
label and count presentation ordering after the distribution is received.

Keep `*` alphabetical for all other facets. Do not globally change every facet
to count ordering as part of this bug fix.

### Search schema version

Advance the source-controlled search schema version from 2 to 3 because the
version contract explicitly covers index settings. Update its comment and
focused tests accordingly.

Cursor hashes bind the schema version. Existing version-2 search cursors must
therefore fail through the existing controlled invalid-cursor behavior rather
than being accepted across the settings change. No result ordering rule changes.

### Existing-index convergence

`EnsureIndex` already patches settings for an existing active index and waits
for the accepted Meilisearch task. Preserve that behavior. The correction must
apply both to:

- the configured active index during API reconciler initialization;
- a physical replacement index created during a future search rebuild.

No document field changes, re-import, PostgreSQL migration, or mandatory full
search rebuild are required. An API restart with the corrected build applies
the settings to the active index. Operational verification must wait until the
settings task completes before checking the public facet response.

## Why not replace facets with PostgreSQL in this increment

A global PostgreSQL Person list would not be a drop-in replacement. Current
facet values and counts are contextual to the active text query and structured
filters. Replacing them with every Person row would expose values with no
matching Project, lose or duplicate contextual count computation, and require a
new API contract or server-side merge policy.

The present collection has hundreds, not thousands, of People. Raising the
bounded distribution cap is the smallest coherent correction that preserves
the current facet contract. A paged or searched Person-option service should be
considered only when observed cardinality or latency approaches this bound.

Do not add a separate People endpoint, direct browser access to Meilisearch, or
a request on each autocomplete keystroke.

## Required outcome

- The empty-search People facet includes all 383 currently published People in
  the local-test corpus, subject only to concurrent source-data changes.
- Phyo Min Tun appears in People with count 11 and Advisor with count 6 for the
  current corpus.
- The left People disclosure can find Phyo Min Tun through its local search.
- Typing `Phyo` in the main Search field offers separate People and Advisor rows
  without issuing a result or Person request.
- Tab still accepts only the highlighted dimension.
- People and Advisor counts remain contextual to the current matching Project
  set.
- Other facets, result ranking, sorting, pagination, and filter semantics remain
  unchanged.
- Existing indexes and future rebuild indexes receive identical explicit
  faceting settings.
- Facet output is no longer selected by UUID order while below the configured
  bound.

## Performance boundary

Meilisearch documents that high-cardinality facet distributions add search and
indexing cost. Keep the fix bounded and verify the actual archive rather than
assuming the change is free.

Record before and after for the local empty-search request:

- Meilisearch `processingTimeMs` when available;
- API response byte size;
- count of returned People and Advisor values.

These observations are diagnostic evidence, not a fragile timing gate. A large
or unstable regression requires review before acceptance. Do not add a
hard-coded millisecond performance assertion to CI.

The frontend already renders People inside a bounded scroll region and filters
choices locally. Verify that several hundred options remain responsive at the
current corpus size and that no page overflow appears.

## Scope

### Included

- Explicit `faceting` settings in Meilisearch index setup.
- A 1,000-value bounded facet distribution.
- Count-first truncation safety for People and Advisor UUID facets.
- Search schema version 3 and controlled old-cursor rejection.
- Client payload tests and pinned real-Meilisearch coverage beyond 100 distinct
  People.
- Local API and browser proof using Phyo Min Tun.
- Focused status and implementation documentation.
- One coherent implementation commit.

### Excluded

- PostgreSQL schema or migration changes.
- Import data corrections or Person deduplication.
- Project Participation or role changes.
- OpenAPI or response-shape changes.
- Search Document field changes.
- Result ranking, sort, or pagination behavior changes.
- A new People, Advisor, or suggestion endpoint.
- Debounced or per-keystroke network requests.
- Direct frontend access to Meilisearch.
- Renaming People or resolving item 28.
- Changes to filter option ordering, autocomplete matching, Tab behavior, or
  route scroll behavior.
- New dependencies.
- Publication or production deployment.

## Focused verification

### Settings payload tests

- `EnsureIndex` sends `faceting.maxValuesPerFacet` as the named 1,000 bound.
- `sortFacetValuesBy.*` remains `alpha`.
- `sortFacetValuesBy.person_ids` is `count`.
- `sortFacetValuesBy.advisor_person_ids` is `count`.
- The settings task is awaited for both newly created and existing indexes.
- Existing searchable, filterable, sortable, ranking, and typo-tolerance
  settings remain unchanged.

Decode and assert the settings request body structurally. A request-sequence
assertion alone does not prove the nested faceting values.

### Pinned Meilisearch integration

Use the existing pinned Meilisearch integration harness and a uniquely named
disposable index. Add more than 100 distinct valid Person UUID facet values,
including a target value that would be excluded by the old alphabetical UUID
cap.

Prove:

- the distribution returns every seeded value when cardinality is below 1,000;
- the returned count is greater than 100;
- the target Person UUID is present;
- repeated target participation produces the expected count;
- Advisor distribution remains present and correct;
- the disposable index is deleted through bounded cleanup.

The test must fail against the old default settings. Avoid generating thousands
of documents when roughly 120 to 150 distinct values prove the regression.

### Search service and cursor tests

- Facet mapping and PostgreSQL label application still preserve separate People
  and Advisor entries for one Person.
- People count includes every projected participation role while Advisor keeps
  its narrower projection.
- A cursor from schema version 2 is rejected through the controlled invalid
  cursor path after version 3 becomes current.
- Current-version cursors still round-trip.

### Local-stack acceptance

After rebuilding and restarting the local-test API against the existing
disposable stack:

1. Wait for index setup to finish and confirm the active index reports
   `maxValuesPerFacet: 1000` with count sorting for both Person ID facets.
2. Request the empty public Search response.
3. Confirm the People facet contains more than 100 entries and matches the
   current distinct published-Person count from PostgreSQL.
4. Confirm Phyo Min Tun appears in People with count 11 and Advisor with count
   6 for the current 205-Project corpus.
5. Open Search, expand People, enter `Phyo` in the local People filter field,
   and confirm Phyo Min Tun is available.
6. Clear that local field, type `Phyo` in the main search input, and confirm
   separate People and Advisor suggestion rows appear without a request while
   typing.
7. Select each dimension separately and confirm its existing URL and result
   behavior.
8. Confirm the filter panel remains usable at 360 pixels and 200 percent zoom.
9. Record the before-and-after diagnostic response size and processing time.

Do not erase or re-import the local database merely to run this acceptance. The
existing local-test data is sufficient. Never apply this acceptance procedure
to production without a separate deployment request.

### Static and regression checks

- Run focused search client, query, cursor, and document tests.
- Run the pinned Meilisearch integration test.
- Run Go formatting and `go vet` for affected backend packages.
- Run the complete backend suite once because the schema version and index
  settings are shared search infrastructure.
- Run focused frontend SearchPage and suggestion regression tests without
  changing their implementation.
- Run the focused browser acceptance above.
- Run `git diff --check`.
- Inspect the complete diff for excluded frontend implementation, OpenAPI,
  generated-client, database, import, dependency, deployment, and
  infrastructure changes.

## Suggested implementation order

1. Add structural settings-payload assertions that fail under the default
   faceting configuration.
2. Add the bounded explicit faceting settings and schema version 3.
3. Add the greater-than-100 pinned Meilisearch regression.
4. Run focused backend verification.
5. Rebuild only the local-test API and verify the complete public facet and
   Phyo Min Tun behavior.
6. Run the focused frontend and browser regression boundary.
7. Update item 33 to implemented and pending independent review.
8. Commit once and stop.

## Completion criteria

- The current archive's complete People facet is returned.
- Phyo Min Tun is present in both People and Advisor with correct contextual
  counts.
- Person autocomplete locally exposes both dimensions without new requests.
- More than 100 distinct People are proven against pinned Meilisearch.
- Faceting settings are explicit, bounded, tested, and applied to existing and
  future rebuild indexes.
- Search schema version 3 and cursor rejection are verified.
- Current filter semantics, ordering, result ranking, and frontend interactions
  do not regress.
- Performance observations show no material problem at the current corpus size.
- No new endpoint, document field, database change, dependency, publication, or
  deployment is introduced.
- One lowercase past-tense commit with no prefix or co-author trailer contains
  the brief, implementation, tests, and status update.

## Required handoff

Stop after the implementation commit. Report:

- accepted Increment 21 baseline and final commit hash;
- confirmed PostgreSQL, active-index settings, and public facet evidence;
- exact `maxValuesPerFacet` and per-facet sorting settings;
- search schema version and cursor impact;
- existing-index and future-rebuild convergence behavior;
- pinned greater-than-100 regression design and result;
- Phyo Min Tun People and Advisor counts;
- proof that both autocomplete rows appear without typing requests;
- before-and-after response size and Meilisearch processing-time observations;
- focused, backend, frontend regression, static, integration, and browser results;
- any failed, skipped, or residual scaling concern;
- confirmation that every excluded boundary remains unchanged;
- confirmation that item 33 remains pending independent review;
- final `git status --short`.

## References

- Meilisearch faceting settings:
  `https://www.meilisearch.com/docs/reference/api/settings/update-faceting`
- Meilisearch high-cardinality facet guidance:
  `https://www.meilisearch.com/docs/capabilities/filtering_sorting_faceting/how_to/handle_large_facet_cardinality`
- Meilisearch facet performance guidance:
  `https://www.meilisearch.com/docs/capabilities/filtering_sorting_faceting/advanced/optimize_facet_performance`
