# Newest-first academic search ordering increment

## Assignment

Implement and commit deterministic academic ordering for public Project search.

This is one bounded search increment spanning Meilisearch settings, backend
query resolution, the public search contract description, frontend URL state,
and focused verification. Keep PostgreSQL canonical, keep Meilisearch
rebuildable derived state, and keep the existing Project search-result shape,
filters, Logo loading, and pagination controls.

Start only after Increment 16 is independently accepted through documentation
commit `0554a6a`, with implementation corrected through `bfb1cff`. Preserve
unrelated working-tree changes and existing behavior unless this brief
explicitly changes it.

## Required reading before coding

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`, especially Project, Search Document, and Search
   Synchronization
4. `.memory/architectural_decision_logs.md`, especially AD-001 and AD-003
5. `.memory/implementation-status/backlog.md`, especially item 22
6. `.memory/implementation-status/completed.md`, limited to accepted search and
   Increment 16 boundaries
7. `.memory/docs/ause-discovery_specification_v1.md`, sections 7, 20, 21, and 57
8. `.memory/docs/ause-discovery_implementation_specification_v1.md`, sections
   7.3, 8, 12.3, and 15
9. `backend/internal/search/client.go`
10. `backend/internal/search/document.go`
11. `backend/internal/search/query.go` and its tests
12. `backend/internal/platform/httpserver/api.go`, limited to public search
13. `api/openapi.yaml`, limited to `GET /search` and `SearchSort`
14. `frontend/src/features/search/state.ts` and its tests
15. `frontend/src/app-shell.tsx`, limited to `SearchPage`
16. `frontend/src/search-page.test.tsx`
17. This brief

## Problem

An empty search currently relies on Meilisearch's ordinary placeholder-result
order. The `relevance` state is also shown when no query terms exist, even
though there is no textual relevance to calculate.

The explicit `newest` and `oldest` choices currently sort only by
`published_at`. That timestamp records when a Project entered AUSE Discovery,
not when the historical Project occurred. A newly imported 2016 Project can
therefore appear newer than a 2025 Project.

The current query-time `sort` rule also sits before `exactness` in the ranking
rules. Adding a chronological query sort to ordinary text search would let the
tie-breaker displace part of lexical relevance. Existing sort lists also omit a
unique final key, so equal values do not define deterministic offset pages.

## Required outcome

- Empty discovery with no explicit sort shows the newest academic Projects
  first.
- Academic chronology uses `academic_year` first and the existing internal
  semester order second.
- Within one academic year, newest order is Summer semester, Second semester,
  then First semester. Oldest order is the reverse.
- Ordinary nonempty text search in relevance mode keeps every accepted lexical
  ranking rule ahead of academic chronology.
- Relevance ties use newest academic chronology.
- An explicit Newest, Oldest, or Title selection is authoritative across the
  matching and filtered result set.
- Explicit Newest and Oldest use academic chronology, never publication time as
  the primary date.
- Every ordering ends with stable deterministic tie-breakers, including Project
  ID.
- The search select displays Newest first for an empty implicit search and Most
  relevant for an implicit text search.
- Explicit sort selections remain shareable URL state and survive filtering and
  pagination.
- A sort, query, filter, or ordering-schema change invalidates an incompatible
  cursor.
- Existing result fields, facets, filters, exact seven-digit identifier lookup,
  highlights, Logo loading cohorts, and public routes remain unchanged.

## Product behavior

### Implicit ordering

When the URL has no explicit `sort` parameter:

| Query state | Effective behavior | Search select |
| --- | --- | --- |
| Empty or whitespace only | Academic newest | Newest first |
| Nonempty text | Lexical relevance, then academic newest | Most relevant |

The implicit state should remain compact in the URL. Submitting text from an
implicit empty search changes the displayed selection to Most relevant without
persisting an artificial `sort=newest`. Clearing the text returns the displayed
selection to Newest first.

### Explicit ordering

An explicit selection is authoritative:

- `newest`: `academic_year DESC`, `semester_order DESC`, stable tie-breakers;
- `oldest`: `academic_year ASC`, `semester_order ASC`, stable tie-breakers;
- `title`: normalized title ascending, stable tie-breakers;
- `relevance`: lexical relevance followed by academic newest; an empty
  relevance request behaves as academic newest because no text ranking exists.

Text and filters still determine which Projects are eligible. Explicit ordering
controls the order of those eligible Projects.

### Deterministic tie-breakers

Use normalized title and Project ID after academic year and semester for
academic ordering. Publication time is intentionally excluded from academic
chronology because imports and republication are administrative events.

Recommended order lists:

```text
newest: academic_year:desc, semester_order:desc, title_sort:asc, id:asc
oldest: academic_year:asc, semester_order:asc, title_sort:asc, id:asc
title:  title_sort:asc, academic_year:desc, semester_order:desc, id:asc
```

Project ID is the unique final key that makes unchanged-index pagination
deterministic.

## Meilisearch ranking decision

Do not pass academic chronology as query-time sort for implicit relevance mode.
Instead, configure academic-newest custom ranking rules after all lexical
relevance rules. Placeholder searches have no active textual ranking criteria,
so those custom rules naturally become their default order.

Keep the built-in query-time `sort` rule separate and high enough to make an
explicit user selection authoritative. A suitable intent is equivalent to:

```text
sort
words
typo
proximity
attribute
exactness
academic_year:desc
semester_order:desc
title_sort:asc
id:asc
```

The exact accepted legacy names supported by the pinned Meilisearch version may
remain. The important contract is:

1. no query-time sort is sent for implicit or explicit relevance mode;
2. every lexical rule, including exactness, precedes default academic
   tie-breakers;
3. explicit query-time sort is authoritative;
4. default academic ordering ends with a unique stable key.

Add `id` to sortable attributes. Existing sortable attributes may remain when
still supported by the public sort contract.

Increment the source-controlled search schema version because index settings
and ranking semantics change. No PostgreSQL migration is required. Existing
Search Documents already contain academic year, semester order, normalized
title, and Project ID.

## Backend query and cursor behavior

Centralize ordering resolution in the search package. Avoid duplicating sort
lists in handlers or clients.

The resolved ordering must distinguish at least:

- implicit empty academic newest;
- relevance followed by academic newest;
- explicit academic newest;
- explicit academic oldest;
- explicit title order.

Normalize text consistently before resolving the order. Whitespace-only text is
empty.

Cursor validation must bind to:

- the normalized query and filters;
- the resolved ordering rather than only the raw sort token;
- the source-controlled search schema version;
- the existing effective limit and other query state already covered by the
  query hash.

A cursor created before this ordering schema must be rejected as invalid rather
than reused against the new ranking behavior. Existing opaque cursor response
and request shapes remain unchanged.

Offset pagination remains accepted for this collection size. The unique final
sort key prevents duplicates or omissions caused by ambiguous order while the
index is unchanged. Concurrent index mutation can still change offset pages and
is outside this increment.

The exact seven-digit identifier path must preserve its current behavior:
attempt the exact Student ID or Reference Code filter first, then fall back to
ordinary text search only when the exact attempt returns no hits. Ordering must
be consistent for both attempts.

## Frontend URL-state behavior

Preserve the difference between an omitted sort and an explicit sort selection.
Do not eagerly normalize every missing or invalid sort to `relevance` in stored
state, because that loses the contextual empty-search default.

Provide one small resolver for the displayed or effective selection:

```text
omitted sort + empty query    -> newest
omitted sort + nonempty query -> relevance
explicit valid sort           -> explicit value
```

Invalid sort values normalize to the implicit state. Existing URL rules remain:

- changing sort resets the cursor and local previous-page history;
- every explicit valid sort value, including `relevance`, serializes into the
  URL;
- implicit state stays omitted;
- browser history restores query, explicit sort, and cursor state;
- filters preserve the current explicit or implicit sort intent.

Keep the four existing visible options and their current labels. No new sorting
control or visual redesign is required.

## OpenAPI contract clarification

Keep the existing `SearchSort` enum values and generated transport shape:

```text
relevance, newest, oldest, title
```

Add a concise description that records the stable semantics:

- relevance uses lexical ranking with academic-newest tie-breakers;
- newest and oldest use academic year and semester;
- title uses normalized title;
- the omitted empty-search default is newest academic order.

Regenerate tracked outputs only if the description changes generated comments.
No new parameter or response field is permitted.

## Scope

### Included

- Search schema-version increment.
- Meilisearch ranking-rule and sortable-attribute changes.
- Central backend ordering resolution.
- Academic Newest and Oldest query sort lists.
- Deterministic Title ordering.
- Cursor binding to resolved ordering and schema version.
- OpenAPI description clarification without shape changes.
- Contextual frontend implicit sort display and URL state.
- Focused backend, HTTP, frontend, and pinned-Meilisearch verification.
- Backlog status updated to implemented and pending independent review without
  claiming acceptance.
- One coherent implementation commit.

### Excluded

- PostgreSQL schema changes or migrations.
- Search Document field additions.
- Changes to search-result response fields.
- Classification filter semantics from backlog item 23.
- Search analytics, recommendations, semantic search, suggestions, or body-text
  indexing.
- Changes to Project Logo eager and lazy cohorts.
- Administrator list ordering or Person-page Project ordering.
- Meilisearch replacement or canonical-data changes.
- Deployment, publication, production reindex, or production rebuild.
- Broad frontend redesign.

## Focused verification

### Backend unit and service tests

- Empty or whitespace-only implicit search resolves to academic newest.
- Nonempty implicit search resolves to relevance followed by academic newest.
- Explicit relevance preserves lexical-first behavior.
- Explicit Newest, Oldest, and Title produce the exact accepted deterministic
  sort lists.
- Semester boundaries prove Summer, Second, First for newest and the reverse
  for oldest.
- Equal academic periods use normalized title and Project ID deterministically.
- Search settings contain the accepted ranking-rule order and `id` as sortable.
- The exact-identifier attempt and fallback receive consistent ordering.
- Cursor round-trip succeeds with identical resolved ordering.
- Cursors fail after query, filter, explicit sort, effective ordering, or search
  schema-version changes.
- Semantically equivalent omitted and explicit relevance states behave
  consistently where their resolved ordering is the same.

### HTTP boundary tests

- Empty `GET /search` returns the ordered Project sequence from the search
  service boundary.
- Valid explicit sort values reach the correct resolved order.
- Invalid sort remains a controlled `400` validation response.
- Two pages preserve order without duplicate Project IDs.
- The public response contract and cursor shape remain unchanged.

### Frontend tests

- An empty implicit search displays Newest first without serializing an
  unnecessary sort parameter.
- An implicit text search displays Most relevant.
- Submitting and clearing text switches only the implicit displayed default.
- Explicit Newest, Oldest, Title, and Relevance selections serialize and remain
  stable as applicable.
- A sort change clears the cursor and previous-page history.
- Invalid URL sort state returns to the contextual implicit default.
- Search requests carry explicit sort only when explicitly selected.
- Existing paging, filters, search submission, and Logo priority tests pass.

### Pinned Meilisearch integration

Add one focused integration test against the pinned Meilisearch service with a
unique disposable index. It must prove actual engine behavior rather than only
mock payloads:

- placeholder search returns academic newest across year and semester
  boundaries;
- relevance mode ranks a stronger older textual match above a weaker newer
  match;
- equally relevant matches use academic newest order;
- explicit Oldest reverses academic chronology;
- explicit Title uses deterministic normalized title order;
- two bounded pages contain no duplicate IDs.

The test must clean up its unique index and run in the repository's existing
integration path. Wire the pinned Meilisearch service into CI only as needed to
keep this test non-optional. Do not add a new search library or test framework.

### Static and build checks

- Run focused search package tests.
- Run the focused HTTP boundary test.
- Run the pinned Meilisearch integration test once.
- Run frontend search-state and SearchPage tests.
- Run Go formatting and `go vet` for the affected backend.
- Run frontend ESLint and `tsc -b` once.
- Run the default-subpath production build once.
- Run generated-code drift verification when OpenAPI output changes.
- Run `git diff --check`.
- Inspect the complete diff for excluded changes.

Do not run unrelated storage, import, B2, or full product acceptance checks
unless a focused failure exposes a cross-system risk.

## Suggested implementation order

1. Define resolved ordering and deterministic sort lists in the search package.
2. Update ranking settings, sortable attributes, and search schema version.
3. Bind cursor validation to resolved ordering and schema version.
4. Clarify the OpenAPI description and regenerate only if required.
5. Preserve implicit versus explicit frontend sort state and contextual display.
6. Add focused service, HTTP, frontend, and pinned-Meilisearch tests.
7. Run the bounded verification once.
8. Keep backlog item 22 active as implemented and pending independent review.
   Do not move it to completed status or claim acceptance.
9. Commit the planning files and implementation once, then stop.

## Completion criteria

- Empty discovery is academic-newest by default.
- Newest and Oldest reflect Project academic periods rather than import or
  publication time.
- Relevance mode preserves the complete lexical ranking pipeline before
  academic tie-breakers.
- Explicit user sorting is authoritative.
- Every order has a unique stable final key.
- Cursor validation covers resolved ordering and search schema version.
- Frontend implicit and explicit sort intent is accurate and shareable.
- Existing search, filter, exact-identifier, paging, highlighting, and Logo
  behavior remains intact.
- Actual pinned-Meilisearch behavior is proven.
- No PostgreSQL migration, new Search Document field, response-shape change,
  unrelated search feature, publication, or deployment occurs.
- One coherent main-repository commit uses a lowercase past-tense message with
  no prefix and no co-author trailer.

## Required handoff

Stop after the implementation commit. Report:

- accepted Increment 16 baseline and final commit hash;
- resolved ordering modes and exact sort lists;
- ranking-rule order and why relevance mode does not send query-time sort;
- search schema-version and cursor changes;
- frontend implicit and explicit URL-state behavior;
- OpenAPI and generated-output impact;
- focused backend, HTTP, frontend, and pinned-Meilisearch results;
- any failed or skipped check;
- confirmation that PostgreSQL schema, Search Document fields, result shapes,
  classification semantics, storage, publication, and deployment did not
  change;
- confirmation that backlog item 22 remains pending independent review;
- final `git status --short`.

## Research basis

- Meilisearch documents ranking as ordered bucket sorting, where later rules
  break ties left by earlier rules:
  <https://www.meilisearch.com/docs/capabilities/full_text_search/advanced/ranking_pipeline>
- Meilisearch documents query-time `sort` as active only when a sort parameter
  is present and explains that its ranking-rule position controls whether user
  sorting or relevance dominates:
  <https://www.meilisearch.com/docs/capabilities/filtering_sorting_faceting/how_to/sort_results>
- Meilisearch documents placeholder searches as having no query terms, leaving
  active ranking and custom rules to order all documents:
  <https://www.meilisearch.com/docs/capabilities/full_text_search/getting_started/placeholder_search>
