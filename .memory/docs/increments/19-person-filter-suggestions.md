# Person-name Search Filter Suggestions increment

## Assignment

Extend the accepted Search Filter Suggestions interaction to the existing
People and Advisor facets, then commit the frontend-only increment.

This increment reuses the current public search response, facet labels, shared
filter application, and autocomplete component. It does not introduce a Person
lookup endpoint, live result requests while typing, a search-document change,
or new public terminology.

Start from commit `68150b5`, which contains the accepted Increment 18 baseline
plus its maintainer-directed visual correction. Include the planning files for
Increments 19 and 20 in the implementation commit. Preserve unrelated work.

## Required reading

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`, especially Person, Project Participation, and Search
   Filter Suggestion
4. `.memory/implementation-status/README.md`
5. `.memory/implementation-status/backlog.md`, especially items 28, 29, and 30
6. `.memory/implementation-status/completed.md`, especially Increment 18
7. `.memory/docs/increments/18-search-filter-suggestions.md`
8. `frontend/src/features/search/suggestions.ts`
9. `frontend/src/features/search/suggestion-input.tsx`
10. `frontend/src/features/search/filter-controls.tsx`
11. `frontend/src/app-shell.tsx`, limited to SearchPage choice construction,
    filter mutation, and active-filter chips
12. `frontend/src/features/search/suggestions.test.ts`
13. `frontend/src/features/search/suggestion-input.test.tsx`
14. `frontend/src/search-page.test.tsx`
15. `frontend/src/App.module.css`
16. This brief

## Existing terminology boundary

The current People facet uses `person_id` and includes every indexed Project
Participation role: student, advisor, co-advisor, and committee member. The
current Advisor facet uses `advisor_id` and is narrower.

Increment 19 must preserve those exact meanings and labels:

- `People: Phyo Min Tun` applies `person_id=<uuid>`;
- `Advisor: Phyo Min Tun` applies `advisor_id=<uuid>`.

One Person can legitimately appear in both suggestion dimensions. That is a
collision to display, not evidence that the rows should be merged. Item 28 may
later rename People or add a student-only facet, but that unresolved product
decision does not block this extension and must not be decided here.

Do not use Student, Professor, Lecturer, Staff, Faculty, or Participant as a
replacement public label in this increment. No Person role may be inferred from
a name.

## Required outcome

- Typing a case-insensitive Person-name prefix offers matching People and
  Advisor filters from already-loaded facet values.
- `Phyo`, `phyo`, and `Phyo Mi` can suggest `Phyo Min Tun` when that Person is
  present in the applicable loaded facet.
- A matched Person-name prefix remains eligible across a trailing space so the
  next name segment can be typed.
- Existing non-Person suggestions continue closing on trailing whitespace under
  the accepted conservative matching contract.
- People and Advisor collisions remain distinct and dimension-labelled.
- Tab accepts only the highlighted Person suggestion and applies the exact
  corresponding `person_id` or `advisor_id` value.
- Already-selected values are excluded independently by dimension.
- Removing an existing People or Advisor filter makes that dimension's
  suggestion eligible again.
- One acceptance removes only the matched trailing name fragment, preserves all
  preceding free text, resets pagination, and triggers one search request.
- Typing alone triggers no result request and no new data request.
- Existing mouse, arrow, Tab, Enter, Escape, caret, composition, and focus
  behavior remains intact.
- The active row retains the small contextual Tab keycap from Increment 18.
- No persistent instructional paragraph is restored.

## Suggestion sources

Use the existing Search response facets already mapped into SearchPage:

| Suggestion dimension | Source | Search state field | Cardinality |
| --- | --- | --- | --- |
| People | `facets.people` through `peopleChoices` | `person_id` | Multiple |
| Advisor | `facets.advisors` through `advisorChoices` | `advisor_id` | Multiple |

Each facet choice already contains:

- the canonical Person ID as its value;
- the public display name as its label;
- the current result occurrence count.

Use only the display name as the matching alias. A UUID must never become a
visible or matching alias. Do not add Student ID matching to suggestions.

Facet data can change after an applied search changes. Suggestion definitions
must refresh from the latest successful Search response while existing
placeholder data keeps the current interaction stable during a short refetch.
When a facet is absent or unavailable, omit that suggestion dimension without
breaking free-text search.

Do not make a second request for a global Person list. The suggestion universe
is intentionally the same contextual set exposed by the current left filter
panel.

## Type and shared-mutation extension

Extend the focused suggestion field model to include:

```text
person_id
advisor_id
```

Both are multiple-value fields. Route their additions through the same
`applyFilterValue` operation used by other suggestion-enabled array filters.
The existing left-panel People and Advisor toggles must also continue using the
shared add behavior while retaining their current remove behavior.

Avoid field-specific branching inside the combobox component. Person-specific
matching allowances belong in suggestion metadata or the pure matching layer,
not in DOM event handling.

One possible narrow extension is a suggestion kind or matching policy such as:

```text
standard controlled value
person name
academic year
```

Exact naming remains a coder choice. The goal is to express the trailing-space
exception without checking `person_id` and `advisor_id` throughout unrelated
functions.

## Person-name matching contract

### Case and prefix behavior

Person display names use the existing case-insensitive normalization. Matching
remains exact or prefix only. No typo correction, edit distance, phonetic
matching, token reordering, transliteration, nickname inference, or arbitrary
substring matching is introduced.

Examples:

```text
Facet label: Phyo Min Tun

phyo      -> match
Phyo      -> match
PhYo Mi   -> match
Phyo Min  -> match
Min Tun   -> no match
Pyo       -> no match
Phyo Tin  -> no match
```

The existing three-character minimum applies. Exact shorter-value exceptions
remain available to other controlled dimensions but should not create special
Person-name abbreviations.

### Spaces within a name prefix

Person names are multiword controlled values. A trailing space after a matched
leading name segment is an incomplete Person-name prefix rather than unrelated
text.

Required behavior:

```text
Input: Phyo
Suggestion: People: Phyo Min Tun

Input: Phyo<space>
Suggestion remains: People: Phyo Min Tun

Input: Phyo Mi
Suggestion remains: People: Phyo Min Tun

Input: Phyo Min Tun
Suggestion is exact
```

The matching layer must preserve enough information to distinguish a trailing
space from an empty normalized suffix. A Person suggestion may remain open only
when the normalized text before the trailing space is already an exact prefix
of the Person display name at a name-segment boundary.

This exception does not apply globally:

```text
game<space>       -> no Category suggestion
2025<space>       -> no Academic year suggestion
unmatched<space>  -> no Person suggestion
```

If another unrelated word follows, the nearest-suffix rules still apply. Do not
scan backward past that word to recover an earlier Person match.

### Nearest suffix and free text

Person suggestions follow the accepted nearest-suffix contract.

```text
Input: archive Phyo Mi
Tab: select People: Phyo Min Tun
Remaining query: archive
```

```text
Input: archive Phyo Mi report
Active trailing term: report
Result: no backward Person suggestion
```

One Tab action converts exactly one suggestion. It must not apply both People
and Advisor entries for the same Person.

### Collisions

When one Person appears in both facets, show both rows:

```text
Phyo Min Tun    People
Phyo Min Tun    Advisor
```

The existing deterministic dimension priority may place People before Advisor,
matching the left-panel order. Arrow keys can move to Advisor. The visible
dimension and accessible option name must make the distinction explicit.

Exact matches still rank before prefixes. Longer consumed name prefixes still
rank before shorter suffix matches. Display-name and stable-ID tie-breakers
must keep ordering deterministic.

### Selected-value exclusion

Selection is dimension-specific:

- a Person ID already in `person_id` suppresses only its People suggestion;
- the same ID may still appear as an Advisor suggestion when not selected in
  `advisor_id`;
- an ID selected in both dimensions produces neither suggestion;
- removing one dimension restores only that suggestion.

Do not hide every suggestion for a Person merely because one role-specific
filter is selected.

## Interaction preservation

No keyboard contract from Increment 18 changes:

- the first matching row is highlighted automatically;
- ArrowDown and ArrowUp use the existing clamped movement;
- plain Tab accepts one highlighted suggestion and keeps input focus;
- Shift+Tab remains ordinary backward navigation;
- Enter submits free text and never accepts a suggestion;
- first Escape suppresses the current popup while keeping focus;
- text or caret change after dismissal allows suggestions to return;
- second Escape for unchanged dismissed input removes input focus;
- mouse selection applies one row without a blur race;
- input method composition blocks suggestion keyboard actions;
- moving the caret away from the end closes the popup.

Person-name support must not restore the removed persistent instruction text.
Only the active suggestion's small Tab keycap remains.

## Search state transition

Person suggestion acceptance must use one atomic SearchPage transition:

1. remove the matched trailing range;
2. preserve and trim only boundary whitespace from remaining free text;
3. set `q` to the remainder or undefined;
4. append the Person ID to the selected `person_id` or `advisor_id` array;
5. preserve every other filter and explicit or implicit sort state;
6. clear the cursor and local previous-page history;
7. issue the normal search request once.

Typing `Phyo Mi` alone must not call the API. Accepting the suggestion calls the
existing search path once. No prefetch, debounced People request, or live result
query is allowed.

## Visual and accessibility behavior

Reuse the accepted suggestion list presentation. Person names may be longer than
classification labels, so rows must:

- allow the display name to use available width;
- keep the dimension and Tab hint readable without forcing horizontal overflow;
- wrap safely at 200 percent zoom and narrow widths;
- retain a usable pointer target;
- retain visible active-row treatment and stable listbox geometry.

The option's accessible name must include both display name and dimension. UUIDs
and occurrence counts are not required in the suggestion row.

Keep the existing combobox, listbox, option, expanded, controls,
active-descendant, and selected-state relationships. No persistent
`aria-describedby` instruction or visible help paragraph is reintroduced.

## Scope

### Included

- `person_id` and `advisor_id` suggestion fields.
- People and Advisor suggestion definitions from existing facets.
- Person display-name prefix matching.
- Person-only matched trailing-space continuation.
- Dimension-specific collision and exclusion behavior.
- Shared filter application for People and Advisor additions.
- Existing combobox integration, Tab keycap, URL state, chips, sidebar, cursor
  reset, and result request.
- Focused pure, component, SearchPage, responsive, and browser verification.
- One coherent implementation commit.

### Excluded

- Backend, OpenAPI, generated-client, Search Document, Meilisearch, PostgreSQL,
  or migration changes.
- A new Person search or suggestion endpoint.
- Live result or Person requests while typing.
- Student ID suggestion matching.
- Person role inference or merging People and Advisor suggestions.
- Renaming People, adding a student-only facet, or resolving item 28.
- Fuzzy names, nicknames, transliteration, phonetic matching, or token
  reordering.
- Project title, Artifact, availability, sort, Major, or Topic suggestions.
- Filter-panel ordering, sort toggles, or expansion focus from Increment 20.
- Restoring the persistent suggestion-help paragraph.
- New dependencies, publication, or deployment.

## Focused verification

### Pure suggestion tests

- People and Advisor definitions contain display-name aliases but not UUID
  aliases.
- `Phyo`, mixed-case `phyo`, `Phyo Mi`, and the exact full name match
  `Phyo Min Tun`.
- `Min Tun`, `Pyo`, and a misspelled segment do not match.
- `Phyo ` keeps Person suggestions eligible.
- `game ` and `2025 ` retain their existing no-suggestion behavior.
- Unmatched trailing whitespace produces no Person suggestion.
- `archive Phyo Mi` consumes only the Person-name suffix and preserves
  `archive`.
- `archive Phyo Mi report` does not scan backward to the Person name.
- One Person in People and Advisor creates two deterministic labelled rows.
- Selecting People excludes only People; selecting Advisor excludes only
  Advisor; selecting both excludes both.
- Removing a filter restores that dimension's suggestion.
- Six-result bounding and existing controlled-value ranking remain intact.

### Component and SearchPage tests

- Typing Person prefixes opens suggestions without an API request.
- Continued typing across `Phyo ` to `Phyo Mi` keeps or refreshes the popup
  without an intermediate result request.
- Arrow movement and Tab choose the intended People or Advisor collision.
- Tab acceptance preserves preceding free text and keeps input focus.
- Search state receives the exact Person UUID in the correct array.
- One acceptance causes one result request with cursor reset.
- Selected-filter chips and left-panel selected state reflect acceptance.
- Existing People and Advisor click toggles, chip removal, and cursor reset do
  not regress.
- Enter, Shift+Tab, both Escape stages, pointer acceptance, caret movement, and
  composition behavior remain unchanged.
- The persistent helper paragraph remains absent.
- Existing Academic and classification suggestion tests continue passing.

### Browser smoke

1. Open Search with the full public facet response loaded.
2. Type `Phyo`; confirm People or Advisor suggestions appear locally without a
   result request.
3. Type one space; confirm matching full-name suggestions remain available.
4. Continue to `Phyo Mi`; confirm `Phyo Min Tun` remains suggested.
5. When both dimensions exist, use ArrowDown to highlight the intended one and
   press Tab.
6. Confirm exactly one People or Advisor filter chip appears, the corresponding
   left facet is selected, the input stays focused, and one search runs.
7. Remove the chip and confirm the matching Person suggestion becomes eligible
   again.
8. Type `archive Phyo Mi`, accept, and confirm `archive` remains as submitted
   free text.
9. Type an unrelated trailing word and confirm no backward Person suggestion.
10. Repeat with mixed case, 200 percent zoom, and a narrow viewport.

The handoff must state whether this browser smoke ran. Do not claim it from
component tests alone.

### Static and build checks

- Run focused suggestions, suggestion-input, and SearchPage tests.
- Run the complete frontend component suite once.
- Run ESLint and `tsc -b` once.
- Run the default-subpath production build once.
- Run the existing focused Chromium suggestion tests if the harness remains
  available.
- Run `git diff --check`.
- Inspect the full diff for excluded backend, generated, dependency,
  Increment 20, and infrastructure changes.

No Go, database, Meilisearch, storage, import, or full-stack test is required.

## Suggested implementation order

1. Extend suggestion types and shared filter application for People and
   Advisor.
2. Build both sources from existing SearchPage facet choices.
3. Add a narrow Person-name matching policy for trailing-space continuation.
4. Add pure matching and exclusion tests.
5. Add component collision, keyboard, and state-transition tests.
6. Run the browser smoke and correct only Person-suggestion defects.
7. Update backlog item 29 to implemented and pending independent review.
8. Keep item 30 sequenced and unimplemented.
9. Commit once and stop.

## Completion criteria

- People and Advisor names autocomplete from existing facets.
- Prefix matching is case-insensitive, conservative, and frontend-only.
- Multiword prefixes remain usable across a matched trailing space.
- One Person can appear distinctly in both dimensions.
- Selected values are excluded per dimension.
- Tab applies exactly one correct Person filter while preserving free text.
- Typing performs no result or Person request.
- Existing suggestion and filter interactions do not regress.
- No persistent helper paragraph returns.
- Focused, full frontend, static, build, and browser checks pass.
- No backend, contract, generated, search-engine, schema, dependency,
  Increment 20, publication, or deployment change occurs.
- One lowercase past-tense commit with no prefix or co-author trailer contains
  the planning files and implementation.

## Required handoff

Stop after the implementation commit. Report:

- starting baseline and final commit hash;
- suggestion type and matching-policy changes;
- People and Advisor source construction;
- exact `Phyo`, `Phyo `, and `Phyo Mi` behavior;
- collision and per-dimension exclusion behavior;
- shared filter mutation and atomic result-request behavior;
- proof that typing issues no request;
- focused, full frontend, static, build, Chromium, and browser-smoke results;
- any skipped check or residual browser risk;
- confirmation that item 30 and every excluded boundary remain unchanged;
- confirmation that item 29 remains pending independent review;
- final `git status --short`.
