# Product coherence increment

## Assignment

Implement and commit the final delegated product-coherence increment for AUSE Discovery.

This is one bounded frontend and web-serving increment. Complete the shared application shell, public and administrator presentation states, localization coverage, responsive and keyboard behavior, neutral launch-content seams, and Nginx static-response hardening described below. Stop after focused verification, durable status updates, one coherent commit, and a review handoff.

Do not perform final technical MVP acceptance in this increment. The reviewing task owns the broader clean-stack, restart, recovery, cross-system, security, and accessibility acceptance pass after this increment is accepted.

The accepted implementation baseline is commit `27ecdd0` (`Corrected CI branch and image metadata contracts`). Commit `dcc1867` only advances the accepted-checkpoint pointer. Treat the latest repository commit and this brief checkpoint as an unfinished handoff layered on the accepted application baseline. Preserve all accepted behavior.

## Required reading before coding

Read the following files in order:

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/project_context.md`
4. `.memory/architectural_decision_logs.md`
5. `.memory/mvp_implementation_status.md`
6. `.memory/lessons_learned.md`
7. `CONTEXT.md`
8. `docs/ause-discovery_specification_v1.md`, especially sections 8, 16, 23 through 32, 34, 43, 49, 50, 55, and 58
9. `docs/ause-discovery_implementation_specification_v1.md`, especially sections 5, 7.6, 11, 12, 13.3, 15, 16.7, 17, and 20
10. `docs/increments/08-ci-operator-readiness.md`, especially its exclusions and verification policy
11. This brief

Then inspect only these implementation surfaces and nearby tests before editing:

- `frontend/src/app-shell.tsx`
- `frontend/src/App.module.css`
- `frontend/src/index.css`
- `frontend/src/app/i18n.ts`
- `frontend/src/app/runtime.ts`
- `frontend/src/features/admin/`
- `frontend/src/features/search/`
- `frontend/src/**/*.test.*`
- `frontend/index.html`
- `frontend/public/`
- `frontend/src/assets/`
- `frontend/Dockerfile`
- `deploy/nginx/default.conf.template`
- `deploy/nginx/20-base-path.envsh`
- `compose.yaml`, only the `web` service and shared logging configuration
- `.env.example`, only public content or web-port documentation if a real configuration seam is added

Do not broadly reread backend packages, migrations, SQL, generated clients, import adapters, search internals, or CI implementation. Existing backend and generated contracts are accepted.

## Repository starting state

The following behavior already works and must remain intact:

- Public Home, Search, Project, and Person routes use the generated API client and configured base path.
- Administrator authentication, Project and Person maintenance, Artifact management, imports, search maintenance, and audit routes are wired to real services.
- Search, import, audit, and Artifact feature tests cover their accepted functional boundaries.
- Visible copy mostly passes through English i18n resources.
- Semantic color, spacing, typography, radius, and layout tokens already exist.
- Public pages have a header, footer, main landmark, skip link, and route-specific document titles.
- The default `/ause-discovery/` and root frontend builds pass.
- Nginx serves the SPA and proxies API traffic with base-path-safe routing and immutable asset caching.
- The Go API already applies security headers and runs as a non-root user.

The following concrete coherence gaps are present:

- Administrator and login routes sit outside the public application frame, so layout width, landmarks, skip navigation, header, and footer behavior are inconsistent.
- The skip link is labelled `Back to results` instead of `Skip to main content`.
- Administrator child routes share the generic `Administration` document title rather than route-specific titles.
- session expiry during an administrator API request does not reliably clear frontend session state and return through the protected-route login flow.
- Project and Person administrator lists stop at the first cursor page and omit controlled loading, error, empty, and paging states.
- Person create and edit failures are not surfaced consistently. Person revision conflicts do not preserve an explicit recovery path.
- Several Project, Artifact, lifecycle, logout, and data-loading failures can leave controls disabled or fail without useful visible feedback.
- The delete confirmation presents modal semantics but lacks complete initial-focus, Escape, and focus-return behavior.
- public participation roles and some Artifact statuses render raw stable keys or missing translation keys.
- PDF View links do not explicitly open a new browser tab as required.
- About, Privacy, Accessibility, and Terms pages all show one generic placeholder, and Contact is not a route.
- Vite starter favicon and unused starter assets remain in the repository.
- responsive styles exist but dense navigation, forms, action groups, and tables need a focused narrow-width and zoom pass.
- Nginx static responses do not provide the accepted CSP, frame, referrer, permissions, and no-sniff baseline.
- The web image runs the stock Nginx master process as root and listens on port 80.

## Required outcome

Public and administrator routes form one coherent, base-path-safe application experience. Every route has appropriate landmarks, navigation, title, loading and failure behavior, localized stable values, and usable narrow-width presentation. Administrator session expiry returns through the existing login redirect contract. Required informational routes exist without inventing approved institutional content. Static responses carry a restrictive compatible security-header baseline. The web container runs Nginx as its existing unprivileged `nginx` account on an unprivileged internal port.

No accepted business behavior, wire contract, database state, or search behavior changes.

## Scope

### Included

- Shared application frame for public, login, and protected administrator routes.
- Correct skip navigation, route landmarks, heading hierarchy, and route-specific document titles.
- Frontend handling of an authenticated API response becoming `401`.
- Administrator Project and Person list filters and cursor navigation using existing generated API parameters.
- Administrator loading, empty, success, validation, conflict, and request-failure presentation where currently absent.
- Accessible delete-confirmation focus and keyboard behavior.
- Localization of visible stable roles, statuses, action labels, and new content.
- PDF View new-tab behavior and safe link attributes.
- About, Privacy, Accessibility, Terms, Contact, Not Found, and unexpected-error presentation.
- Neutral build-time configuration seams only when needed for optional contact or developer-credit values.
- Removal of unused Vite starter assets and replacement of the starter favicon with a neutral repository-owned mark.
- Focused responsive, zoom, focus, disabled, and status styling.
- Nginx security headers for static and proxied responses.
- Non-root Nginx runtime on an unprivileged internal port, with matching Compose wiring.
- Focused component tests, frontend static checks, both base-path builds, and a bounded web-image smoke check.
- Verified implementation status and durable lessons when appropriate.
- One coherent past-tense implementation commit.

### Excluded

- OpenAPI, generated client, SQL, migration, Go service, or API-handler changes.
- New backend endpoints, query parameters, roles, permissions, or content tables.
- Final institutional branding, logos, legal text, privacy approval, addresses, email addresses, social links, or developer identity.
- Invented contact values or claims that placeholder legal text has institutional approval.
- Dark mode, additional languages, a language picker, or automatic translation.
- A new design system, component library, CSS framework, icon dependency, global state library, or router replacement.
- Featured Projects, recommendations, analytics, telemetry, or content-body search.
- Person merge, taxonomy editing, catalog editing, permanent destruction, or new administration capabilities.
- Reworking accepted search, Artifact, import, audit, or Project service behavior.
- Dependency upgrades, vulnerability remediation, action changes, or release-workflow changes.
- Playwright test creation, full browser acceptance, clean-clone bootstrap, restart persistence, Meilisearch destruction/rebuild, or full Compose acceptance.
- Deployment, tag creation, image publication, registry login, or external production mutation.
- Broad source reformatting.

## Implementation order

Use the order below. Keep the application compiling between sections when practical.

1. Establish the shared shell and route metadata behavior.
2. Correct session-expiry routing and common async feedback.
3. Complete public route and localization coherence.
4. Complete administrator list, Person, lifecycle, and modal states.
5. Refine responsive and accessibility styling.
6. Remove starter assets and add neutral content seams.
7. Harden Nginx headers and non-root runtime.
8. Run focused verification and record only verified state.

Minor component extraction and private helper names remain implementation decisions. Avoid turning the 333-line application shell into a large architectural rewrite solely for aesthetics.

## Shared application frame

Refactor routing so one outer frame owns the page container, site header, primary navigation, one `<main>` landmark, and footer for all browser routes, including:

- `/admin/login`;
- `/admin` and every protected child route;
- public routes;
- Not Found.

Requirements:

- Keep one main landmark per rendered route.
- Keep the main content target at a stable ID and make it programmatically focusable without adding it to normal tab order.
- Change the skip-link copy to `Skip to main content` through i18n.
- Preserve React Router basename behavior and relative route generation.
- Keep the public site header available from login and administrator pages so returning to public discovery is clear.
- Keep the administrator secondary navigation inside the main region and preserve active-link semantics.
- Do not render the public footer or site header twice around nested administrator routes.
- Ensure administrator session-loading feedback renders inside the shared page structure.
- Give the header and footer navigation distinct accessible names.
- Use route changes to move focus to the main page heading or main content in a restrained way that does not steal focus during query refetches or same-page form mutations.
- Keep browser Back behavior and protected `next` query behavior intact.

Route-specific document titles are required for:

- Home;
- Search;
- public Project and Person details after data loads;
- About, Privacy, Accessibility, Terms, Contact, and Not Found;
- administrator login and overview;
- administrator Project list, new Project, edit Project;
- administrator People list and edit Person;
- Import upload and preview;
- Search maintenance;
- Audit log.

A small route-title component or location-to-title helper is acceptable. Do not scatter direct `document.title` assignments.

The top-level unexpected-error state must remain localized and should render inside a minimal readable page structure with a safe link back to Home or a reload action. It must not expose error objects or stack traces.

## Authenticated session expiry

Preserve server-side sessions and the existing `AdminGuard` redirect contract.

Add one handwritten frontend integration at the generated client boundary so any API `401` response can invalidate or clear the cached session. Do not edit generated files.

Required behavior:

- A protected administrator route that receives `401` clears frontend session state.
- `AdminGuard` then redirects to `/admin/login` with the full current path and query in `next`.
- A failed login remains on the login page and still shows the generic credential failure.
- Public requests that legitimately return `401` do not create a redirect loop.
- The response interceptor or subscription is installed exactly once and is cleaned up if its API requires cleanup.
- Session and CSRF token state remain synchronized after login and logout.
- No token, response body, username, or request detail is logged.

Use the generated client's supported interceptor surface or a small application-owned notification callback. Do not wrap `fetch` globally and do not fork the generated client.

## Public-route coherence

### Search and results

Preserve the current URL-owned Search state, facet semantics, safe highlighting, and cursor contract.

Make only bounded presentation corrections:

- Distinguish first load from background refetch so stale results remain readable during a refresh.
- Keep a visible accessible search status without announcing every keystroke excessively.
- Show a controlled Catalog-loading or Catalog-failure state instead of silently presenting empty facet lists.
- Rename cursor movement according to actual behavior. A control that replaces the current page must say `Next page`, not `Load more`.
- Provide a Previous control only when a valid local cursor history exists. Do not synthesize reverse cursors.
- Reset cursor history when query, sort, or filters change.
- Disable page controls during a request.
- Use an available bounded abstract highlight as an excerpt when the API returns one. Do not change the API merely to create an excerpt.
- Preserve text-only highlight rendering. `dangerouslySetInnerHTML` remains prohibited.
- Keep empty and unavailable states distinct.

Search interaction details below route level remain implementation choices. A draft search field may be introduced if needed to make the Search button meaningful, but URL shareability and filter-reset behavior must remain correct.

### Project, Person, and Artifact presentation

- Translate participation roles `student`, `advisor`, `co_advisor`, and `committee_member` through dedicated i18n keys.
- Present role labels in public Project and Person pages rather than raw stable keys.
- Keep Taxonomy keys as fallback content only when an English label is unavailable.
- Render optional Reference Code and Student ID elements only when values exist.
- Open PDF View links in a new browser tab with safe `rel` attributes. Apply the same behavior to administrator Artifact View links.
- Keep Download links as normal same-origin links governed by server content disposition.
- Preserve public Project metadata when no Artifacts exist.
- Distinguish Not Found from a temporary request failure when the available error shape permits that distinction without contract changes.

### Informational routes and launch-time content

Provide separate route content and i18n keys for:

- About;
- Privacy Policy;
- Accessibility Statement;
- Terms of Use;
- Contact and Institutional Information.

Add `/contact` and make Contact a footer link.

Safe factual content may describe the repository's current purpose, public metadata behavior, administrator-only maintenance, supported accessibility mechanisms, and the fact that final institutional policy or contact content awaits approval. Do not draft legal commitments, retention promises, compliance certifications, privacy guarantees, institutional addresses, or contact values.

The Privacy and Terms pages must visibly identify their content as pending institutional approval. The Accessibility page may state implemented mechanisms such as keyboard navigation, visible focus, semantic landmarks, and reduced-motion support only after those mechanisms are present.

If optional contact or developer-credit values are made configurable:

- use build-time `VITE_` values only;
- keep defaults empty;
- render absent values cleanly;
- document them as launch-time inputs in `.env.example`;
- never add fake names, links, or email addresses.

Configuration is optional. A localized pending-contact state is sufficient for the technical MVP.

The Not Found page must include a useful route back to Home or Search.

## Administrator coherence

### Project and Person lists

Use the existing generated list parameters. Do not change OpenAPI.

Project list requirements:

- query text filter;
- status filter for draft, published, and deleted;
- explicit Apply and Clear actions;
- cursor-based Next and local-history Previous actions;
- filter changes reset cursor history;
- loading, background loading, empty, and request-error states;
- disabled pagination while pending;
- stable title, status, academic year, and Edit columns;
- no false empty state while the first request is still pending.

Person list requirements:

- query text filter;
- explicit Apply and Clear actions;
- the same cursor-history, loading, empty, request-error, and disabled-control behavior;
- stable display name, Student ID, staff ID where useful, and Edit columns.

A small reusable cursor-history hook is acceptable because Project, Person, import, and audit pages now demonstrate a real repeated boundary. Do not replace accepted import or audit paging unless reuse is clearly mechanical and their tests remain intact.

### Person creation and editing

- Keep Person creation on the People page unless a small route extraction clearly reduces complexity.
- Validate seven-digit Student IDs before submission while allowing empty Student ID.
- Require a non-empty display name.
- Disable submit while pending or CSRF is unavailable.
- Surface server validation and request failure accessibly.
- On successful edit, show a restrained saved status and update the query cache with the returned Person.
- On `revision_conflict`, preserve entered values and offer an explicit reload-current-record action.
- Handle loading, Not Found, and unexpected request failure before rendering the edit form.
- Do not add Person deletion or merge controls.

### Project, Artifact, lifecycle, and logout feedback

Do not redesign accepted forms. Fill the missing feedback boundaries:

- Project and Catalog or Person dependency failures must not render a misleading empty form.
- non-validation Project save and publish failures need a general accessible error.
- successful draft save and publish need a restrained status or visibly updated state.
- Project Delete and Restore must disable while pending and display request or revision-conflict feedback.
- Artifact update, replacement, Delete, and Restore must keep their accepted behavior and use localized active/deleted statuses.
- Sign out must disable while pending and display a controlled failure without discarding an authenticated session on a failed server request.
- unexpected errors must not expose raw Problem Details internals unless a safe field issue is intentionally shown.

Do not refactor the accepted import, search-maintenance, or audit feature logic beyond route title integration, shared styles, localized common status, or a clearly mechanical shared helper.

## Delete-confirmation accessibility

Keep the Project-specific confirmation phrase and reversible Delete wording.

The modal interaction must:

- expose a labelled dialog with an accessible description;
- move initial focus to the confirmation input or safest useful control;
- close on Escape through the same Cancel path;
- keep keyboard focus inside the modal while open, using the platform `<dialog>` behavior or a small explicit focus trap;
- return focus to the Delete trigger after Cancel or successful close when the trigger still exists;
- prevent background pointer activation while open;
- disable confirmation until the exact value matches;
- disable actions while the mutation is pending;
- avoid password re-entry and permanent-destruction language.

Prefer the native `<dialog>` element when it can be used accessibly with the current browser baseline and tested without a large polyfill. A small explicit implementation is acceptable when native test-environment behavior becomes a blocker.

## Localization and content rules

Every new visible application string must use i18n. Correct existing visible stable-key leaks encountered in the named surfaces, including:

- participation roles;
- Artifact active/deleted status;
- page and navigation labels;
- list filters and pagination;
- loading, empty, success, conflict, and failure states;
- skip navigation;
- informational page sections.

Server-provided record content, identifiers, audit event names, import issue locations, and safe error codes are data and do not need translation. Stable internal enum values used as interface labels do need translation.

English remains the only complete locale. Do not add partial language resources or a locale switcher.

Keep `frontend/src/app/i18n.ts` manageable. A mechanical split into one or more English resource files is allowed only if it improves readability without changing the i18n initialization contract. Avoid a broad reformat of unrelated generated or accepted files.

## Responsive and visual coherence

Preserve the accepted restrained academic direction and semantic tokens. Final university colors and branding remain external.

Required checks and corrections:

- Maintain usable layout at a 320 CSS-pixel viewport.
- Maintain critical function at 200 percent zoom on a typical desktop viewport.
- Keep header, primary navigation, footer, administrator navigation, search controls, action rows, Artifact controls, and assignment fields from overlapping or clipping.
- Keep dense tables in an explicitly scrollable region with keyboard-reachable content. A complete card-view rewrite is not required.
- Ensure long Project titles, Person names, filenames, taxonomy labels, audit metadata, and UUIDs wrap without forcing page-level horizontal scrolling.
- Give disabled buttons and controls a clear non-color-only state and remove pointer cursor while disabled.
- Add hover styling only when it does not replace focus styling.
- Move the focus-ring color into a semantic token and maintain visible contrast against light surfaces.
- Keep status, warning, error, and conflict meaning in text or semantics, not color alone.
- Keep reduced-motion handling.
- Preserve readable line length and heading hierarchy.
- Avoid decorative animation, heavy shadows, excessive cards, gradients, or speculative branding.

No pixel-perfect redesign is required. The goal is coherent and robust presentation of existing product behavior.

## Starter asset cleanup

Inspect tracked usage before deletion.

- Remove unused `frontend/src/assets/react.svg`, `frontend/src/assets/vite.svg`, and `frontend/src/assets/hero.png` when repository search confirms no import.
- Remove unused `frontend/public/icons.svg` when repository search confirms no reference.
- Replace the Vite starter `frontend/public/favicon.svg` with a simple neutral AUSE Discovery mark implemented as repository-owned SVG source.
- Do not imitate or invent an Assumption University logo.
- Keep the favicon path base-path-safe through `%BASE_URL%`.
- Do not add generated raster artwork or a new image dependency.

## Nginx security headers

Add a compatible static and proxy response baseline without duplicating the public base path.

Required headers:

- `X-Content-Type-Options: nosniff`;
- `Referrer-Policy: same-origin`;
- `Permissions-Policy` disabling at least camera, microphone, and geolocation;
- `Content-Security-Policy` with `default-src 'self'`, `script-src 'self'`, `style-src 'self'`, `img-src 'self' data:`, `font-src 'self'`, `connect-src 'self'`, `object-src 'none'`, `base-uri 'self'`, `form-action 'self'`, and `frame-ancestors 'none'`.

Apply headers with `always` so controlled error responses retain them. Account for Nginx `add_header` inheritance: the current asset and SPA locations set Cache-Control, so security headers must not disappear in those locations. A small included snippet is acceptable.

Preserve:

- one-year immutable caching for fingerprinted assets;
- no-cache behavior for the SPA entry document;
- API and health proxying;
- Artifact streaming and range behavior through the API path;
- request ID and forwarded-header behavior;
- root and `/ause-discovery/` routing.

Do not add HSTS because this repository stack does not terminate confirmed production HTTPS. Do not allow inline scripts or broad wildcard sources merely to avoid testing the policy.

## Non-root web container

Keep the accepted pinned `nginx:1.30.4-alpine` image and make the runtime process use its existing `nginx` account.

Expected implementation shape:

- change the internal Nginx listener from privileged port 80 to an unprivileged port such as 8080;
- update the Compose web port mapping to target that internal port;
- grant the `nginx` account only the write access required for Nginx runtime directories and template rendering;
- ensure `/etc/nginx/conf.d` is writable for the official entrypoint's environment substitution;
- ensure cache, PID, and temporary directories required by the stock configuration are writable;
- keep static content read-only at runtime where practical;
- add `USER nginx` after image assembly;
- do not run package managers or grant broad world-writable permissions;
- do not replace the pinned image with a floating unprivileged image tag.

If the official entrypoint cannot render the current template after dropping privileges, correct directory ownership narrowly. Do not bypass the failure by returning to root.

## Focused tests

Add or extend focused tests around changed behavior. Tests should exercise contracts rather than copy markup structure.

Required frontend test boundaries:

1. Shared shell landmarks and correct skip-link label on a public route and an administrator or login route.
2. Route-specific document title for at least one public and one administrator route.
3. `401` session invalidation preserves the protected destination and returns to login without breaking generic login failure.
4. Search paging uses accurate labels, cursor history, and resets history after a filter change.
5. Project and Person administrator list paging or filtering behavior, preferably through a small reusable helper plus one rendered interaction.
6. Person validation and one server-failure or revision-conflict recovery path.
7. Participation-role and Artifact-status localization without raw stable-key leakage.
8. PDF View links use a new browsing context and safe relation attributes.
9. Delete confirmation initial focus, Escape cancellation, exact phrase gating, and focus restoration.
10. Contact route and useful Not Found recovery link.

Existing Artifact, import, search-maintenance, audit, assignment, delete-confirmation, search-state, highlight, runtime, and base-path tests must continue to pass. Modify accepted tests only when the user-visible contract intentionally changes, such as `Load more` becoming `Next page`.

Do not add snapshot tests for entire pages. Do not add a test for every translation key or CSS declaration. Do not install an accessibility test dependency solely for this increment.

## Focused verification

Follow `AGENTS.md`. Use focused checks during implementation and expand once at this coherent frontend and web-image boundary.

### During implementation

- Run the directly affected Vitest file after each coherent behavior group.
- Run `tsc -b` after shared component or generated-client integration changes.
- Run Nginx configuration syntax and container startup checks after web-image changes.
- Do not rerun backend, PostgreSQL, Meilisearch, import adapter, or generated-code tests.

### Boundary verification

Run once after the complete increment is coherent:

1. `git diff --check`.
2. Frontend lint and TypeScript check.
3. The complete frontend component test suite once.
4. Production frontend build for `/ause-discovery/`.
5. Production frontend build for `/`.
6. Build the web image for `/ause-discovery/` with no push.
7. Build the web image for `/` with no push.
8. Render `docker compose config --quiet` after the internal web-port change.
9. Start a disposable web container or disposable Compose web boundary and confirm:
   - the process user is `nginx` rather than root;
   - Nginx configuration passes `nginx -t`;
   - the configured internal unprivileged port listens;
   - Home and one nested SPA route return the entry document;
   - a fingerprinted asset retains immutable caching;
   - the entry document retains no-cache behavior;
   - CSP, no-sniff, referrer, and permissions headers appear on Home, nested fallback, and an asset response;
   - no HSTS header is emitted by the local HTTP stack.
10. Repeat only the minimal static-routing and header smoke for the root-path image.
11. Inspect the built HTML and JavaScript console for CSP violations only if a browser is already available. Absence of browser tooling is not a blocker when static policy and HTTP smoke checks pass.
12. Search tracked frontend source for removed starter asset references and obvious raw stable-key rendering in changed surfaces.

No backend test suite, generated drift check, vulnerability scan, Playwright run, clean-environment bootstrap, search rebuild, registry operation, or full product acceptance belongs in this increment. Escalate verification only when a focused failure exposes a cross-system issue.

Use disposable names for containers and images. Remove only resources created by this increment. Preserve active development and verification services and every persistent volume. Do not use broad Docker prune commands.

## Acceptance details

The following behaviors are mandatory:

- No route renders two main landmarks.
- The skip link says `Skip to main content` and reaches the focusable main region.
- Administrator routes retain the public site frame and their own secondary navigation.
- Every named route has a useful document title.
- Session expiry on a protected request returns to login with the intended path.
- Generic login failure does not disclose account validity.
- Public stable roles and statuses are human-readable.
- PDF View opens a new tab; Download does not become an inline preview control.
- List pagination never duplicates cursor pages or retains stale cursor history after filters change.
- Loading does not render a false empty state.
- mutation failures are visible and do not discard form input.
- Delete confirmation remains deliberate and keyboard operable.
- informational content is truthful and visibly pending where institutional approval is required.
- no fake branding, contact, legal, or developer values are committed.
- no Vite or React starter branding remains in tracked public assets.
- narrow layouts do not produce page-level horizontal scrolling for ordinary content.
- Nginx security headers survive locations that also set caching headers.
- local HTTP does not emit HSTS.
- the web process runs as `nginx` on an unprivileged internal port.
- root and default subpath builds remain functional.

## Completion criteria

The increment is ready for review only when all conditions below hold:

- Included route and state behavior is implemented without API or database changes.
- Existing accepted feature tests pass.
- New focused product-coherence tests pass.
- frontend lint, TypeScript, and both base-path builds pass.
- both web images build without publication.
- the non-root Nginx and security-header smoke passes for default and root paths.
- Compose renders with the new internal web port.
- starter assets are removed only after tracked-reference checks.
- `.memory/mvp_implementation_status.md` records implemented behavior and exact focused verification without declaring final technical MVP acceptance.
- `.memory/MEMORY.md` continues to name `27ecdd0` as the accepted review baseline until this implementation passes independent review. It may state that product coherence is submitted for review.
- `.memory/lessons_learned.md` changes only for a verified reusable failure pattern.
- no unrelated source, generated output, dependency, lockfile, CI, or backend changes are present.
- `.DS_Store` remains untracked and unstaged.
- one coherent implementation commit exists with a past-tense sentence, for example `Completed product coherence and web hardening`.

## Required handoff for review

Stop after the implementation commit. Do not begin final technical MVP verification.

The completion report must include:

- final commit hash and exact message;
- parent brief-checkpoint hash;
- concise behavior summary by shared shell, public routes, administrator routes, accessibility, styling, assets, and Nginx;
- every file changed, grouped by area;
- session-expiry integration design and proof that the interceptor is installed once;
- Project and Person paging/filter behavior;
- informational content and any unresolved launch-time value;
- exact Nginx security headers and internal port;
- proof of runtime process user;
- focused tests added or changed;
- every verification command and outcome;
- any failed approach, skipped check, environmental limitation, or unresolved defect;
- confirmation that no OpenAPI, generated, database, dependency, CI, publication, deployment, or external system state changed;
- final `git status --short` output;
- disposable Docker resources removed and persistent resources preserved.

The reviewing task will inspect the complete diff from `27ecdd0` through the submitted commit, confirm that the brief checkpoint and accepted implementation history remain intact, rerun focused checks where useful, and either accept the increment or return a bounded correction list. Final integrated technical MVP verification starts only after acceptance and checkpoint advancement.
