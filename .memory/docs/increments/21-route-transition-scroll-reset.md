# Route transition scroll reset increment

## Assignment

Correct public route navigation so a newly selected page begins at the top of
the document with the complete site header visible. Preserve the shared shell's
route-focus accessibility behavior without allowing focus to scroll the header
out of view.

This is a small frontend-only increment after Increment 20 acceptance. It must
cover the AUSE Discovery brand link to the landing page and Browse the archive
to the Search page, then apply the same coherent policy to other internal
pathname changes owned by the shared application frame.

## Required reading

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/implementation-status/README.md`
4. `.memory/implementation-status/backlog.md`, especially item 32
5. `.memory/implementation-status/completed.md`, especially Increment 20 after
   acceptance
6. `frontend/src/app-shell.tsx`, limited to `AppFrame`, `HomePage`, routing,
   and the relevant internal links
7. `frontend/src/app-frame.test.tsx`
8. relevant public navigation browser tests
9. This brief

## Observed problem

The shared `AppFrame` focuses the `<main>` landmark after a pathname change.
That focus currently uses the default browser behavior, which may scroll the
focused main element into view. The main element begins below the site header,
so internal navigation can leave the destination partially scrolled with the
header above the viewport.

React Router also does not imply that every client-side pathname transition
must begin at document position zero. The application needs an explicit policy
that coordinates visual scroll position with the existing focus transition.

The supplied screenshots demonstrate the Search route both with the complete
header and with the header omitted by the carried or focus-induced scroll
position.

## Required outcome

- Activating the AUSE Discovery brand link from another route opens the landing
  page at document position zero with the full site header visible.
- Activating Browse the archive from the landing page opens Search at document
  position zero with the full site header visible.
- Submitting the landing-page search form opens Search at document position
  zero under the same policy.
- Other new internal pathname navigations through the shared shell receive the
  same top-of-page behavior rather than route-specific one-off fixes.
- The destination `<main>` landmark still receives programmatic focus after a
  pathname change for keyboard and assistive-technology orientation.
- Main-landmark focus uses `preventScroll` so focus cannot move the viewport
  below the header.
- The reset is immediate. No smooth scrolling or visible scroll animation is
  introduced.
- Search state changes that keep the `/search` pathname, including query,
  filters, sort, and pagination, do not reset the visitor's scroll position.
- Hash-only navigation, including Skip to main content, retains its native
  target behavior.
- Initial rendering does not add a redundant focus or scroll transition.

## Navigation policy

### New pathname navigation

For a normal internal navigation that selects a different pathname:

1. render the destination route;
2. focus the existing main landmark with `{ preventScroll: true }`;
3. place the document at horizontal and vertical position zero without smooth
   animation.

The exact effect ordering may be adjusted to avoid an intermediate visual jump,
but the final state must be:

```text
window.scrollX = 0
window.scrollY = 0
document.activeElement = #main-content
site header visible
```

Use the shared application-frame boundary. Do not duplicate `scrollTo` calls in
the landing page, Search page, About page, and administrator pages.

### Browser history navigation

Back and Forward navigation should not be converted blindly into a new-page
top reset. Preserve native or existing history scroll restoration where the
router and browser provide it, while ensuring main-landmark focus uses
`preventScroll` and cannot overwrite the restored position.

The implementation may use React Router's navigation type to distinguish a
history `POP` from `PUSH` and `REPLACE`. Do not add a complete custom
scroll-position registry unless focused browser evidence proves it necessary.

### Same-path navigation

Changes to `location.search` on the same pathname are application state changes,
not new pages. They must not reset scroll. This protects Search filter changes,
sort changes, accepted suggestions, and cursor paging from unexpected jumps.

The brand link has one useful same-path exception: activating the AUSE Discovery
brand while already on the landing page should return the page to the top. Keep
that behavior on the semantic link itself without spreading special click
handlers across navigation.

Do not make the already-active Search navigation link reset an in-progress
Search page unless existing product behavior already performs a real pathname
transition.

## Focus and accessibility contract

Preserve the accepted shared-shell behavior:

- one `<main id="main-content">` landmark;
- `tabIndex={-1}` for programmatic route focus;
- the Skip to main content anchor and hash target;
- semantic Link and NavLink activation by pointer and keyboard;
- route-specific document titles;
- no focus transfer on the first render.

`preventScroll` changes only the visual side effect of focus. It must not remove
focus management or shift focus to the body, header, brand link, or Search
field.

The implementation must work for keyboard activation of the brand and Browse
links as well as pointer activation. No hidden announcement, timer, or new
instructional text is required.

## Component boundary

Keep the route-transition policy in or immediately beside `AppFrame`. A small
focused hook or component is acceptable if it makes navigation-type and first
render handling testable. A generic navigation framework is not justified.

Prefer browser APIs and the existing React Router version. Do not add a scroll
restoration package, migrate to a data router solely for `ScrollRestoration`,
or introduce page-specific CSS offsets.

Avoid fixing the symptom with a sticky header, negative margin, artificial top
padding, or fixed positioning. The header layout is not the cause.

## Scope

### Included

- Shared internal pathname-transition scroll reset.
- Main-landmark focus with `preventScroll`.
- Brand navigation to the landing page.
- Browse the archive and landing-search navigation to Search.
- Same-path brand activation returning the landing page to the top.
- Preservation of same-path Search scroll position.
- Focused component and real-browser navigation verification.
- One coherent implementation commit.

### Excluded

- Header redesign, sticky navigation, footer changes, or layout restyling.
- Search result ordering, filters, autocomplete, or filter-panel behavior.
- Project Logo loading or image changes.
- Custom persistent scroll-position storage.
- Backend, OpenAPI, generated-client, database, search-engine, storage, import,
  deployment, or infrastructure changes.
- New dependencies.
- Publication or production deployment.

## Focused verification

### Application-frame tests

- Initial rendering does not call the route-transition scroll reset and does not
  programmatically focus main.
- A different-path internal navigation focuses `#main-content` using
  `preventScroll` and requests position zero.
- Browse the archive reaches `/search` and applies the route-transition policy.
- The brand link reaches `/` from Search and applies the route-transition
  policy.
- Brand activation while already on `/` returns the document to position zero.
- A Search query-string-only transition does not request position zero.
- Skip to main content retains `href="#main-content"` and is not intercepted.
- Existing shared frame, route title, login redirect, and error-page navigation
  tests remain green.

Mock only the missing DOM scroll implementation required by the component test
environment. Assert meaningful coordinates and focus options instead of
browser-layout details that jsdom cannot provide.

### Browser smoke

1. Open Search, scroll far enough that the site header is outside the viewport,
   and activate the AUSE Discovery brand link.
2. Confirm the landing page begins at `scrollY === 0`, the complete header is
   visible, and main has route focus.
3. Scroll the landing page, then activate Browse the archive.
4. Confirm Search begins at `scrollY === 0`, the complete header is visible,
   and main has route focus.
5. Repeat the landing-to-Search transition by submitting the landing search
   form.
6. On Search, scroll into the results and change a filter or sort option.
7. Confirm the same-path Search update does not jump to the top.
8. Use browser Back and Forward once and confirm route focus does not destroy
   the browser's restored scroll position.
9. Repeat brand and Browse activation using keyboard controls.
10. Repeat at a narrow viewport and 200 percent zoom, confirming no partial
    header clipping caused by the transition.

The handoff must state whether real-browser verification ran. Component tests
alone cannot prove viewport behavior.

### Static and build checks

- Run focused application-frame tests.
- Run any affected public navigation tests.
- Run the complete frontend component suite once.
- Run ESLint and `tsc -b` once.
- Run the default-subpath production build once.
- Run a focused Chromium navigation test against the existing local-test stack
  if available.
- Run `git diff --check`.
- Inspect the complete diff for excluded feature, backend, dependency,
  deployment, and infrastructure changes.

No Go, database, Meilisearch, storage, import, or full-stack verification is
required.

## Suggested implementation order

1. Add focused AppFrame route-transition tests that reproduce the scroll caused
   by focus and the carried Search position.
2. Coordinate main focus and top reset at the shared frame boundary.
3. Cover the same-path landing-page brand exception without changing Search
   query navigation.
4. Add a focused Chromium navigation test or execute the browser smoke.
5. Update backlog item 32 to implemented and pending independent review.
6. Commit once and stop.

## Completion criteria

- Brand navigation to Home and Browse navigation to Search end at document
  position zero with the full header visible.
- Route focus remains on main and never causes a second downward scroll.
- Landing search submission follows the same behavior.
- Same-path Search state changes preserve scroll.
- Skip-link and history behavior do not regress.
- Pointer, keyboard, narrow-viewport, and 200 percent zoom checks pass.
- Focused, full frontend, static, build, and browser checks pass.
- No layout workaround, dependency, backend, contract, data, deployment, or
  infrastructure change occurs.
- One lowercase past-tense commit with no prefix or co-author trailer contains
  the implementation and status update.

## Required handoff

Stop after the implementation commit. Report:

- accepted Increment 20 baseline and final commit hash;
- confirmed cause of the partial scroll state;
- shared pathname-transition policy and navigation-type handling;
- main-focus behavior and `preventScroll` use;
- same-path brand behavior;
- proof that Search query, filter, sort, and pagination changes do not reset
  scroll;
- focused, full frontend, static, build, Chromium, and browser-smoke results;
- any skipped check or residual browser-specific risk;
- confirmation that Increment 20 behavior and every excluded boundary remain
  unchanged;
- confirmation that item 32 remains pending independent review;
- final `git status --short`.
