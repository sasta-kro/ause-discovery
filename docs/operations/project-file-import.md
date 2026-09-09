# Bulk Project-file import

The `ausectl artifacts` commands attach files to existing Projects through the same validation, storage, quota, revision, search-synchronization, and audit boundaries as administrator uploads. Project metadata must exist before Project files are imported. Direct database inserts and direct copies into `AUSE_ARTIFACT_ROOT` are unsupported.

Both commands default to a read-only dry-run. Add `--apply` only after the dry-run succeeds. An active administrator username is required so every uploaded Artifact has an audit actor.

## Demonstration files for every published Project

The demonstration command expects exactly these source files:

| Source filename | Artifact type | Public display name | Download filename |
|---|---|---|---|
| `mock-report.pdf` | `report` | Final report | `final-report.pdf` |
| `mock-slides.pdf` | `slides` | Presentation slides | `presentation-slides.pdf` |
| `mock-poster.png` | `poster` | Project poster | `project-poster.png` |
| `mock-source-code.zip` | `source_code` | Source code | `source-code.zip` |

The public names and download filenames intentionally resemble ordinary Project files for end-to-end showcase testing. The source files may be identical across Projects. Each Project receives independent Artifact metadata and stored content.

The command skips a demonstration type when the Project already has any active file of that type. Repeating the command therefore does not add another report, slide deck, poster, or source archive.

### Local test stack

Rebuild the API image after adding or updating the command:

```sh
docker compose -p ause-local-test --env-file .env.local-test build api
```

Run the read-only preview from the repository root:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/resources/mock-artifacts:/bulk:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api artifacts seed-demo \
  --source-directory /bulk \
  --actor-username Test1234567890 \
  --all-published
```

Review the Project and planned-upload counts, then apply the same plan:

```sh
docker compose -p ause-local-test --env-file .env.local-test \
  run --rm --no-deps \
  --volume "$PWD/resources/mock-artifacts:/bulk:ro" \
  --entrypoint /usr/local/bin/ausectl \
  api artifacts seed-demo \
  --source-directory /bulk \
  --actor-username Test1234567890 \
  --all-published \
  --apply
```

Replace `--all-published` with `--project-id <project-uuid>` to rehearse against one published Project.

### Target VM

Copy the four demonstration source files to a temporary operator directory on the VM, such as `/srv/ause-discovery/project-file-import`. Do not copy them into the Artifact storage volume.

Run the same dry-run and apply commands with the deployment environment file and a read-only bind mount:

```sh
docker compose --env-file <file> \
  run --rm --no-deps \
  --volume /srv/ause-discovery/project-file-import:/bulk:ro \
  --entrypoint /usr/local/bin/ausectl \
  api artifacts seed-demo \
  --source-directory /bulk \
  --actor-username <admin-username> \
  --all-published
```

Add `--apply` after the dry-run succeeds. The API service writes final bytes into the configured persistent Artifact volume. The read-only source mount can be removed after import.

## Manifest import for real Project files

The manifest is a CSV file with this exact header:

```csv
project_id,project_import_key,artifact_type,display_name,file_path
```

Each row must provide exactly one Project identifier:

- `project_id`: canonical Project UUID.
- `project_import_key`: the `import_key` used by the metadata CSV or XLSX import.

`reference_code` is not accepted as a mapping identity because it does not guarantee uniqueness. `file_path` must be relative to the manifest directory and must resolve to a regular file inside that directory.

Example bundle:

```text
project-file-import/
├── manifest.csv
└── projects/
    ├── sp-1703/
    │   ├── report.pdf
    │   └── slides.pdf
    └── sp-1711/
        └── source.zip
```

Example manifest:

```csv
project_id,project_import_key,artifact_type,display_name,file_path
,sp-1703,report,Final report,projects/sp-1703/report.pdf
,sp-1703,slides,Presentation slides,projects/sp-1703/slides.pdf
,sp-1711,source_code,Source code,projects/sp-1711/source.zip
```

Run a dry-run:

```sh
docker compose --env-file <file> \
  run --rm --no-deps \
  --volume /srv/ause-discovery/project-file-import:/bulk:ro \
  --entrypoint /usr/local/bin/ausectl \
  api artifacts import-manifest \
  --manifest /bulk/manifest.csv \
  --actor-username <admin-username>
```

Add `--apply` to perform the uploads. A repeated manifest import skips an active Artifact when the Project, Artifact type, and file digest already match. A different file of the same type remains a separate Artifact because Projects may legitimately contain multiple reports, slide decks, or archives.

## Validation and recovery behavior

Before writing, the importer validates every source path, file size, extension, detected content type, Project mapping, administrator identity, and projected per-Project quota. Missing or ambiguous `project_import_key` values stop the run.

Uploads are committed one file at a time. A runtime failure can leave earlier uploads committed. Repeating the command is the recovery procedure because already imported demonstration types or matching manifest files are skipped. Published Projects are queued for normal search reconciliation after each successful upload.

Demonstration files and final corpus files should normally use separate deployments. Manifest import can add real files to a demonstration deployment, but it does not automatically delete or replace demonstration Artifacts.
