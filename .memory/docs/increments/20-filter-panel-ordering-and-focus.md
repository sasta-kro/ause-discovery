# Filter panel ordering, sort toggles, and expansion focus increment

## Assignment

Improve public left-filter scanning and keyboard flow by applying meaningful
default option ordering, adding per-group occurrence/alphabetical sort toggles,
and focusing a group's filter-search input after user-initiated expansion.

This is one bounded frontend-only increment. Start only after Person-name
Search Filter Suggestions Increment 19 is independently accepted. Use the
accepted Increment 19 commit as the implementation baseline and preserve its
People and Advisor autocomplete behavior.

## Required reading

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`, especially Academic Catalog Value, Taxonomy Value,
   Person, and Search Filter Suggestion
4. `.memory/implementation-status/backlog.md`, especially items 28 and 30
5. `.memory/implementation-status/completed.md`, especially Increments 17, 18,
   and 19 after acceptance
6. `.memory/docs/increments/18-search-filter-suggestions.md`
7. `.memory/docs/increments/19-person-filter-suggestions.md`
8. `frontend/src/features/search/filter-controls.tsx`
9. `frontend/src/app-shell.tsx`, limited to public SearchPage choices and facet
   construction
10. `frontend/src/search-page.test.tsx`
11. existing focused filter-control tests
12. `frontend/src/App.module.css`
13. `frontend/src/app/i18n.ts`
14. This brief

## Problem

Filter values currently inherit source order. That order is not consistently
useful for discovery:

- Academic years can appear oldest first even though recent Projects are the
  normal starting point.
- non-ordinal values do not reliably place the most common choices first;
- alphabetical scanning is sometimes preferable but has no per-group control;
- expanding a group still requires a second pointer or keyboard action before
  typing into its filter-search input.

The desired behavior is local presentation only. Counts and choices already
exist in the frontend. Search filter semantics, facet computation, and result
requests must not change.

## Required outcome

- Academic year choices display in descending numeric order by default.
- Semester remains in its explicit academic order: First, Second, Summer.
- Every applicable non-ordinal group defaults to occurrence count descending.
- Equal counts use deterministic alphabetical and stable-key tie-breakers.
- Missing and zero counts appear after positive counts.
- Each applicable group has a small accessible icon button that toggles only
  that group between occurrence-descending and alphabetical order.
- Toggle state is local presentation state and does not enter the URL or search
  API.
- Selected values remain selected and in their sorted position; they are not
  pinned or moved solely because they are selected.
- Text filtering operates over the same choices and preserves the current sort
  mode.
- User-initiated expansion focuses that group's filter-search input.
- Initial default-open rendering does not steal focus.
- Groups without a filter-search input do not receive artificial focus.
- Existing disclosure, selection, counts, filter semantics, Search Filter
  Suggestions, pagination, and result behavior remain unchanged.

## Dimension policy

### Fixed-order dimensions

Academic year:

```text
2026
2025
2024
...
```

Parse valid year keys numerically and sort descending. Invalid or unexpected
values belong after valid years with deterministic label and key ordering.
Academic year has no occurrence/alphabetical toggle in this increment.

Semester:

```text
First semester
Second semester
Summer semester
```

Keep the existing explicit product order. Semester has no sort toggle.

Project File availability:

Keep the existing explicit product order. It is a compact set of boolean
concepts, not a counted catalog list, and has no sort toggle.

### Toggleable non-ordinal dimensions

Apply occurrence/alphabetical toggles to:

- Program;
- Course;
- People;
- Advisor;
- Category;
- Platform;
- Domain;
- Technology.

Major and Topic remain outside the visible public interface and therefore need
no toggle.

## Ordering contract

Create one focused pure ordering operation. Do not scatter comparator logic
through SearchPage and the disclosure component.

### Occurrence-descending mode

Default non-ordinal ordering:

1. choices with positive numeric counts before missing or zero counts;
2. higher counts before lower counts;
3. normalized visible label ascending for equal counts;
4. stable choice value ascending as the final tie-breaker.

Treat a missing count as unavailable rather than inventing a visible number.
For ordering, missing and zero belong in the same trailing tier, with label and
value deciding their deterministic order.

Example:

```text
React       56
Node.js     41
Go           1
Kotlin       0
Uncounted    -
```

Do not pin selected options, active matches, or previously clicked values above
more common values.

### Alphabetical mode

Alphabetical ordering:

1. normalized visible label ascending;
2. stable choice value ascending.

Counts remain visible but do not influence alphabetical order.

Use a predictable case-insensitive comparison suitable for the current English
labels. Do not add locale-collation or natural-sort dependencies. Existing
labels and keys remain unchanged.

### Filtering and sorting sequence

When a group has a local filter query:

1. filter the complete choice set by the existing case-insensitive search;
2. sort the matching choices using the group's active order mode;
3. render the bounded scroll region as today.

Changing the local filter query must not reset the selected order mode. Changing
the order mode must not clear the local query.

Changing server facet counts should re-sort occurrence mode from the latest
props without resetting local query, selection, disclosure state, or
alphabetical mode.

## Sort-toggle interaction

Each toggleable disclosure gets one compact icon button inside the expanded
body, visually associated with the filter-search input. Do not place a button
inside the `summary`, because nested interactive controls would create invalid
and confusing disclosure behavior.

The button toggles only between:

```text
Most common first
Alphabetical
```

Default mode is Most common first on mount. Each group owns independent local
mode state. Closing and reopening the same mounted disclosure preserves its
mode. A full SearchPage remount may return to the default.

The visual control should:

- use a small sort or filter-order icon drawn with existing means, such as a
  tiny inline SVG or CSS shape;
- avoid a new icon package;
- retain a minimum usable pointer target even when the glyph is visually
  compact;
- show visible hover and focus states;
- communicate the current mode and next action through `aria-label` and
  `title`;
- use `aria-pressed` only if its boolean meaning is explicit, for example
  pressed means Alphabetical;
- never rely on the icon alone for assistive technology.

Recommended accessible labels:

```text
People order: Most common first. Change to alphabetical.
People order: Alphabetical. Change to most common first.
```

The exact visible glyph is a local presentation choice. No persistent prose
instruction is required.

## Expansion-focus contract

When a visitor opens a disclosure through its summary by pointer, Enter, or
Space and that disclosure contains a filter-search input:

1. allow the disclosure to open normally;
2. after the input exists and is visible, focus it;
3. preserve any existing local query and place the caret predictably, preferably
   at its end;
4. do not select or clear text automatically.

This applies to Academic year and every toggleable group because those groups
currently provide `searchLabel` and an input.

Do not focus:

- when the disclosure closes;
- during initial rendering;
- merely because `defaultOpen` is true;
- when state restoration or responsive layout leaves a group open;
- when a sort-toggle button is clicked;
- for Semester or Project File availability, which have no local search input.

The Technology disclosure is currently default-open. It must not steal focus
from the page heading or main search input on initial load.

Use a focused implementation guard that distinguishes a user-triggered opening
from initial or programmatic open state. Avoid arbitrary timers. A layout effect,
microtask, or ref callback is acceptable only when tied directly to the open
transition and proven stable in tests.

If browser-native `<details>` event order requires a small controlled-state
adjustment, keep the component local and preserve semantic `details` and
`summary` elements.

## Component boundary

The existing `FacetDisclosure` is the natural boundary. Extend it with explicit
ordering intent rather than inferring behavior from translated labels.

One possible API shape is conceptually:

```text
order: fixed | year-desc | toggleable
defaultOrder: occurrence | alphabetical
```

Exact prop names and helper placement remain coder choices. The caller must make
fixed ordering explicit for Academic year, Semester, and availability, and
toggleable ordering explicit for non-ordinal groups.

Avoid a generic table-sorting or collection framework. A small pure comparator
module is justified because the same ordering repeats across eight facets and
needs focused tests.

## Search Filter Suggestion boundary

Increment 20 changes only left-panel presentation order. It must not change:

- Search Filter Suggestion candidate priority;
- autocomplete matching or suffix detection;
- People and Advisor name suggestions from accepted Increment 19;
- the active suggestion row;
- Tab, Enter, Escape, pointer, or caret behavior;
- result-query timing;
- URL filter order or meaning.

The left panel and autocomplete may therefore display candidates in different
orders for different purposes. Suggestion ranking favors match quality and
dimension priority; left-panel ordering favors counts or alphabetic scanning.
Do not force one ordering abstraction to serve both.

## Responsive and zoom behavior

The search input and sort icon should share a compact local row without reducing
the text field below a usable width. At narrow widths the row may wrap or keep a
fixed compact icon column.

Requirements:

- no horizontal page overflow at 200 percent zoom;
- no clipping of the icon, focus ring, counts, or Add/Remove action;
- long labels continue wrapping safely;
- the choice scroll region remains bounded;
- auto-focused inputs remain visible without forced page jumps beyond normal
  browser focus scrolling;
- the icon has sufficient contrast and a visible keyboard focus indicator.

## Scope

### Included

- Pure year-descending, occurrence-descending, and alphabetical option ordering.
- Explicit fixed versus toggleable dimension policy.
- Per-group local sort mode.
- Small accessible icon toggle in applicable expanded bodies.
- User-initiated expansion focus for groups with local search inputs.
- Existing filter search, selection, counts, scrolling, and responsive layout.
- Focused component, SearchPage, accessibility, zoom, and browser verification.
- One coherent implementation commit after Increment 19 acceptance.

### Excluded

- Backend, OpenAPI, generated-client, Search Document, Meilisearch, PostgreSQL,
  or migration changes.
- Changing facet counts or how Meilisearch calculates them.
- Changing OR-within-dimension or AND-across-dimensions behavior.
- Pinning selected values or zero-result values.
- Persisting per-group sort mode in URL, local storage, database, or account
  preferences.
- New visible Major or Topic filters.
- Resolving People terminology item 28.
- Changes to Search Filter Suggestions or Person-name matching.
- A new icon, component, or styling library.
- Administrator filter redesign.
- Publication or deployment.

## Focused verification

### Pure ordering tests

- Academic years sort numerically newest first, including mixed input order.
- Unexpected nonnumeric year values appear after valid years deterministically.
- Positive counts sort descending.
- Equal positive counts use label then stable value.
- Zero and missing counts follow positive counts.
- Zero and missing ties use label then stable value.
- Alphabetical mode ignores counts and uses label then stable value.
- Input choices are not mutated in place.

### FacetDisclosure tests

- A toggleable group defaults to Most common first.
- The icon button has an understandable accessible name, title, and visible
  focus behavior.
- Clicking the icon switches to alphabetical order and clicking again restores
  occurrence order.
- One group's mode does not affect another group.
- Closing and reopening preserves the mode while mounted.
- Local choice filtering preserves the active order mode.
- Updated count props re-sort occurrence mode without clearing the local query.
- Selected state and Add/Remove actions stay bound to the correct values after
  reordering.
- A pointer-opened searchable disclosure focuses its local input.
- A keyboard-opened searchable disclosure focuses its local input.
- Closing does not focus the input.
- Initial `defaultOpen` does not steal focus.
- A group without a search input does not receive artificial focus.
- Activating the sort button does not close the disclosure or move focus to the
  filter input.

### SearchPage tests

- Academic year renders newest first.
- Semester remains First, Second, Summer.
- Program, Course, People, Advisor, Category, Platform, Domain, and Technology
  default to count descending.
- Alphabetical toggles operate independently on representative academic,
  Person, and classification groups.
- Filter selection sends the same URL state and API query before and after
  reordering.
- Existing People and Advisor name suggestions remain available and unchanged.
- Cursor history still resets only when an applied filter changes.
- No result request occurs from local option sorting or local option searching.

### Browser smoke

1. Open Search with representative facet counts.
2. Expand Academic year and confirm newest year appears first and its local
   search input receives focus.
3. Expand Semester and confirm First, Second, Summer with no sort icon and no
   artificial focus target.
4. Expand People and confirm its search input receives focus and the highest
   occurrence choices appear first.
5. Activate the sort icon and confirm alphabetical order, accessible mode text,
   persistent disclosure state, and no result request.
6. Close and reopen People and confirm alphabetical mode persists while the
   page remains mounted.
7. Repeat the toggle in Technology and confirm People remains unaffected.
8. Open the default-open Technology group on a fresh page and confirm it does
   not steal initial focus.
9. Select and remove values after reordering and confirm chips, counts, and API
   parameters remain correct.
10. Repeat with keyboard-only interaction, 200 percent zoom, and a narrow
    viewport.

The handoff must state whether the browser smoke ran. Do not infer it from
component tests.

### Static and build checks

- Run focused filter-control and SearchPage tests.
- Run accepted Increment 19 suggestion tests as a regression boundary.
- Run the complete frontend component suite once.
- Run ESLint and `tsc -b` once.
- Run the default-subpath production build once.
- Run a focused existing Chromium test if extended without new infrastructure.
- Run `git diff --check`.
- Inspect the complete diff for excluded backend, generated, suggestion,
  dependency, and infrastructure changes.

No Go, database, Meilisearch, storage, import, or full-stack verification is
required.

## Suggested implementation order

1. Add pure non-mutating option comparators and tests.
2. Extend `FacetDisclosure` with explicit ordering policy.
3. Add the local sort icon and independent group mode state.
4. Add guarded user-expansion focus behavior.
5. Mark year, ordinal, and toggleable callers explicitly in SearchPage.
6. Add focused component and rendered interaction tests.
7. Run the browser smoke and correct only this increment's interaction defects.
8. Update backlog item 30 to implemented and pending independent review.
9. Commit once and stop.

## Completion criteria

- Academic year is newest first.
- Semester retains defined academic order.
- Non-ordinal filters default to count descending with deterministic ties.
- Each applicable group can toggle independently to alphabetical and back.
- The small icon is accessible, keyboard operable, and requires no library.
- User expansion focuses an existing group search input without stealing focus
  on initial render.
- Local sorting and searching issue no result request.
- Filter semantics, state, chips, counts, autocomplete, and paging do not
  regress.
- 200 percent zoom and narrow layouts remain usable.
- Focused, full frontend, static, build, and browser checks pass.
- No backend, contract, generated, search-engine, database, dependency,
  publication, or deployment change occurs.
- One lowercase past-tense commit with no prefix or co-author trailer contains
  the implementation and status update.

## Required handoff

Stop after the implementation commit. Report:

- accepted Increment 19 baseline and final commit hash;
- ordering helper and exact comparator behavior;
- fixed and toggleable dimension policy;
- icon implementation and accessible mode communication;
- independent local mode behavior;
- user-initiated expansion focus guard;
- proof that initial default-open state does not steal focus;
- proof that sorting and local filtering issue no result request;
- focused, regression, full frontend, static, build, Chromium, and browser-smoke
  results;
- any skipped check or residual browser risk;
- confirmation that Search Filter Suggestions and every excluded boundary remain
  unchanged;
- confirmation that item 30 remains pending independent review;
- final `git status --short`.
