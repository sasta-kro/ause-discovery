# Project Logo Loading Experience increment

## Assignment

Implement and commit native lazy loading plus a decoded Project Logo reveal
across public and administrator interfaces.

This is one bounded frontend increment. Keep the existing Project Logo storage,
database, API, URL, cache, import, and authorization contracts unchanged. Do not
introduce a CDN, direct B2 URLs, generated image derivatives, a service worker,
or a new image format in this increment.

Start only after Increment 15 is independently accepted. Its accepted commit,
plus this planning brief, becomes the baseline. Preserve unrelated working-tree
changes and existing behavior unless this brief explicitly changes it.

## Required reading before coding

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`, especially Project Logo and Artifact Store
4. `.memory/architectural_decision_logs.md`, especially AD-009 and AD-011
5. `.memory/implementation-status/completed.md`, especially former backlog item 21
6. `.memory/docs/increments/12-project-logo-integration.md`
7. `frontend/src/app-shell.tsx`
8. `frontend/src/features/admin/project-logo.tsx`
9. `frontend/src/identity.test.tsx` and the administrator Logo tests
10. `frontend/src/App.module.css`
11. `backend/internal/platform/httpserver/api.go`, limited to existing public
    and administrator Logo response behavior
12. This brief

## Problem

The current identity components insert a visible `<img>` as soon as a Logo URL
exists. Large PNGs can therefore appear progressively from top to bottom on a
cold B2-backed request. The fixed square prevents layout shift, but it does not
prevent partially decoded image content from being painted.

The current fallback already provides a useful loading state. Initials are
immediately recognizable, occupy the final footprint, and require no extra
network request. A spinner, blank skeleton, blurred preview, or low-quality
placeholder would add motion or visual noise without improving identification.

## Required outcome

- Initials remain fully visible while a Logo request or image decode is pending.
- A Logo becomes visible only after the browser reports a successful load and
  `HTMLImageElement.decode()` completes successfully.
- The decoded Logo replaces the initials with a short opacity transition.
- The existing initials identity is the loading placeholder. No separate
  spinner, skeleton, or loading graphic is introduced.
- Reduced-motion preferences disable the transition.
- Missing, failed, or undecodable Logos keep the initials fallback and never
  expose a broken-image icon.
- A changed versioned Logo URL resets the state to pending and cannot be settled
  by an asynchronous event from the previous URL.
- Search-result images use deliberate eager and lazy loading instead of giving
  every result the same network priority.
- The public Project header remains eager and receives high fetch priority
  because it is above the fold and may be the page's largest image.
- The administrator preview remains eager without competing for high priority.
- Existing fixed dimensions, accessibility behavior, and immutable-cache URLs
  remain unchanged.

## Product and architecture decision

Use a shared Project Logo identity component or a narrowly shared decoded-image
primitive instead of maintaining separate state machines in search, Project
detail, and administrator preview code. The abstraction is justified by the
same repeated lifecycle in three interfaces: absent, pending, ready, failed,
and reset after URL change.

Keep the application-owned URL contract:

```text
/ause-discovery/api/v1/projects/<project-id>/logo?v=<revision>
```

The existing response is public, versioned, and immutable for one revision.
That contract already gives repeat visits effective browser caching. Direct B2
URLs would bypass application-owned publication checks, expose storage
identifiers, and conflict with the accepted private-store boundary. Deleted or
superseded bytes are intentionally retained, so direct object access is not an
acceptable shortcut.

## Scope

### Included

- One reusable decoded Logo loading path.
- Initials underlay during loading and after failure.
- Hidden pending image that still participates in normal browser loading.
- Successful reveal only after load and decode.
- Short opacity transition with reduced-motion handling.
- URL-change reset and stale asynchronous completion protection.
- `decoding="async"` on Project Logo images.
- Context-specific `loading` and `fetchPriority` hints.
- Existing search-result, public Project-detail, and administrator-preview
  Logo locations.
- Focused component, style, lint, type, and production-build verification.
- A maintainer-run throttled-network browser smoke.
- One coherent implementation commit.

### Excluded

- Backend, database, OpenAPI, generated-client, storage-adapter, or import
  changes.
- Public or private B2 bucket policy changes.
- Direct browser access to B2 objects or presigned storage URLs.
- Cloudflare, another CDN, Nginx proxy caching, or DNS changes.
- VM-local image caching. The deployed host must not become a byte store.
- Resizing, recompressing, metadata stripping, WebP, AVIF, SVG conversion,
  `srcset`, or new stored Logo variants.
- JavaScript viewport observers when native image loading hints are sufficient.
- Image preloads, preconnects, or service-worker caching.
- BlurHash, dominant-color placeholders, blurred thumbnails, shimmer skeletons,
  or spinners.
- New Logo placement on Person pages. If Person Project lists do not currently
  render Logos, this increment does not add them.
- Search ordering, filter semantics, image publication, or deployment.
- Broad visual redesign or automated visual acceptance.

## Loading state contract

Use an explicit state equivalent to:

```text
absent  -> initials only
pending -> initials visible, image mounted but visually hidden
ready   -> decoded image visible over the initials
failed  -> initials only
```

The component must preserve these rules:

1. A nonempty `logoUrl` enters `pending`.
2. The image request starts while the initials remain visible.
3. The load handler awaits `image.decode()` before entering `ready`.
4. A request error or decode rejection enters `failed`.
5. A missing URL enters `absent` without mounting an image.
6. A URL change immediately returns to `pending` for the new URL.
7. A completion associated with an older URL cannot alter the current state.
8. A cached image that is already complete still reaches the decode path.
9. Unmounting prevents a pending decode promise from updating component state.

An opacity-zero image may remain mounted over the initials while pending. Do
not use `display: none` for the loading image. Keep the final dimensions fixed
through the existing identity footprint and size the image to that footprint.

The transition should be approximately 120 to 180 milliseconds. Apply the
transition only to opacity, not dimensions, transforms, filters, or position.
Under `prefers-reduced-motion: reduce`, remove the transition and reveal the
decoded image immediately.

## Loading priority contract

Use browser-native hints as hints rather than a custom scheduler.

- Public Project detail Logo: `loading="eager"`, `fetchPriority="high"`, and
  `decoding="async"`.
- Administrator Logo preview: `loading="eager"`, default fetch priority, and
  `decoding="async"`.
- Search results likely visible in the initial viewport: eager loading with
  default fetch priority.
- Remaining search results: `loading="lazy"`, low fetch priority, and
  `decoding="async"`.

Use the first four results as the bounded eager search cohort. Search ordering
already determines which Projects occupy those positions, so later newest-first
or relevance-first ordering changes require no Logo-loading branch. Every result
after the first four uses native lazy loading. Do not assign high fetch priority
to every Logo because that would remove the browser's ability to prioritize the
document, styles, fonts, and primary content.

Do not preload Logo URLs. Search Logo URLs are unavailable until the API response
arrives, and the Project detail Logo is supplementary rather than a required
render-blocking asset.

## Accessibility behavior

- Keep each Project Logo decorative with `alt=""` because adjacent Project
  text supplies the accessible name.
- Keep the containing identity footprint out of the accessibility tree as it
  is today.
- Do not announce Logo loading, success, or failure through live regions.
- Keep readable initials as the visual fallback throughout pending and failed
  states.
- Preserve usable colors and sizing in both identity variants.

## Component behavior and integration

Prefer a component such as `ProjectLogoIdentity` with explicit presentation and
priority props, or a smaller decoded-image child shared by the existing
`ProjectIdentity` and `ProjectDetailIdentity` wrappers. Avoid a generic image
framework. The implementation needs only the Project Logo lifecycle.

Search-result callers already have the result index and can select the eager
cohort without new global state. Project detail and administrator callers can
pass their fixed priority intent. The administrator's upload, replacement,
removal, validation, and revision-conflict behavior must remain unchanged.

The initials should stay rendered beneath the pending image. When state becomes
ready, the image surface may cover the initials. This prevents an empty flash
between data rendering and image decoding and makes decode failure a simple
return to the already-present fallback.

## Deliberately deferred image optimization

This increment improves scheduling and perceived loading only. The browser still
downloads the current full PNG for every requested Logo. Do not add downscaling,
cropping, recompression, alternate formats, stored derivatives, `srcset`, or an
image-transformation path.

A bounded display derivative remains a parked possibility in the improvement
backlog, not a committed future increment. No performance measurement or follow-up
implementation is required by this brief. CDN delivery is not planned for the
current product scale and must not enter this work.

## Focused verification

### Component tests

- A missing Logo URL renders initials and no image.
- A present Logo URL starts with initials visible and the image visually hidden.
- Successful load plus successful decode reveals the image.
- The image remains hidden until the decode promise settles.
- Request failure keeps initials and removes or permanently hides the failed
  image without a broken-image icon.
- Decode rejection behaves like request failure.
- A changed versioned URL returns the component to pending.
- A late load, decode, or error from the previous URL cannot settle the new URL.
- An already-complete cached image enters the decode path.
- Search callers assign the accepted eager cohort and lazy-load later entries.
- Project detail and administrator preview use their required priority hints.
- Decorative image and fixed-footprint behavior remain intact.
- Existing administrator upload, replace, remove, and preview-reset tests pass.

Mock `HTMLImageElement.decode()` explicitly in jsdom. Use controllable promises
for pending and stale-completion cases rather than timers.

### Static and build checks

- Run the focused identity and administrator Logo component tests.
- Run frontend lint once.
- Run `tsc -b` once.
- Run the default-subpath production build once.
- Inspect the focused diff for accidental API, generated-client, or backend
  changes.

### Maintainer-run browser smoke

The implementation handoff must provide a short manual procedure, not claim
visual acceptance:

1. Open a cold search page with browser cache disabled and apply a slow network
   throttle.
2. Confirm that initials remain visible until each complete Logo fades in.
3. Confirm that no Logo paints progressively from top to bottom.
4. Scroll and confirm that later Logo requests begin through native lazy loading.
5. Open one Project detail page and confirm that the header Logo begins eagerly.
6. Enable reduced motion and confirm that the decoded Logo appears without a
   transition.
7. Repeat with cache enabled and confirm warm immutable-cache behavior.

No automated visual check is required. Visual acceptance belongs to the
maintainer.

## Suggested implementation order

1. Add the focused shared Logo loading component or hook and explicit state
   contract.
2. Integrate search-result and public Project-detail identities.
3. Integrate the administrator Logo preview without changing mutation behavior.
4. Add opacity and reduced-motion styles.
5. Add focused lifecycle and priority tests.
6. Run static checks and the production build once.
7. Move backlog item 21 to `.memory/implementation-status/completed.md` after acceptance.
8. Commit the main repository once and stop.

## Completion criteria

- No Project Logo is visible before successful browser decode.
- Initials remain visible during every pending state and after every failure.
- URL changes and stale asynchronous events are safe.
- Fixed footprints prevent layout shift.
- Search uses a bounded eager cohort and native lazy loading for later entries.
- Detail and administrator contexts use appropriate priority hints.
- Reduced-motion preferences are respected.
- Existing immutable application URLs and private B2 architecture are untouched.
- Focused component, lint, type, and build checks pass.
- A maintainer-run throttled-network procedure is supplied without claiming its
  result.
- No CDN, derivative generation, publication, deployment, or broad redesign
  occurs.
- One coherent main-repository commit uses a lowercase past-tense message with
  no prefix and no co-author trailer.

## Required handoff

Stop after the implementation commit. Report:

- accepted Increment 15 baseline and final commit hash;
- component or hook boundary and loading state contract;
- stale URL and unmount protection;
- loading, fetch-priority, decode, and reduced-motion behavior by interface;
- focused test and build results;
- exact maintainer-run browser smoke procedure;
- any failed check, skipped check, or residual browser risk;
- confirmation that backend contracts, B2 policy, CDN, image derivatives,
  publication, and deployment did not change;
- final `git status --short`.

## Research basis

- MDN `HTMLImageElement.decode()` documents decoding before insertion or reveal
  to avoid an empty or partially prepared image:
  <https://developer.mozilla.org/en-US/docs/Web/API/HTMLImageElement/decode>
- MDN `HTMLImageElement` documents `decoding`, `fetchPriority`, and `loading`:
  <https://developer.mozilla.org/en-US/docs/Web/API/HTMLImageElement>
- web.dev documents native lazy loading and warns against lazy-loading images
  visible in the initial viewport:
  <https://web.dev/learn/performance/lazy-load-images-and-iframe-elements>
- web.dev documents fetch priority as a browser hint and recommends using high
  priority selectively:
  <https://web.dev/articles/fetch-priority>
