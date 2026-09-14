# Project Content import commands and explanations

This note covers both bulk Project Content paths:

- `artifacts seed-demo` attaches the same realistic mock Project Files to
  published Projects.
- `project-content import-manifest` imports individually mapped Project
  Logos, Project Files, and Repository Links from one JSON bundle.

Both commands attach content to Projects that already exist in PostgreSQL.
Project metadata must therefore be imported first from
`tools/ausesp-data-extractor/output/ause-discovery-projects-metadata-import.csv` through the
administrator Imports page.

## Quick copy: reset and verify B2

For a deliberate fresh-data rehearsal, empty B2 together with the disposable
PostgreSQL volumes. This is routine for a local-test bucket. Emptying the
production bucket is a deliberate maintainer decision, reserved for
data-model changes, storage-layout changes, or corruption recovery, and must
always run together with a production PostgreSQL reset. Load the application
environment into the B2 CLI without placing secret values directly in shell
history:

```sh
set -a
source .env.local-test
set +a

export B2_APPLICATION_KEY_ID="$AUSE_ARTIFACT_B2_KEY_ID"
export B2_APPLICATION_KEY="$AUSE_ARTIFACT_B2_APPLICATION_KEY"
```

Preview the removal, remove every object version, cancel unfinished multipart
uploads, and prove that the bucket is empty:

```sh
b2 rm \
  --versions \
  --recursive \
  --dry-run \
  "b2://$AUSE_ARTIFACT_B2_BUCKET"

b2 rm \
  --versions \
  --recursive \
  "b2://$AUSE_ARTIFACT_B2_BUCKET"

b2 file large unfinished cancel \
  "b2://$AUSE_ARTIFACT_B2_BUCKET"

b2 ls \
  --versions \
  --recursive \
  --long \
  "b2://$AUSE_ARTIFACT_B2_BUCKET"
```

An empty final listing confirms that no current or hidden object versions
remain. Bucket deletion must always be coordinated with PostgreSQL reset. The
relationship is described under Current reset model.

Before a bulk import, verify the storage provider in the running API rather
than relying only on the environment file:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  exec -T api printenv AUSE_ARTIFACT_STORAGE_BACKEND
```

Expected output for the B2 rehearsal is `b2`. An unexpected `local` value means
the API container was created with stale configuration or the Compose API
service is not passing the B2 variables. Correct the Compose environment and
recreate the stack before importing.

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
`/home/saiaike/apps/ause-discover/mock-artifacts/` on the current VM, then run
from `/home/saiaike/apps/ause-discover`:

```sh
docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps \
  --volume "$PWD/mock-artifacts:/bulk:ro" \
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

## Quick copy: real Project Content

The application command is ready to import each Project's real files, Logo,
and Repository Links in one run. It uses the same storage and PostgreSQL
association path as the mock seeder for byte-backed content, while links are
stored as Project metadata.

The extractor assembles the complete bundle in one command (from the
extractor repository root):

```sh
.venv/bin/python pipeline/build_import_bundle.py
```

This writes, or fully rebuilds, `resources/REAL_IMPORT_BUNDLE/` in the main
repository with this layout:

```text
REAL_IMPORT_BUNDLE/
  project-content-manifest.json
  logos/
  projects/
    sp-<identifier>/
      <real Project files>
```

The current bundle contains 205 Project entries, 68 Logos, 332 Project Files,
and 15 Repository Links across 13 Projects. The manifest uses the same
`sp-<identifier>` import keys as
`ause-discovery-projects-metadata-import.csv`, classifies every file with a
supported Project File type, references extracted files rather than a ZIP
that merely contains a report or poster, and carries each Project's Logo and
repository links alongside its files. The legacy `.doc` report and two
presentation decks mislabeled as `.pptx` posters are converted to PDF and
included under their correct types. Three award images remain excluded because
no current Project File type admits them, as recorded in the extractor's
`notes/bundle-building.md` and backlog item 19.

Do not seed mock files first on a clean deployment intended for real content.
Mock files create active Project File records and would coexist with, or cause
skips during, the later real import. Import the real combined bundle directly.

### Local stack

The following commands assume the completed bundle the extractor built at
`resources/REAL_IMPORT_BUNDLE/`.

Dry-run:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/resources/REAL_IMPORT_BUNDLE:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content-manifest.json \
  --actor-username Test1234567890 \
  --workers 4
```

Apply only after the complete plan succeeds:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/resources/REAL_IMPORT_BUNDLE:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content-manifest.json \
  --actor-username Test1234567890 \
  --workers 4 \
  --apply
```

### VM with B2 storage

The current VM deployment directory is
`/home/saiaike/apps/ause-discover`. Copy the complete bundle into its
`project-content/` staging directory from the local repository root:

```sh
rsync -avh --progress \
  resources/REAL_IMPORT_BUNDLE/ \
  saiaike@life.au.edu:/home/saiaike/apps/ause-discover/project-content/
```

The destination must contain `project-content-manifest.json`, `logos/`, and
`projects/` directly. Run the import commands from
`/home/saiaike/apps/ause-discover` on the VM.

Dry-run:

```sh
docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps \
  --volume "$PWD/project-content:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content-manifest.json \
  --actor-username <admin-username> \
  --workers 4
```

Apply:

```sh
docker compose -p ause-discovery --env-file .env \
  run --rm --no-deps \
  --volume "$PWD/project-content:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content-manifest.json \
  --actor-username <admin-username> \
  --workers 4 \
  --apply
```

With `AUSE_ARTIFACT_STORAGE_BACKEND=b2`, Project File and Logo bytes go to B2
and their associations go to the VM PostgreSQL database. The mounted bundle is
only the import source.

The verified 2026-09-13 dry-run reported `205` Projects, `68` Logo uploads,
`332` Project File uploads, `15` declared Repository Links, and `13` replaced
link sets. Apply reported `1965.9 MiB` planned bytes.

Wait for all `205` progress lines, the final `mode: apply` summary, no failed
Projects, and a zero exit status. Before deleting the source, rerun the dry-run
and confirm that matching byte-backed items and link sets are unchanged. Then
remove the staging copy without touching B2 or production volumes:

```sh
realpath /home/saiaike/apps/ause-discover/project-content
du -sh /home/saiaike/apps/ause-discover/project-content
rm -rf /home/saiaike/apps/ause-discover/project-content
```

The local `resources/REAL_IMPORT_BUNDLE/` remains the development source of
truth.

### Common VM command mistakes

- `-p ause-local-test --env-file .env.local-test` is the local command. The VM
  uses `-p ause-discovery --env-file .env`.
- `$PWD/resources/REAL_IMPORT_BUNDLE` is the local bundle path. The current VM
  mount is `$PWD/project-content`.
- A missing bind-mount source can still create an empty directory, after which
  the container reports `open /bundle/project-content-manifest.json: no such
  file or directory`.
- An accidental VM command using `-p ause-local-test` can leave an isolated
  network and named volumes. Remove only that accidental project with:

```sh
docker compose -p ause-local-test --env-file .env \
  down -v --remove-orphans
```

## Quick copy: extracted logos on the local stack

The combined bundle builder (previous section) is the primary path and its
manifest already carries every Logo. The logo-only export below remains
for one-off runs against a fresh Project metadata import. Write it outside
`resources/REAL_IMPORT_BUNDLE/`, for example into
`resources/logo-only-bundle/`, because the builder wipes and fully
rebuilds that directory. Run from the main repository root after Project
metadata import:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/resources/logo-only-bundle:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content-manifest.json \
  --actor-username Test1234567890 \
  --workers 4
```

Apply after the complete bundle plans successfully:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/resources/logo-only-bundle:/bundle:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api project-content import-manifest \
  --manifest /bundle/project-content-manifest.json \
  --actor-username Test1234567890 \
  --workers 4 \
  --apply
```

## Required order

The complete fresh-data sequence is:

1. Empty B2 only when PostgreSQL and local volumes will also be reset.
2. Remove the disposable Compose stack and volumes.
3. Build or pull the selected images.
4. Start PostgreSQL and Meilisearch.
5. Apply and inspect database migrations.
6. Validate and synchronize catalogs.
7. Create the administrator on a fresh database.
8. Start the complete stack and verify API readiness.
9. Confirm that the live API reports `AUSE_ARTIFACT_STORAGE_BACKEND=b2`.
10. Import and commit `ause-discovery-projects-metadata-import.csv` through the
    administrator Imports page.
11. Run the Project Content dry-run and compare its counts with the bundle.
12. Apply the same Project Content command.
13. Rerun the dry-run and confirm that matching content is skipped.
14. Check public Project pages, Logos, PDF views, and downloads.
15. Remove the VM staging bundle only after verification.

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
| `report` | `pdf`, `docx` |
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
the Logo first, Project Files afterward, and Repository Links last.

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

The logo-only manifest is generated on demand from the extractor's combined
record and the metadata CSV, filtered to the projects that actually appear
in the CSV (membership filter instead of any hardcoded project exclusion, so
it can never reference a Project without a database row). Run from the
extractor repository root:

```sh
mkdir -p ../../resources/logo-only-bundle

jq -n --rawfile csv output/ause-discovery-projects-metadata-import.csv \
  --slurpfile m output/extraction-evidence/manifest.json '
  ($csv | split("\n")
    | map(select(length > 0 and (startswith("import_key") | not)))
    | map(split(",")[0])) as $keys
  | {version: 1, projects: [$m[0] | to_entries[]
      | select(.value.logo.output != null)
      | select(("sp-" + .key) as $k | ($keys | index($k)))
      | {project_import_key: ("sp-" + .key),
         logo: {file_path: .value.logo.output},
         files: []}]}
' > ../../resources/logo-only-bundle/project-content-manifest.json
```

Copy `output/logos/` to `../../resources/logo-only-bundle/logos/` so the
manifest's relative paths resolve. The full bundle-builder script
(`pipeline/build_import_bundle.py`) is now the primary path and assembles
the complete bundle, Project Files and repository links included, in one
command. The jq export above remains the one-off logo-only fallback.

Historical note: an earlier revision hardcoded `select(.key != "2021")`
because the slide-only project sp-2021 had metadata missing at the time. The
2026-09-12 SP1/SP2 pass gave all three slide-only projects full metadata, so
that exclusion is gone and sp-2021 now carries both metadata and its Logo.

Data corrections belong in the extractor dataset records and generation
pipeline, rather than in a generated manifest.

## Repository links in the manifest

Each project may declare an authoritative `links` array beside `logo` and
`files`:

- omitted `links`: the Project's repository links stay unchanged;
- present `links` (including `[]`): the complete desired set, so an empty
  array removes every link;
- `null` is rejected;
- each link must explicitly carry `url`, `primary`, `availability`, and
  `checked_at` (RFC 3339); an omitted `primary` is rejected rather than
  read as false. URLs must be absolute HTTPS with a hostname, without
  credentials, fragments, or control characters, at most 2048 characters of
  the normalized form, unique after normalization, and a nonempty set needs
  exactly one primary link;
- availability is `accessible`, `not_accessible`, or `unverified`.

Dry-run and the summary report declared links plus replaced and unchanged
link sets, and progress lines read `links replaced (N repositories)`,
`links unchanged (N repositories)`, or `links failed` with the controlled
error. Apply rechecks every declared set under the transactional Project
lock, so a set changed after planning is restored and reported as
replaced. Links own no bytes and never enter byte totals, quotas, Project
File lists, or search. Within one Project apply runs logo, then files, then
links with a fresh revision read.

Build the repository-link manifest from the extractor evidence (extractor
repository root; only `kind == "project_repo"` survives, so third-party
references never reach the manifest, and projects missing from the metadata
CSV are filtered out the same way as the logo export). Write this one-off
manifest outside `REAL_IMPORT_BUNDLE` because the primary builder owns that
directory:

```sh
mkdir -p ../../resources/link-only-bundle

jq -n --rawfile csv output/ause-discovery-projects-metadata-import.csv \
  --slurpfile m output/extraction-evidence/manifest.json '
  ($csv | split("\n") | map(select(length > 0 and (startswith("import_key") | not))) | map(split(",")[0])) as $keys
  | [$m[0] | to_entries[]
     | select((("sp-" + .key)) as $k | ($keys | index($k)))
     | select(((.value.links // []) | map(select(.kind == "project_repo")) | length) > 0)
     | {project_import_key: ("sp-" + .key),
        links: [(.value.links // [])[] | select(.kind == "project_repo")
          | {url: .normalized,
             primary: (.primary // false),
             availability: (if (.liveness.status // "") == "public" then "accessible"
                            elif (.liveness.status // "") == "not_found" then "not_accessible"
                            else "unverified" end),
             checked_at: (.liveness.checked_at // "")}]}]
  | {version: 1, projects: .}
' > ../../resources/link-only-bundle/project-content-manifest.json
```

Verified 2026-09-12: the export produced exactly 13 Project entries and 15
links (13 accessible, 1 not accessible, 1 unverified), every timestamp valid,
every set carrying exactly one primary, and the guarded evidence test
planned all 13 entries through the real loader and planner. A link-only run
uses the same `ausectl project-content import-manifest` command. The bundle
builder merges these same `links` arrays into the combined manifest
automatically, so the jq export is only needed for one-off link-only runs.

## Temporary disk use

Metadata CSV and XLSX previews use `AUSE_IMPORT_TEMP_ROOT` in the application
volume. B2 Project File uploads use the container operating system temporary
directory while computing the digest, detected MIME type, and final byte count.
That B2 staging location is separate from `AUSE_IMPORT_TEMP_ROOT` and requires
enough free container disk for the largest single file being uploaded.

## Current reset model

The reviewed metadata CSV and Project Content bundle remain the source of
truth. Production data carries soft preservation: everything is rebuildable
from these files, but a full re-import costs roughly 10 to 15 minutes for
about 2 GB, so routine upgrades preserve the production PostgreSQL volume and
B2 objects instead of resetting them. A clean rebuild that deletes PostgreSQL
and local volumes and empties B2 is routine for `ause-local-test` and, for
production, a deliberate maintainer decision reserved for data-model changes,
storage-layout changes, or corruption recovery.

PostgreSQL and B2 must be considered together:

- Resetting PostgreSQL while preserving B2 leaves orphaned objects.
- Emptying B2 while preserving PostgreSQL leaves Project File and Logo rows
  whose bytes no longer exist.
- Reusing both preserves idempotent skip behavior.

Once administrators begin creating authoritative records outside these source
files, this soft reset model no longer applies and coordinated database plus
storage backups become necessary.
