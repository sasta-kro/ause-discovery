# Bulk Project-file import

The `ausectl artifacts` commands attach files to existing Projects through the same validation, storage, quota, revision, search-synchronization, and audit boundaries as administrator uploads. Project metadata must exist before Project files are imported. Direct database inserts and direct copies into `AUSE_ARTIFACT_ROOT` are unsupported.

Both commands default to a read-only dry-run. Add `--apply` only after the dry-run succeeds. An active administrator username is required so every uploaded Artifact has an audit actor. Final bytes go through the configured Artifact storage backend. With B2 selected, validation stages one file at a time in the container operating system temporary directory before uploading it to B2. This staging location is separate from `AUSE_IMPORT_TEMP_ROOT`, which is reserved for metadata import previews.

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

Add `--apply` after the dry-run succeeds. The API service writes final bytes into the configured Artifact backend. With `AUSE_ARTIFACT_STORAGE_BACKEND=b2`, final objects are written to B2 rather than the local Artifact volume. The read-only source mount can be removed after import.

## Manifest import for real Project files

The former CSV manifest import (`ausectl artifacts import-manifest`) was
removed. Real Project Files and Project Logos import through the unified
Project Content JSON manifest instead. See
`project-content-import.md` in this directory for the manifest contract, the
extractor export command, and dry-run and recovery behavior.
