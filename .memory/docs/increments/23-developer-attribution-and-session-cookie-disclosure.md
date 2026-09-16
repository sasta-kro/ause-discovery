# Developer attribution and session-cookie disclosure increment

## Assignment

Add accurate, professional developer attribution to the shared footer and the
About page. Add one compact disclosure beside the administrator sign-in form
for the strictly necessary administrator authentication cookies. Replace the
public placeholder language with current Terms, Privacy, and Contact text based
on the settled product decisions.

This is a bounded frontend content and presentation increment. It implements
settled factual attribution and current technical behavior. It does not create
a binding software license, transfer copyright, assign ownership of Project
content, designate a legal operator or data controller, or publish terms that
claim an executed institutional software agreement.

Start from the current repository chain through `3bfbcd3`, whose accepted
implementation baseline is Increment 22 corrected through `7388627`. Preserve
all unrelated work. At brief-writing time, uncommitted landing-page edits
overlap `frontend/src/App.module.css`, `frontend/src/app-frame.test.tsx`,
`frontend/src/app-shell.tsx`, and `frontend/src/app/i18n.ts`. Implementation
must not overwrite, revert, or silently absorb those edits. Begin only after
the edits' ownership and commit boundary are clear, then stage and inspect the
Increment 23 changes precisely.

## Required reading

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`, especially Governance and attribution
4. `.memory/implementation-status/README.md`
5. `.memory/implementation-status/backlog.md`, especially item 26
6. `.memory/implementation-status/completed.md`, especially Increment 22
7. `frontend/src/app-shell.tsx`, limited to AppFrame, InformationalPage,
   LoginPage, and route composition
8. `frontend/src/app/i18n.ts`, limited to legal, administrator, and footer copy
9. `frontend/src/App.module.css`, limited to footer, prose, panel, and text
   hierarchy styles
10. `frontend/src/app-frame.test.tsx`
11. This brief

## Confirmed product and governance decisions

### Developer identity

- Public name: `Sai Aike Shwe Tun Aung`
- Public role: `Developer and maintainer`
- Public profile: `https://github.com/sasta-kro`
- Status: bachelor Computer Science student at Assumption University
- No official university employment or administrative title may be implied.
- No direct public email address is included in this increment.
- No public source-repository link is included in this increment. The GitHub
  destination is a profile link only.

### Software ownership direction

The settled direction is personal copyright in the independently developed
software plus a written institutional operating license for Assumption
University. In plain terms, copyright identifies ownership of the software,
while a license grants defined permission without transferring ownership. The
intended institutional permission will allow hosting, operation, reproduction,
backup, and reasonable security and continuity maintenance, including work by
an authorized successor maintainer.

Exact modification, redistribution, commercialization, termination,
attribution, and successor-maintainer rights still require a written agreement
and qualified Thai legal review. The website must not publish the intended
model as an executed fact. In particular, do not add a copyright notice,
license badge, `All rights reserved`, MIT license, custom public license, or a
claim that Assumption University owns or does not own the software.

### Responsibility boundary

- Sai Aike Shwe Tun Aung is credited for software design, development, and
  technical maintenance.
- Authorized university administrators make institutional content-policy and
  correction decisions.
- AUSE Discovery is hosted on university infrastructure with faculty
  authorization and support.
- Project metadata is transcribed and processed from source material supplied
  by authorized university administrators.
- The developer is not presented as the guarantor of source-report accuracy,
  the owner of submitted Project content, the sole institutional operator, or
  the institutional contact for correction and takedown decisions.

## Required public content

### Shared footer credit

Replace the current plain `AUSE Discovery` footer credit with:

```text
Developed and maintained by Sai Aike Shwe Tun Aung
```

Render `Sai Aike Shwe Tun Aung` as an external link to
`https://github.com/sasta-kro`. Keep the surrounding phrase as ordinary text.
The accessible link name must identify the person rather than expose the raw
URL. Follow the existing safe external-link convention. If a new browsing
context is used, include the appropriate `rel` protection.

The credit belongs in the existing shared footer, so it appears at the bottom
of the landing page and every other public and administrator route. Do not add
a second landing-page-only credit.

The credit should remain visually secondary to the page content and footer
navigation, but it must be readable, keyboard reachable, and visible at 200
percent zoom and narrow widths.

### About page structure

Replace the single-paragraph About page with a small structured page. Reuse the
existing informational-page visual language and add dedicated rendering only
where the richer About content requires it. Do not create a generic content
management system or parse markup from translation strings.

Use these sections and approved meanings:

#### About AUSE Discovery

Preserve the current archive description in natural public language:

```text
AUSE Discovery is an institutional archive of historical senior projects. Visitors can search public project metadata by people, academic context, and controlled classifications. Authorized administrators maintain the records, project files, and search state.
```

#### Development and maintenance

Use this text:

```text
AUSE Discovery is an independent software project designed, developed, and maintained by Sai Aike Shwe Tun Aung, a Computer Science student at Assumption University. The platform is hosted on university infrastructure with authorization and support from faculty administrators for the benefit of the university community.
```

Follow it with one ordinary external link:

```text
Developer GitHub profile
```

Present the name with the visible role `Developer and maintainer` near this
link. The layout may use a compact definition, byline, or small profile block,
provided the heading and prose remain the primary reading structure.

The destination is `https://github.com/sasta-kro`. Do not display an email
address, contact form, repository URL, employment title, or invented university
position.

#### Content and corrections

Use this text:

```text
Project metadata is transcribed and processed from source materials supplied by authorized university administrators. Content-policy and correction decisions are handled through authorized university administrators. Requests concerning project records, corrections, privacy, or institutional policy may be sent through the Contact page.
```

Do not direct content requests to the developer. The Contact page separates the
developer's technical contact route from the public VMES institutional contact.

### Administrator session-cookie disclosure

Place one short disclosure directly below the administrator sign-in form:

```text
Strictly necessary cookies are used to authenticate authorized administrators and protect administrative requests. They are not used for public tracking or advertising.
```

This is intentionally quiet supporting text. Add a dedicated style that is
compact, muted, and subordinate to the login description and controls. It must
remain readable and meet the existing color-contrast standard. Do not hide it,
remove it from the accessibility tree, reduce opacity below readable contrast,
or turn it into a banner, modal, checkbox, consent control, or repeated sitewide
notice.

The disclosure describes the existing administrator authentication behavior.
It does not authorize analytics. No public visitor cookie banner is needed in
this increment because no optional analytics or advertising cookie exists.

### Privacy, Terms, and Contact pages

Publish the current operating statements without exposing internal approval
status. Privacy describes the existing public records, absence of behavioral
analytics and advertising cookies, necessary administrator cookies, ordinary
hosting request data, operational purposes, retention basis, and the
institutional privacy framework. Terms records the settled educational and
non-commercial use boundary, content-rights boundary, access-control and abuse
rules, source-record accuracy limitation, and ability to correct or restrict
access. Contact separates technical questions through the developer's GitHub
profile from institutional requests through the public VMES contact address.

No public software-copyright claim, executed institutional operating license,
governing-law clause, liability allocation, or ownership claim over submitted
Project content ships in this increment.

## Interaction and accessibility contract

- The footer credit is present in the shared frame without changing landmark
  count or navigation labels.
- The external profile links are reachable by keyboard and have visible focus.
- Link text remains meaningful without surrounding prose.
- The About page retains one page-level `h1` and uses ordered `h2` section
  headings.
- Content reflows without horizontal page overflow at 360 CSS pixels and at
  200 percent zoom.
- Long footer text wraps naturally without colliding with footer navigation.
- The login notice remains associated by proximity with sign-in behavior but
  is not an alert, live region, form requirement, or consent prompt.
- No animation or motion is introduced.

## Scope

### Included

- Shared footer developer credit.
- GitHub profile link on the credited name.
- Structured About page with archive, developer, hosting-context, and
  content-stewardship sections.
- One compact administrator authentication-cookie disclosure below the sign-in
  form.
- Current public Privacy, Terms, Accessibility-contact, and Contact copy without
  internal approval-status messages.
- Focused frontend tests for exact public content, destinations, structure,
  route coverage, and disclosure placement.
- Relevant internal implementation-status updates.
- One coherent implementation commit.

### Excluded

- Backend, API, OpenAPI, generated-client, database, search, storage, import,
  authentication, session, or cookie-behavior changes.
- Microsoft Entra ID or Project File access-control changes.
- Telemetry, analytics, tracking, consent management, visitor identifiers, or
  cookie banners.
- Direct personal email address, contact form, or source-repository link.
- An official university title or employment claim.
- Public software copyright or license claims.
- Project-content ownership or republication-rights claims.
- A LICENSE file, bespoke legal agreement, MOU, or ADR.
- New libraries, packages, content frameworks, or routing changes.
- Publication or production deployment.

## Focused verification

### Shared-frame tests

- The landing page and at least one non-home route show the exact footer phrase.
- The credited name links to `https://github.com/sasta-kro`.
- The footer still has one named Footer navigation landmark.
- Existing footer links remain present and unchanged.
- The developer profile link is not incorrectly included inside the Footer
  navigation landmark unless the existing structure makes that grouping
  intentional and accessible. Preserve the preferred current split between
  credit and policy navigation.

### About-page tests

- `/about` has one `h1` named `About AUSE Discovery`.
- The three required sections and exact approved factual text render.
- The developer role, student status, faculty authorization context, and
  institutional content-stewardship boundary are present.
- `Developer GitHub profile` points to `https://github.com/sasta-kro`.
- No direct email address, source-repository link, university employment title,
  copyright symbol, or software-license claim appears.

### Administrator-login tests

- `/admin/login` renders the exact session-cookie disclosure below the sign-in
  form.
- The disclosure is ordinary supporting text with no alert, live-region,
  checkbox, or button semantics.
- Existing username, password, failed-login, redirect, and session behavior
  remains unchanged.

### Visual and responsive checks

Inspect Home, About, and administrator sign-in in both current themes or color
schemes supported by the application, if more than one exists. Check desktop,
360 CSS pixels, and 200 percent zoom. Confirm:

- the longer footer credit wraps cleanly;
- footer navigation does not overlap the credit;
- About headings and paragraphs have a clear reading order;
- both GitHub links have visible focus;
- the cookie disclosure is small and low emphasis without becoming illegible;
- no horizontal page overflow is introduced.

### Static checks

- Run the focused shared-frame and login tests.
- Run the complete frontend component suite once because AppFrame and the
  shared translation resource affect every route.
- Run frontend lint and `tsc -b`.
- Run the default-subpath production build.
- Run `git diff --check`.
- Inspect the complete diff for excluded backend, API, generated-client,
  dependency, authentication, telemetry, legal-agreement, publication, and
  deployment changes.

No backend suite or infrastructure stack is required because this increment
does not change those boundaries.

## Suggested implementation order

1. Resolve or commit the pre-existing overlapping landing-page edits without
   changing the existing intent.
2. Add focused failing assertions for the footer, About page, and login
   disclosure.
3. Add the translation keys and the smallest dedicated About rendering needed
   for semantic sections and external links.
4. Replace the footer credit while preserving footer navigation.
5. Add the quiet disclosure below the sign-in form.
6. Add minimal styles for About structure and the low-emphasis disclosure.
7. Run focused tests and responsive inspection.
8. Run the bounded frontend regression and static checks.
9. Mark the public-copy portion of item 26 complete while leaving the private
   institutional operating agreement open.
10. Commit once and stop.

## Completion criteria

- Sai Aike Shwe Tun Aung receives accurate shared-footer and About-page credit
  as Developer and maintainer.
- The public GitHub destination is the profile `sasta-kro`, with no public
  email or repository link.
- The About page clearly separates software development from institutional
  content stewardship and does not invent a university position.
- The administrator sign-in page carries the exact compact necessary-cookie
  disclosure without a banner or consent interaction.
- Current Privacy, Terms, Accessibility, and Contact pages contain settled
  public copy without internal approval-status messages.
- No ownership transfer, executed license, Project-content ownership, sole
  operator, or final institutional-policy claim is published.
- Existing authentication and all backend behavior remain unchanged.
- Focused, frontend regression, static, build, responsive, and accessibility
  checks pass.
- One lowercase past-tense commit with no prefix or co-author trailer contains
  the brief, implementation, tests, and status update.

## Required handoff

Stop after the implementation commit. Report:

- accepted Increment 22 baseline and final commit hash;
- handling of the pre-existing overlapping landing-page edits;
- exact footer and About attribution shipped;
- exact GitHub destinations and external-link behavior;
- exact content-stewardship wording;
- cookie-disclosure placement, visual treatment, and semantics;
- confirmation that no email, repository link, official title, copyright
  notice, or public software license was added;
- confirmation that public Privacy, Terms, and Contact placeholders were
  replaced while the private institutional operating agreement remains open;
- focused, full frontend, lint, typecheck, build, responsive, and accessibility
  results;
- any failed, skipped, or residual concern;
- confirmation that every excluded boundary remains unchanged;
- confirmation that item 26 remains open only for the private institutional
  operating agreement and later policy changes;
- final `git status --short`.

## Legal and policy references

These sources explain why the public implementation remains narrower than the
intended ownership and institutional-license model. They are reference material,
not legal advice and not public copy for the website.

- Thailand Department of Intellectual Property, Copyright:
  `https://www.ipthailand.go.th/th/copyright.html`
- Thailand Copyright Act, including computer programs, employee works,
  commissioned works, and licensing rights:
  `https://www.ipthailand.go.th/images/781/ActCopyright_update.pdf`
- Thailand PDPA text published by ETDA:
  `https://www.etda.or.th/getattachment/e820df2c-848f-4e03-86cb-a9dad38cc713/ENG-Version.aspx`
- Thailand PDPC cookie-policy example distinguishing necessary and optional
  cookies:
  `https://gppc.pdpc.or.th/cookie-policy/`
