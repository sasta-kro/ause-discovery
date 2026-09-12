# Project Content import commands and explanations

This note covers both bulk Project Content paths:

- `artifacts seed-demo` attaches the same realistic mock Project Files to
  published Projects.
- `project-content import-manifest` imports individually mapped Project Logos
  and Project Files from one JSON bundle.

Both commands attach content to Projects that already exist in PostgreSQL.
Project metadata must therefore be imported first from
`tools/ausesp-data-extractor/output/reviewed-import.csv` through the
administrator Imports page.

## Quick copy: mock Project Files on the local stack

Run the dry-run first from the main repository root:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/resources/mock-artifacts:/bulk:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api artifacts seed-demo \
  --source-directory /bulk \
  --actor-username Test1234567890 \
  --all-published \
  --workers 4
```

Apply the same plan after the counts look correct:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/resources/mock-artifacts:/bulk:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api artifacts seed-demo \
  --source-directory /bulk \
  --actor-username Test1234567890 \
  --all-published \
  --workers 4 \
  --apply
```

Use `--project-id <project-uuid>` instead of `--all-published` to test one
published Project.

## Quick copy: mock Project Files on the VM

Copy `resources/mock-artifacts/` to
`/srv/ause-discovery/mock-artifacts/` on the VM, then run:

```sh
docker compose -p ause-discovery --env-file .env.production \
  run --rm --no-deps \
  --volume /srv/ause-discovery/mock-artifacts:/bulk:ro \
  --entrypoint /usr/local/bin/ausectl \
  api artifacts seed-demo \
  --source-directory /bulk \
  --actor-username <admin-username> \
  --all-published \
  --workers 4
```

Add `--apply` after the dry-run counts are correct. With B2 selected, the
command uploads the mock bytes to B2 and writes their Project associations to
the VM PostgreSQL database.

## Quick copy: real Project Files and Logos

The application command is ready to import each Project's real files and Logo
in one run. It uses the same storage and PostgreSQL association path as the
mock seeder.

The current source material is not yet an import-ready bundle. At present:

- `tools/ausesp-data-extractor/output/enrichment/logo-manifest.json` contains
  Logo entries but no Project Files.
- `resources/all-sp-projects/` contains the raw Project material. Some reports
  and posters are inside ZIP containers, and no combined
  `project-content.json` maps those files to Project import keys and Project
  File types.

Before these commands are run, the extractor must produce one portable bundle
with this layout or an equivalent layout:

```text
project-content/
  project-content.json
  logos/
  projects/
    sp-<identifier>/
      <real Project files>
```

The manifest must use the same `sp-<identifier>` import keys as
`reviewed-import.csv`, classify every file with a supported Project File type,
and reference extracted files rather than a ZIP that merely contains a report
or poster. The manifest can contain a Logo, Project Files, or both for each
Project.

Do not seed mock files first on a clean deployment intended for real content.
Mock files create active Project File records and would coexist with, or cause
skips during, the later real import. Import the real combined bundle directly.

### Local stack

The following commands assume the completed bundle is at
`tools/ausesp-data-extractor/output/project-content/`.

Dry-run:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/tools/ausesp-data-extractor/output/project-content:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content.json \
  --actor-username Test1234567890 \
  --workers 4
```

Apply only after the complete plan succeeds:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/tools/ausesp-data-extractor/output/project-content:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content.json \
  --actor-username Test1234567890 \
  --workers 4 \
  --apply
```

### VM with B2 storage

Copy the complete bundle to `/srv/ause-discovery/project-content/` on the VM.
The directory must contain the manifest and every relative path it references.

Dry-run:

```sh
docker compose -p ause-discovery --env-file .env.production \
  run --rm --no-deps \
  --volume /srv/ause-discovery/project-content:/bundle:ro \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content.json \
  --actor-username <admin-username> \
  --workers 4
```

Apply:

```sh
docker compose -p ause-discovery --env-file .env.production \
  run --rm --no-deps \
  --volume /srv/ause-discovery/project-content:/bundle:ro \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content.json \
  --actor-username <admin-username> \
  --workers 4 \
  --apply
```

With `AUSE_ARTIFACT_STORAGE_BACKEND=b2`, Project File and Logo bytes go to B2
and their associations go to the VM PostgreSQL database. The mounted bundle is
only the import source and can be removed after a successful import.

## Quick copy: extracted logos on the local stack

The extractor currently provides a logo-only manifest beside its `logos/`
directory. Run from the main repository root after Project metadata import:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/tools/ausesp-data-extractor/output/enrichment:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/logo-manifest.json \
  --actor-username Test1234567890 \
  --workers 4
```

Apply after the complete bundle plans successfully:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/tools/ausesp-data-extractor/output/enrichment:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/logo-manifest.json \
  --actor-username Test1234567890 \
  --workers 4 \
  --apply
```

## Required order

The complete fresh-data sequence is:

1. Apply database migrations.
2. Synchronize catalogs.
3. Create the administrator.
4. Start the complete stack.
5. Import and commit `reviewed-import.csv` through the administrator Imports
   page.
6. Run a Project Content or demonstration dry-run.
7. Apply the same content command.
8. Check public Project pages, logos, PDF views, and downloads.
9. Rerun the same command and confirm that matching content is skipped.

The metadata import creates Projects and stores each source `import_key` in
Project metadata. The content manifest then uses that key to locate the correct
Project. Importing content before metadata fails during planning because no
matching Project exists.

## How the database and B2 stay connected

Directly copying files into B2 is insufficient. B2 stores opaque bytes and has
no knowledge of Projects. The importer performs the association:

```text
manifest Project identity
  -> existing PostgreSQL Project
  -> validated source file
  -> B2 object with an opaque storage key
  -> PostgreSQL Project File or Project Logo row
       stores Project ID, storage provider, storage key, digest, and metadata
```

Public viewers ask the Backend API for a Project File or Logo. The API reads
the PostgreSQL row, selects its recorded storage provider and key, reads the B2
object, and streams the bytes. The B2 bucket remains private and public users
never need B2 credentials.

## Demonstration seeding

`artifacts seed-demo` expects these source files:

| Source file | Project File type | Public name | Download name |
|---|---|---|---|
| `mock-report.pdf` | `report` | Final report | `final-report.pdf` |
| `mock-slides.pdf` | `slides` | Presentation slides | `presentation-slides.pdf` |
| `mock-poster.png` | `poster` | Project poster | `project-poster.png` |
| `mock-source-code.zip` | `source_code` | Source code | `source-code.zip` |

Every targeted Project receives its own four Project File records and storage
objects. Two hundred Projects therefore produce about eight hundred B2 objects,
even though the source bytes are identical. The current design does not
deduplicate content across Projects.

The command skips a type when that Project already has any active file of the
same type. A repeated run therefore avoids duplicate reports, slides, posters,
or source archives. Replacing or removing an existing Project File remains an
administrator action.

The mock files use ordinary public labels and exercise the same public serving
path as real files. Public PDF files receive View and Download actions. Other
allowed files receive Download.

## Unified Project Content manifest

`project-content import-manifest` accepts a strict JSON document:

```json
{
  "version": 1,
  "projects": [
    {
      "project_import_key": "sp-1703",
      "logo": {
        "file_path": "logos/1703.png"
      },
      "files": [
        {
          "artifact_type": "report",
          "display_name": "Final report",
          "original_filename": "final-report.pdf",
          "file_path": "projects/sp-1703/final-report.pdf"
        },
        {
          "artifact_type": "slides",
          "display_name": "Presentation slides",
          "original_filename": "presentation-slides.pdf",
          "file_path": "projects/sp-1703/presentation-slides.pdf"
        }
      ]
    }
  ]
}
```

Every `file_path` is resolved relative to the directory containing the
manifest. Keeping the manifest, `logos/`, and `projects/` under one directory
creates a portable bundle for local and VM imports.

Each Project entry sets exactly one identity:

- `project_import_key` uses the metadata import key, normally `sp-<identifier>`.
- `project_id` uses the canonical Project UUID when an import key is not
  appropriate.

`reference_code` is a retained internal legacy value and is not an import
identity.

The loader rejects unknown fields, trailing JSON, a version other than `1`,
blank values, duplicate Project identities, and entries containing neither a
logo nor files. Different identity forms that resolve to the same Project are
also rejected before any write.

## Validation rules

Planning validates the entire manifest before uploads begin:

- Every Project identity must resolve.
- Every source path must remain inside the bundle and must not traverse a
  symlink.
- Logos must be fully decodable PNG files, at most 2 MiB and no larger than
  1600 by 1600 pixels.
- Project Files must satisfy the type, filename-extension, detected-content,
  size, and executable-content rules.
- The resulting active Project File total must stay within the per-Project
  quota.
- The named audit actor must be an active administrator.

Common Project File extensions are:

| Type | Allowed extensions |
|---|---|
| `report` | `pdf` |
| `slides` | `pdf`, `ppt`, `pptx` |
| `source_code` | `zip`, `tar.gz`, `tgz` |
| `proposal` | `pdf`, `docx` |
| `poster` | `pdf`, `png`, `jpg`, `jpeg` |
| `dataset` | `zip`, `csv`, `tsv`, `json`, `xlsx` |
| `demo_video` | `mp4`, `webm` |
| `other` | `pdf`, `docx`, `pptx`, `zip`, `csv`, `xlsx`, `json` |

The Backend API detects actual content rather than trusting the extension.
Executable formats and active web content are rejected.

## Dry-run, apply, progress, and recovery

Both bulk commands default to dry-run. Dry-run resolves Projects, opens and
validates files, checks quotas, and reports the plan without writing database
rows or storage objects. `--apply` enables writes.

`--workers` accepts 1 through 8 and defaults to 4. It controls how many
Projects run concurrently. Content within one Project stays sequential, with
the Logo first and Project Files afterward.

Apply prints one progress line per completed Project. The line contains the
Project title and UUID plus each uploaded, skipped, or failed item. This gives
visible progress without printing one line for every file.

Successful items commit as the run proceeds. A later failure does not roll
back earlier Projects. Recovery consists of fixing the cause and rerunning the
same command:

- A matching active Logo digest is skipped.
- A matching active Project File is skipped.
- Missing content uploads normally.
- A storage-provider outage stops further assignment and returns a nonzero
  exit status.

This behavior makes interrupted or repeated imports safe without requiring a
separate resume file.

## Building the logo manifest from extractor output

The extractor's `steps/apply_sp_pass.py` regenerates
`output/enrichment/logo-manifest.json` on every CSV rebuild, filtered to the
projects that actually appear in `reviewed-import.csv`. Prefer that script;
the jq below is the manual equivalent (membership filter instead of any
hardcoded project exclusion, so it can never reference a Project without a
database row):

```sh
jq -R -s 'split("\n") | map(select(startswith("sp-"))) | .[0:-1] as $keys
  | (input | to_entries) as $m
  | {version: 1, projects: [$m[]
      | select(.value.logo.output != null)
      | select((.key | "sp-" + .) as $k | $keys | index($k))
      | {project_import_key: ("sp-" + .key),
         logo: {file_path: .value.logo.output},
         files: []}]}' \
  <(cut -d, -f1 output/reviewed-import.csv) output/enrichment/manifest.json \
  > output/enrichment/logo-manifest.json
```

Historical note: an earlier revision hardcoded `select(.key != "2021")`
because the slide-only project sp-2021 had metadata missing at the time. The
2026-09-12 SP1/SP2 pass gave all three slide-only projects full metadata, so
that exclusion is gone and sp-2021 now carries both metadata and its Logo.

The generated manifest belongs beside the referenced `logos/` directory. Data
corrections belong in the extractor ground-truth files and generation pipeline,
rather than in a generated manifest.

## Temporary disk use

Metadata CSV and XLSX previews use `AUSE_IMPORT_TEMP_ROOT` in the application
volume. B2 Project File uploads use the container operating system temporary
directory while computing the digest, detected MIME type, and final byte count.
That B2 staging location is separate from `AUSE_IMPORT_TEMP_ROOT` and requires
enough free container disk for the largest single file being uploaded.

## Current reset model

During development, the reviewed metadata CSV and Project Content bundle are
the source of truth. A clean rebuild can delete PostgreSQL and local volumes,
empty B2, then repeat both imports.

PostgreSQL and B2 must be considered together:

- Resetting PostgreSQL while preserving B2 leaves orphaned objects.
- Emptying B2 while preserving PostgreSQL leaves Project File and Logo rows
  whose bytes no longer exist.
- Reusing both preserves idempotent skip behavior.

Once administrators begin creating authoritative records outside these source
files, this reset model no longer applies and coordinated database plus storage
backups become necessary.
