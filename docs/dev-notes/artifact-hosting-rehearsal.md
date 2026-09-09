# Artifact hosting rehearsal

How artifacts work end to end, and a seeded mock project for clicking through
the same flow you will run on the target VM. Verified on the local test stack
on 2026-09-09.

## The short version

1. Artifacts are uploaded bytes, not links. The admin picks a type (report,
   slides, source_code, proposal, poster, dataset, demo_video, other) and a
   file whose extension the type allows, plus a display name.
2. The API sniffs the content (magic bytes and MIME must match the extension,
   executables and HTML/SVG are rejected), streams the bytes to disk under
   `AUSE_ARTIFACT_ROOT`, and records the row with an opaque storage key.
3. Publishing requires the parent project to have at least one student, one
   advisor, one category, and one platform. Artifacts on a draft project are
   invisible publicly.
4. Public pages get two URLs per artifact: `/view` (inline, PDF only) and
   `/download` (attachment, safe filename). View supports byte ranges, which
   is what makes the in-browser PDF reader work.

## Where the bytes live

Inside the api container:

```
AUSE_ARTIFACT_ROOT=/var/lib/ause-discovery/artifacts
```

Files are content addressed and sharded, with no extension:

```
/var/lib/ause-discovery/artifacts/v1/<2 hex>/<2 hex>/<uuidv7>
/var/lib/ause-discovery/imports/          import preview spool (AUSE_IMPORT_TEMP_ROOT)
```

Staging happens in `<root>/.tmp` and is finalized with an atomic rename, so a
crashed upload never leaves a half-written file under `v1/`.

In Compose the whole `/var/lib/ause-discovery` tree is the named volume
`artifact-data`. On the target VM that means:

- The default named volume keeps working with no changes. Find it on the host
  under `/var/lib/docker/volumes/<project>_artifact-data/_data`.
- If you want a predictable host path (easier backup and inspection), replace
  the volume with a bind mount in an override file:

```yaml
services:
  api:
    volumes:
      - /srv/ause-discovery/artifacts:/var/lib/ause-discovery
```

- Back up that directory together with PostgreSQL. The files are rebuildable
  only from the originals, not from the database.
- Limits: `AUSE_MAX_ARTIFACT_BYTES` per file (default 250 MB) and
  `AUSE_MAX_PROJECT_ARTIFACT_BYTES` per project (default 2 GB).

## Type and extension allowlist

```
report      pdf
slides      pdf ppt pptx
source_code zip tar.gz tgz
proposal    pdf docx
poster      pdf png jpg jpeg
dataset     zip csv tsv json xlsx
demo_video  mp4 webm
other       pdf docx pptx zip csv xlsx json
```

There is no external link artifact type. A GitHub repo can be attached today
only as a `source_code` archive, or mentioned in project text. If mock links
are needed as first class objects, that is a schema and API change (a new
artifact type carrying a URL instead of bytes), recorded as a backlog item.

## The seeded mock project

Created through the real API on the local test stack (admin
Test1234567890):

- Project: Artifact Rehearsal Mock Project, published, 2025 second semester,
  one mock student (student id 9990001), one mock advisor, category Software
  or Application, platform Web.
- Four artifacts: report (pdf), slides (pdf), poster (png), source_code (zip).
- Mock files plus the generator live in `resources/mock-artifacts/` (git
  ignored). `make-mock-artifacts.py` regenerates them with stdlib only.

Click through: open `/ause-discovery/` on the web port, search "Artifact
Rehearsal", open the project. The report and slides show a View button (opens
the PDF inline in a new tab) and all four show Download. The poster and zip
have no View action by design, since inline view is PDF only.

## Bulk rehearsal and later corpus import

The `ausectl artifacts seed-demo` command attaches these same four files to
every published Project with realistic public display names. The
`ausectl artifacts import-manifest` command maps later corpus files by Project
UUID or metadata `import_key`. Both commands provide a read-only dry-run and
safe rerun behavior. Complete local and VM commands are documented in
`docs/operations/project-file-import.md`.

## API flow used (for scripting the VM seed)

```
POST /admin/auth/login                      session cookie
GET  /admin/auth/csrf                       X-CSRF-Token for mutations
GET  /catalogs                              program, course, taxonomy ids
POST /admin/projects                        draft (rev 1)
POST /admin/projects/{id}/artifacts         multipart per file,
                                           expected_project_revision each time
POST /admin/people                          student and advisor persons
PUT  /admin/projects/{id}                   flat ReplaceProjectRequest with
                                           participations and taxonomy_values
POST /admin/projects/{id}/publish           {"expected_revision": N}
```

Gotchas hit during seeding, so you do not hit them again:

- Every artifact upload bumps the project revision. Fetch the current
  revision before each upload and each publish.
- The PUT body is flat. There is no nested draft object.
- Publish fails with a database validation error until the project has a
  student, an advisor, a category, and a platform.
- Mutations need both the session cookie and the CSRF header.
