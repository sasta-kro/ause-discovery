# Search Filter Suggestions increment

## Post-acceptance visual correction

Recorded 2026-09-15: remove the persistent instructional paragraph beneath the
search input and align the input, sort select, and Search button to one shared
height. Retain the small contextual Tab keycap inside the active suggestion.
The maintainer determined that the persistent keyboard and filter-combination
copy was visually intrusive and unnecessary in this interface.

## Assignment

Implement and commit conservative Search Filter Suggestions in the public
search input.

This is one bounded frontend-only increment. It adds an accessible autocomplete
surface that converts one recognized trailing query fragment into one existing
structured filter. It does not add live result search, a backend endpoint, an
OpenAPI change, a search-engine change, or a fuzzy-search dependency.

Start only after Increment 17 is independently accepted through correction
commit `5f4bac7`. Preserve all existing public search, ordering, cursor, filter,
Logo, and result-rendering behavior unless this brief explicitly changes it.

## Required reading before coding

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`, especially Search Filter Suggestion, Taxonomy Value,
   Academic Catalog Value, and Search Document
4. `.memory/implementation-status/backlog.md`, especially item 25 and the
   filter-combination explanation correction
5. `.memory/implementation-status/completed.md`, especially accepted Increment
   17 behavior
6. `.memory/docs/increments/17-newest-first-academic-search-ordering.md`
7. `.memory/docs/ause-discovery_specification_v1.md`, sections 7 and 8
8. `.memory/docs/ause-discovery_implementation_specification_v1.md`, sections
   12.3 and 15
9. `frontend/src/app-shell.tsx`, limited to `SearchPage` and its filter
   definitions and mutation paths
10. `frontend/src/features/search/state.ts`
11. `frontend/src/features/search/filter-controls.tsx`
12. `frontend/src/search-page.test.tsx`
13. `frontend/src/features/search/state.test.ts`
14. `frontend/src/App.module.css`
15. `frontend/src/app/i18n.ts`
16. This brief

## Product intent

The search input currently accepts free text only. Academic years,
classifications, Programs, Courses, and Semesters must be selected separately
from the left filter panel. A visitor who types `2025` expects a useful path to
the Academic year filter, but ordinary text search can only match indexed text
containing that number.

Search Filter Suggestions should recognize conservative, high-confidence
controlled-value prefixes while text is composed. A suggestion such as
`Academic year: 2025` or `Category: Game` appears beneath the input. Selecting
it applies exactly the same structured search state as the equivalent left
filter, removes only the recognized trailing fragment, preserves all remaining
free text, and runs one filtered search.

This interaction is inspired by tag-oriented archive search, but the AUSE
Discovery domain term remains Search Filter Suggestion. Accepted values remain
Academic Catalog Values, Taxonomy Values, academic periods, and structured
search filters. No new Tag domain object is introduced.

## Required outcome

- Typing alone never requests result updates on every keystroke.
- Suggestions are computed locally from catalog and facet data already loaded
  by the Search page.
- Suggestions appear only for conservative exact or prefix matches.
- Matching is case-insensitive and has no typo correction or fuzzy matching in
  the initial version.
- A misspelling or unrelated trailing term produces no suggestion.
- Correcting or removing that text recomputes suggestions immediately.
- Only the nearest eligible suffix ending at the caret can become a filter.
- One Tab action converts exactly one highlighted suggestion.
- Every unmatched free-text term remains intact.
- Accepted filters use the same URL state, selected-filter chips, left-panel
  selection, filter semantics, and result request as mouse-selected filters.
- Already-selected values are not suggested.
- Collisions remain visible and are labelled by dimension.
- Mouse, arrows, Tab, Enter, Escape, focus, and screen-reader behavior follow
  the explicit interaction contract below.
- The small active-suggestion hint shows a Tab keycap and `to select filter`
  without visually dominating the result label.
- Existing search ordering from Increment 17 remains unchanged.

## Initial suggestion dimensions

Use only active dimensions already represented in the public left filter panel
and already available from loaded catalogs or search facets:

| Dimension | Search state field | Cardinality | Source |
| --- | --- | --- | --- |
| Academic year | `academic_year` | Scalar | Search facet values |
| Semester | `semester` | Scalar | Existing Semester choices |
| Program | `program_key` | Multiple | Public catalogs |
| Course | `course_key` | Multiple | Public catalogs |
| Category | `category_key` | Multiple | Public taxonomy catalog |
| Platform | `platform_key` | Multiple | Public taxonomy catalog |
| Domain | `domain_key` | Multiple | Public taxonomy catalog |
| Technology | `technology_key` | Multiple | Public taxonomy catalog |

Do not suggest these in Increment 18:

- Major and Topic, because those dimensions are paused in the current public
  interface;
- Person, Advisor, or Student ID, because names and identifiers already have
  direct text-search behavior and require separate collision decisions;
- Project File availability booleans or Artifact types;
- sort choices;
- Project titles or result links;
- free-form values outside controlled catalogs and facets.

Suggestions should degrade cleanly while sources are loading or unavailable.
The normal search input and left filter panel remain functional. Do not add a
new request solely to populate suggestions.

## Suggestion model

Use one small presentation-independent model equivalent to:

```text
id
dimension
dimensionLabel
stateField
value
valueLabel
cardinality
matchAliases
```

The exact TypeScript type and file location remain implementation choices.
Suggestion IDs must be stable and unique across dimensions, for example a
combination of state field and value.

Generate suggestions from the same choice data used by the left panel. Avoid a
second independently maintained dimension list when the current filter
definitions can be extended or adapted safely.

Aliases may include:

- the visible English label;
- the stable key with underscores treated as spaces when useful;
- the four-digit Academic year value;
- existing Semester labels and canonical values.

Do not invent synonyms, abbreviations, inferred meanings, or related words.

## Matching contract

### Caret boundary

Suggestions appear only when:

- the search input has focus;
- the selection is a collapsed caret;
- the caret is at the end of the input;
- the input does not end in whitespace;
- at least one eligible controlled value matches the active suffix.

Moving the caret away from the end closes suggestions without altering text.
Returning it to the end recomputes them.

### Candidate suffixes

Consider suffixes that:

- end exactly at the caret;
- begin at the start of the input or a word boundary;
- contain a bounded amount of text;
- normalize to a nonempty value.

The implementation may cap suffix inspection to a reasonable length such as 80
characters. It must not scan an unbounded input repeatedly.

For each controlled alias, find the longest eligible suffix that is an exact or
prefix match of that alias. Matching is case-insensitive. Normalize surrounding
and repeated whitespace and treat underscores in stable keys as spaces. Do not
apply edit distance, sound-alike matching, stemming, or arbitrary substring
matching.

Examples:

```text
Input: 2025
Match: Academic year: 2025

Input: best gam
Active suffix: gam
Match: Category: Game

Input: best game project
Active suffixes end in project
Result: no Game suggestion

Input: machine learning 2025
Active suffix: 2025
Match: Academic year: 2025

Input: fuck
Candidate: Duck
Result: no match because Duck does not start with fuck
```

### Minimum confidence

- A normal textual fragment requires at least three normalized characters.
- A shorter fragment is eligible only when it exactly matches the complete
  controlled alias, such as `Go`.
- An Academic year requires exactly four digits and must exist in the available
  Academic year facet choices.
- A trailing space closes suggestions.
- A misspelling closes suggestions.
- Deleting or correcting the mismatch reopens valid suggestions immediately.

These initial matching rules are deliberately provisional. A later manual
interaction review may justify a separate fuzzy-matching decision, but no fuzzy
logic or dependency enters Increment 18.

### Ranking and collisions

Return a bounded list, recommended maximum six.

Rank deterministically by:

1. exact alias match before prefix match;
2. more consumed suffix characters before fewer;
3. stable dimension priority matching the public filter-panel order;
4. normalized visible label;
5. stable suggestion ID.

Do not silently discard collisions. If `game` matches values in two dimensions,
show both as separate rows with labels such as `Category: Game` and
`Platform: Game`. The first ranked item is highlighted automatically. Arrow
navigation can choose another collision before Tab acceptance.

## One-at-a-time conversion contract

One acceptance converts only the highlighted suggestion associated with the
nearest eligible suffix. It never extracts every recognizable term in one
operation.

Required sequence:

```text
Draft: machine learning 2025
Tab 1: select Academic year: 2025
Remaining draft and submitted query: machine learning
Recompute immediately
Next suggestion, only when it is a real controlled value: <dimension>: Machine Learning
Tab 2: select that one suggestion
Remaining draft and submitted query: empty
```

The common path is earlier conversion:

```text
Draft reaches mach...
Machine Learning appears when a controlled alias matches
Tab: select Machine Learning
Draft becomes empty
Draft reaches 2025
Tab: select Academic year: 2025
```

`best gam` followed by Tab selects Category Game and leaves `best`. `best game
project` has no eligible Game suggestion because Game is not the nearest suffix
ending at the caret.

## Text preservation

Store the exact source range consumed by every suggestion. Acceptance removes
only that range and adjacent boundary whitespace needed to avoid doubled or
trailing spaces. Preserve all other characters and word order.

Since Increment 18 suggests only at the end of the input, no suffix exists
after the consumed range. The remaining prefix becomes both:

- the visible draft input value;
- the submitted `q` value, or undefined when empty.

Do not clear unrelated text. Do not rebuild the query from normalized tokens.
Whitespace cleanup may trim the new end and collapse only the removal boundary.

## Shared filter mutation

Extract or introduce one focused pure operation that applies a filter value to
`SearchState`. Both the left filter panel and Search Filter Suggestions must use
the same operation where their behavior overlaps.

Required cardinality behavior:

- Academic year and Semester replace their current scalar value.
- Program, Course, Category, Platform, Domain, and Technology append a missing
  value to the existing array.
- Existing array order remains deterministic.
- Applying a value already present is a no-op and such a suggestion should have
  been excluded earlier.
- Every accepted suggestion clears the cursor and local previous-page history
  through the existing Search page state transition.

The accepted OR-within-dimension and AND-across-dimensions semantics remain
unchanged. Increment 18 may add concise visible help near the filter surface to
explain that contract if it can be done without broad copy or layout changes.

Do not introduce a generic global state store. Search URL state remains
canonical for applied filters; component-local state owns draft text, popup
visibility, active option, dismissal stage, and caret state.

## Result-request behavior

Typing changes only the existing draft input state. It must not update `q`, URL
parameters, TanStack Query keys, or result requests.

Acceptance performs one existing Search page state transition containing:

- the remaining free-text query;
- the applied structured filter;
- no cursor;
- the unchanged explicit or implicit sort intent;
- every other existing filter.

That transition triggers the normal filtered search once. Avoid an intermediate
request that submits the text before applying the filter or applies the filter
before updating the remaining text.

Pressing Enter submits the complete current draft as free text through the
existing search form. It never accepts a highlighted suggestion. The popup
closes as part of submission.

## Keyboard and focus contract

### Popup opening

- Input focus plus a valid match opens the popup.
- The first suggestion is highlighted automatically.
- DOM focus remains on the input.
- `ArrowDown` and `ArrowUp` cycle or move through the bounded suggestions.
- The implementation must choose one predictable boundary behavior, either
  clamp or wrap, and test it. Clamping is preferred for lower surprise.

### Tab

When the popup is open and a suggestion is highlighted:

- Tab accepts exactly that suggestion;
- the event prevents normal focus movement for that one action;
- focus stays in the search input;
- the applied filter appears in URL state, the selected-filter area, and the
  left filter panel;
- the remaining query appears in the input;
- suggestions recompute immediately from the remaining query.

When no popup is open, Tab retains ordinary browser focus navigation.

### Enter

Enter always submits the current draft as free text, even while a suggestion is
highlighted. Enter never accepts a Search Filter Suggestion in this increment.

### Escape

Escape has a deliberate two-stage contract:

1. First Escape while the popup is open closes and suppresses suggestions for
   the exact current draft and caret state. Input focus and the caret remain.
2. Any text or caret change clears suppression and immediately allows matching
   suggestions to reopen.
3. Second Escape while the same input remains unchanged and the popup remains
   dismissed removes focus from the search input.
4. After focus leaves, later Tab behavior follows ordinary document navigation.

If no suggestion was open and no dismissal stage exists, Escape must not create
surprising text changes. The implementation may begin the two-stage sequence or
leave the input focused, but this minor behavior must be consistent and tested.

Real-browser verification must confirm that focus after the second Escape and
the next Tab follows a predictable document-order path. If raw `blur()` causes
the browser to restart focus from an unexpected location, move focus to the next
logical search control while still satisfying the visible outcome that the
search input is no longer focused.

### Mouse and touch

- Pointer activation accepts the clicked suggestion exactly once.
- Prevent premature input blur from closing the popup before the selection is
  applied.
- Outside interaction closes the popup and uses normal focus behavior.
- Suggestion rows retain usable touch targets at narrow widths.

## Accessible combobox contract

Use the editable combobox with listbox-popup semantics already supported by
native input and ARIA primitives. Do not add a combobox library.

At minimum:

- the search input exposes `role="combobox"` when needed;
- `aria-autocomplete="list"` communicates suggestion behavior;
- `aria-expanded` reflects popup visibility;
- `aria-controls` references the listbox;
- `aria-activedescendant` references the highlighted option while open;
- the popup uses `role="listbox"`;
- every suggestion uses `role="option"`, a stable ID, and accurate
  `aria-selected` state;
- DOM focus remains in the input during arrow and Tab interaction;
- dimension and value form one understandable option name;
- a persistent or conditionally visible description explains Arrow keys, Tab,
  Enter, and Escape;
- popup opening, closing, and selection do not create an assertive live-region
  interruption on every keystroke.

The product deliberately assigns Tab to acceptance and Enter to search while
the popup is open, which differs from the common APG Enter-acceptance model.
Therefore visible and screen-reader instructions are mandatory, and browser
keyboard verification is part of acceptance.

## Visual presentation

Keep the existing search layout and semantic tokens.

- Position the listbox directly beneath the text input within a local relative
  wrapper.
- Match the input width.
- Keep the popup above nearby content without changing page geometry.
- Bound its height and scroll overflow when needed.
- Distinguish the active row through more than color alone when practical.
- Show the dimension as quiet supporting text and the controlled value as the
  primary label.
- Place a small keycap-style `<kbd>Tab</kbd>` hint at the trailing edge of the
  highlighted suggestion with nearby `to select filter` text.
- Keep the hint non-interactive and outside the accessibility tree when the
  equivalent instruction is already provided through `aria-describedby`.
- Avoid animation beyond an optional short opacity appearance that respects
  reduced motion.
- Preserve 200 percent zoom, narrow-screen use, visible focus, and existing
  Search button and sort-control access.

No icon package, portal system, popover framework, animation library, or design
system is required.

## Suggested component boundary

A focused decomposition may contain:

- pure candidate construction and matching functions;
- a pure shared filter-application function;
- a narrow `SearchFilterSuggestions` component or hook that owns combobox
  interaction state;
- SearchPage wiring that supplies choices, draft state, applied state, and one
  acceptance callback.

Exact filenames and whether matching lives in one or two modules remain coder
choices. Avoid placing the entire interaction state machine inline in
`app-shell.tsx`, but do not create a generic autocomplete framework.

The boundary is justified only for this repeated search-filter behavior. It is
not a general application combobox abstraction.

## Edge cases

Cover these explicitly:

- empty input;
- whitespace-only input;
- trailing whitespace;
- caret not at the end;
- non-collapsed text selection;
- uppercase and mixed-case input;
- exact short value such as Go;
- two-character non-exact fragment;
- four-digit year that exists;
- four-digit year that does not exist;
- misspelling that later becomes correct;
- unrelated trailing word;
- multiword controlled value;
- collision across dimensions;
- already-selected multiple value;
- current scalar value and a different replacement scalar value;
- popup source data loading or unavailable;
- first Escape, text after Escape, and second Escape;
- Tab with popup, Tab without popup, Enter with popup, and pointer selection;
- rapid repeated Tab acceptance across two sequential suffixes;
- result request in flight while another suggestion is accepted;
- browser history navigation changing applied filters and draft query.

Do not add speculative handling for input method editor composition beyond
respecting native composition events and avoiding acceptance while composition
is active. A focused test is appropriate if the component listens directly to
keyboard events.

## Scope

### Included

- Search Filter Suggestion candidate model.
- Conservative local exact and prefix matching.
- Nearest-suffix range detection and one-at-a-time removal.
- Initial dimensions listed in this brief.
- Shared left-panel and suggestion filter application logic.
- Accessible combobox and listbox semantics.
- Mouse, arrow, Tab, Enter, and two-stage Escape behavior.
- Tab keycap hint and screen-reader instructions.
- Immediate recomputation after text correction or filter acceptance.
- Existing URL-state, chip, sidebar, pagination-reset, and result-query
  integration.
- Concise filter-combination guidance if it fits the focused surface.
- Focused pure-function, component, accessibility, and browser verification.
- One coherent implementation commit.

### Excluded

- Backend, OpenAPI, generated-client, Meilisearch, PostgreSQL, or schema changes.
- Live result requests on each keystroke.
- New catalog or suggestion endpoints.
- Fuzzy matching, typo tolerance, edit distance, stemming, synonym inference,
  or AI intent detection.
- Person, Advisor, Student ID, Artifact, availability, sort, Project-title, or
  result suggestions.
- Major or Topic suggestions while those public dimensions remain paused.
- Search history, trending searches, recent searches, or analytics.
- Tokenizing every recognizable term in one action.
- Suggesting from report bodies, files, or Project results.
- A reusable application-wide autocomplete framework.
- New third-party libraries.
- Search ordering changes from accepted Increment 17.
- Classification OR and AND semantic changes.
- Publication or deployment.

## Focused verification

### Pure matching tests

- Case-insensitive exact and prefix matching.
- No fuzzy match for a misspelling or unrelated word.
- Three-character threshold and exact shorter-value exception.
- Existing and missing four-digit Academic years.
- Trailing whitespace and caret-not-at-end suppression.
- Longest eligible suffix selection.
- `best gam` matches Game and consumes only `gam`.
- `best game project` does not match Game.
- `machine learning 2025` matches only 2025 first.
- After removing 2025, Machine Learning is eligible only when it exists as a
  real controlled value.
- Exact-before-prefix ordering.
- Deterministic collision ordering with dimension labels.
- Already-selected values excluded.
- Maximum suggestion count enforced.
- Range removal preserves all unmatched text.

### Component interaction tests

- Typing opens suggestions without calling the search API.
- Correcting a mismatch reopens suggestions immediately.
- Active option, arrows, ARIA IDs, expanded state, and selected state remain in
  sync.
- Tab accepts one option, prevents focus movement, keeps input focus, applies
  the filter, preserves remaining text, resets paging, and issues one search.
- A second immediate Tab can accept a newly recomputed remaining suggestion but
  does not batch-convert both suggestions in the first action.
- Tab without an open popup follows normal focus behavior.
- Enter with an open popup submits free text and does not accept the option.
- First Escape dismisses without blur; text change reopens; second Escape with
  unchanged input removes focus.
- Mouse selection applies exactly one option without a blur race.
- The selected filter appears in the selected-filter area and left panel.
- Removing the filter through an existing chip or left-panel action makes it
  eligible again when the draft matches.
- Collision navigation selects the chosen dimension rather than the first
  label match.
- Existing explicit and implicit sort state is preserved.
- Existing search submission, filters, paging, and Logo loading tests pass.
- No unhandled React or jsdom errors remain.

### Accessibility and layout checks

- Automated accessible-name and role queries cover combobox, listbox, and
  options.
- Keyboard-only traversal reaches every existing control.
- Screen-reader help describes the custom Tab and Enter behavior.
- The visual Tab keycap does not duplicate accessible text.
- The popup does not cover the active input or become clipped by its local
  container.
- 200 percent zoom and narrow layout retain the input, suggestions, keycap,
  Search button, and sort control.
- Reduced motion introduces no required animation.

### Browser interaction smoke

Use the existing local frontend and stack. Do not claim this smoke unless run.

1. Open an empty Search page and focus the search input.
2. Type `2025`; confirm one Academic year suggestion, a highlighted row, and the
   small Tab keycap instruction.
3. Press Tab; confirm Year 2025 is selected, the input stays focused and empty,
   and one filtered result request occurs.
4. Type a valid mixed-case classification prefix and accept it with Tab; confirm
   case-insensitive matching and a second selected filter.
5. Type `best gam`; accept Game and confirm `best` remains and becomes the
   submitted free-text query.
6. Type an unrelated trailing term after a possible earlier match; confirm the
   earlier match is not offered. Remove the trailing term and confirm the
   suggestion returns immediately.
7. Create a collision, navigate with Arrow keys, and confirm Tab applies the
   highlighted dimension.
8. With a popup open, press Enter; confirm a free-text search runs and no filter
   is accepted.
9. Reopen a popup, press Escape once, and confirm the popup closes while focus
   stays in the input. Type one character and confirm suggestions can return.
10. Dismiss again, press Escape a second time without changing input, and
    confirm input focus leaves predictably. Confirm later Tab follows normal
    page navigation.
11. Use pointer selection and confirm no blur race or duplicate search.
12. Repeat at 200 percent zoom and a narrow viewport.

A focused Playwright test for the custom Tab and two-stage Escape path is
preferred when the existing test harness can express it without new
infrastructure. Otherwise, component coverage plus the completed manual smoke
is acceptable, but the handoff must state which path ran.

### Static and build checks

- Run focused matching and component tests during implementation.
- Run the complete frontend component suite once after the interaction is
  coherent.
- Run frontend ESLint and `tsc -b` once.
- Run the default-subpath production build once.
- Run the focused existing Playwright path only if extended.
- Run `git diff --check`.
- Inspect the complete diff for excluded backend, contract, generated-client,
  search-engine, dependency, and infrastructure changes.

No Go, PostgreSQL, Meilisearch, storage, import, or full product acceptance test
is required because those boundaries do not change.

## Suggested implementation order

1. Build the typed suggestion definitions from existing SearchPage choices.
2. Add pure normalization, suffix matching, ranking, exclusion, and range
   removal functions.
3. Extract the shared filter-application operation and route existing left-panel
   mutations through it where applicable.
4. Add the focused combobox interaction component or hook.
5. Wire one atomic acceptance transition into SearchPage.
6. Add semantic listbox markup, instructions, and the Tab keycap presentation.
7. Add pure and component tests before broad verification.
8. Run the real-browser keyboard smoke and adjust only interaction defects.
9. Update backlog item 25 to implemented and pending independent review without
   claiming acceptance.
10. Commit the planning files and implementation once, then stop.

## Completion criteria

- Conservative suggestions appear promptly without live result search.
- One Tab converts one nearest-suffix suggestion.
- Remaining free text is preserved and submitted with the accepted filter.
- Matching is case-insensitive exact or prefix matching without fuzzy behavior.
- Misspellings and unrelated trailing terms suppress suggestions; correction
  restores them immediately.
- Collisions, already-selected values, scalar replacement, and multiple-value
  append behavior are correct.
- Filter state is shared with the existing left panel and selected-filter chips.
- Enter remains Search, Tab is suggestion acceptance only while open, and the
  two Escape stages behave exactly as specified.
- Combobox semantics and custom keyboard instructions are accessible.
- The Tab keycap hint is visible, small, and non-intrusive.
- Existing sorting, paging, filtering, result rendering, and Logo behavior
  remains unchanged.
- Focused tests, the full frontend suite, static checks, production build, and
  browser interaction verification pass.
- No backend, contract, search-engine, database, dependency, publication, or
  deployment change occurs.
- One coherent main-repository commit uses a lowercase past-tense message with
  no prefix and no co-author trailer.

## Required handoff

Stop after the implementation commit. Report:

- accepted Increment 17 baseline and final commit hash;
- component and pure-function boundaries;
- included suggestion dimensions and source data;
- exact normalization, suffix, threshold, ranking, collision, and exclusion
  behavior;
- exact one-at-a-time removal behavior for the required examples;
- shared filter mutation and atomic SearchState transition;
- mouse, arrows, Tab, Enter, two-stage Escape, focus, and composition behavior;
- ARIA structure, instructions, and Tab keycap presentation;
- proof that typing does not request results;
- focused and full frontend verification results;
- exact browser or Playwright interaction procedure and result;
- every failed or skipped check and any residual browser risk;
- confirmation that backend, OpenAPI, generated clients, Meilisearch,
  PostgreSQL, dependencies, search ordering, classification semantics,
  publication, and deployment did not change;
- confirmation that backlog item 25 remains pending independent review;
- final `git status --short`.

## Research basis

- WAI-ARIA Authoring Practices defines editable combobox and listbox-popup
  semantics, active-descendant focus, arrow navigation, Escape dismissal, and
  normal text editing behavior:
  <https://www.w3.org/WAI/ARIA/apg/patterns/combobox/>
- The WAI-ARIA editable list-autocomplete example demonstrates native input,
  listbox, option, expanded, controls, autocomplete, and active-descendant
  relationships without requiring a component framework:
  <https://www.w3.org/WAI/ARIA/apg/patterns/combobox/examples/combobox-autocomplete-list/>

Increment 18 deliberately keeps Enter assigned to search and Tab assigned to
suggestion acceptance while the popup is open. That product decision differs
from the common Enter-to-accept pattern and therefore requires explicit visible
and screen-reader instructions plus real-browser verification.
