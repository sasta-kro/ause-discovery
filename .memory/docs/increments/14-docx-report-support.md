# DOCX report support increment

## Assignment

Implement and commit DOCX support for the `report` Artifact type while
preserving PDF as the only inline-viewable report format.

This is a small bounded Artifact-validation increment. Start from the planning
commit containing this brief, whose parent accepted baseline is `11d3b3a`.
Preserve unrelated working-tree changes and existing behavior unless this
brief explicitly changes it.

## Required reading before coding

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`
4. `.memory/mvp_implementation_status.md`, especially backlog item 11
5. `.memory/docs/ause-discovery_implementation_specification_v1.md`, Artifact
   allowlist and serving sections
6. `backend/internal/artifacts/validation.go`
7. `backend/internal/artifacts/inspection.go`
8. `backend/internal/artifacts/service.go`
9. `backend/internal/platform/httpserver/api.go`, especially Artifact response
   and inline-serving behavior
10. `backend/internal/artifacts/validation_test.go`
11. `backend/internal/artifacts/inspection_test.go`
12. `backend/internal/platform/httpserver/api_test.go`
13. `backend/internal/projectcontentimport/`
14. `.memory/sasta-dev-notes/project-content-import.md`
15. This brief

## Current limitation

`backend/internal/artifacts/validation.go` permits only `.pdf` for the
`report` type. The same validation path is used by administrator uploads,
replacement uploads, demonstration and Project Content planning, and local or
B2 persistence. Fourteen source reports in the reviewed corpus are Word DOCX
documents, so they cannot currently be represented as reports without either
converting them or misclassifying the original files.

DOCX already has an accepted content-detection path for Artifact types such as
`proposal` and `other`: the extension is allowlisted, the detected content must
be a ZIP-compatible Office container, active web and executable content remain
rejected, normal file and Project quotas apply, and macro-enabled `.docm`
remains unsupported.

## Required outcome

- `report` accepts `.pdf` and `.docx`.
- PDF reports retain both View and Download actions.
- DOCX reports expose Download only and are never served through the inline
  View route.
- Administrator upload, replacement, and Project Content import all inherit
  the behavior from the shared Artifact validation path.
- No new Artifact type, conversion service, document viewer, schema change, or
  compatibility branch is introduced.

The preferred operational practice remains:

1. Convert a DOCX report to PDF where practical.
2. Import the PDF as the canonical `report` for browser viewing.
3. Import the submitted DOCX as another `report` when retaining original bytes
   matters.

Multiple active reports are already supported. This increment must not invent
replacement or deduplication rules based only on the Artifact type.

## Scope

### Included

- Add `docx` to the shared extension allowlist for `report`.
- Update the internal implementation specification and Project Content
  operator guide.
- Focused validation and content-inspection tests using a minimal generated
  DOCX-compatible ZIP fixture.
- Focused response and serving tests proving DOCX is download-only.
- One focused Project Content planning test proving a DOCX report entry plans
  successfully under the normal `report` type.
- Memory status update after verification.
- One coherent implementation commit.

### Excluded

- Server-side DOCX-to-PDF conversion.
- LibreOffice, Microsoft Office, document preview services, or new runtime
  dependencies.
- Inline DOCX rendering or third-party Office viewer URLs.
- Allowing legacy `.doc`, `.docm`, RTF, ODT, or arbitrary ZIP files as reports.
- MIME-sniffing redesign or deeper OOXML package validation than the existing
  accepted DOCX path.
- New API fields, OpenAPI generation, database migrations, frontend layout
  changes, or search changes.
- Corpus extraction, file conversion, Project Content bundle generation, live
  B2 upload, stack rebuild, image publication, or VM deployment.
- Storage terminology work or unrelated Artifact storage corrections.

## Validation contract

Change the `report` entry in the existing extension map from PDF-only to PDF
plus DOCX. Reuse the current DOCX detection branch. Do not duplicate upload
validation in the CLI, Project Content importer, or HTTP layer.

The following must hold:

- `.docx` matching the existing ZIP/Office detection contract is accepted as
  `report`.
- `.DOCX` follows the existing case-insensitive extension normalization.
- `.doc`, `.docm`, `.zip`, and extensionless files remain rejected as
  `report`.
- A file named `.docx` whose detected content does not match the existing DOCX
  content rule is rejected.
- Executable, HTML, SVG, and script content remains rejected regardless of its
  filename.
- Existing maximum file size and active Project quota rules remain unchanged.

No extra content-signature implementation is required solely for reports. If
focused testing reveals that the existing DOCX path accepts content that its
current tests claim to reject, report that existing gap rather than broadening
this increment into an OOXML validator rewrite.

## Public and administrator behavior

Artifact response construction already grants `view_url` only when both the
stored MIME type and extension identify PDF. Preserve that rule.

For an active DOCX report:

- `download_url` is present for published public access and authenticated
  administration as applicable;
- `view_url` is absent;
- the download response uses attachment disposition and the sanitized original
  DOCX filename;
- direct access to the Artifact View route is rejected through the existing
  non-PDF behavior;
- the frontend naturally renders Download without View from the response and
  requires no DOCX-specific branch.

A report's semantic type does not imply that every stored representation is
browser-viewable. Viewability remains a property of the concrete validated
file format.

## Focused verification

Add or adjust tests proving:

1. `report.pdf` remains accepted.
2. A minimal valid DOCX-compatible fixture is accepted as `report`.
3. Uppercase `.DOCX` is normalized and accepted.
4. `.doc`, `.docm`, `.zip`, and mismatched `.docx` content are rejected as
   reports.
5. Unsafe active content remains rejected when named `.docx`.
6. Artifact response mapping gives DOCX Download but no View URL.
7. The View boundary rejects a DOCX report while Download returns the exact
   bytes with attachment disposition and a safe filename.
8. Project Content dry-run planning accepts one `report` entry whose source and
   original filename are DOCX.
9. Existing PDF View and Download behavior remains unchanged.

Use an in-test ZIP fixture created with the standard library rather than
committing a binary Office document solely for tests.

Run only focused checks:

- `go test` for `backend/internal/artifacts`;
- focused `backend/internal/platform/httpserver` tests needed for response and
  serving behavior;
- focused `backend/internal/projectcontentimport` tests;
- `go vet` for touched packages;
- tracked `gofmt` and `git diff --check`.

Do not run Playwright, the complete frontend suite, a full product acceptance
pass, live B2 operations, stack rebuilds, or deployment.

## Documentation and memory

After focused verification:

- change the internal implementation specification's `report` allowlist to
  `.pdf`, `.docx`, with PDF-only inline view;
- change the Project Content operator guide's allowed-extension table;
- update backlog item 11 in `.memory/mvp_implementation_status.md` to
  implemented with concise verification evidence;
- add a lesson only if a new durable failure pattern is discovered.

No public documentation change is required.

## Suggested implementation order

1. Add focused failing validation tests.
2. Extend the shared report allowlist.
3. Add response and HTTP boundary proofs for download-only behavior.
4. Add the focused Project Content planning proof.
5. Run the bounded verification set once.
6. Update internal specification, operator guide, and implementation status.
7. Commit once and stop.

## Completion criteria

- PDF and DOCX are both valid `report` formats.
- Only PDF receives an inline View URL.
- DOCX downloads retain exact bytes and a safe original filename.
- Administrator upload, replacement, and Project Content import use the same
  shared validation behavior.
- No conversion runtime, viewer, schema, OpenAPI, search, or frontend branch is
  added.
- Focused checks pass and unrelated files remain untouched.
- One coherent commit uses a lowercase past-tense message with no prefix and no
  co-author trailer.

## Required handoff

Stop after the implementation commit. Report:

- commit hash and exact message;
- files changed;
- exact allowlist change;
- DOCX detection and safety behavior reused;
- View and Download behavior for PDF and DOCX;
- Project Content planning result;
- commands and test results;
- failures encountered and residual limitations;
- confirmation that no schema, OpenAPI, frontend, storage-provider, live B2,
  stack, image, or VM work occurred;
- final `git status --short`.
