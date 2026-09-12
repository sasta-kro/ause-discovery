# Project Content import (logos and Project files)

`ausectl project-content import-manifest` is the one command that attaches a
Project Logo and Project Files to existing Projects. It replaces the removed
`ausectl artifacts import-manifest` and its five-column CSV. Project metadata
must already exist (metadata CSV/XLSX import or manual creation). Direct
database inserts and direct copies into storage volumes are unsupported.

The command defaults to a read-only dry-run. Add `--apply` only after the
dry-run succeeds. An active administrator username is required so every
upload has an audit actor.

```sh
ausectl project-content import-manifest \
  --manifest <bundle>/project-content.json \
  --actor-username <admin-username> \
  [--workers <1-8>] [--apply]
```

`--workers` bounds concurrent Projects (default 4, maximum 8, set once).
Within one Project the logo uploads first, then files upload sequentially.
A repeated run skips an item when the existing state already matches: a logo
whose active digest equals the file digest is left untouched, and Project
Files follow the same active-Artifact digest skip as before.

`ausectl artifacts seed-demo` and `ausectl artifacts migrate` remain separate
commands for demonstration seeding and storage migration. They are unchanged
by this command.

## Manifest contract

One strict JSON file named by `--manifest`. Every `file_path` resolves
relative to the manifest's own directory, so the manifest and its files form
one self-contained bundle.

```json
{
  "version": 1,
  "projects": [
    {
      "project_import_key": "sp-1703",
      "logo": { "file_path": "logos/1703.png" },
      "files": [
        {
          "artifact_type": "report",
          "display_name": "Final report",
          "original_filename": "report.pdf",
          "file_path": "projects/sp-1703/report.pdf"
        }
      ]
    }
  ]
}
```

Rules the loader enforces before any database or storage work:

- `version` must be exactly `1`; unknown fields and trailing JSON data are
  rejected.
- Each project sets exactly one of `project_id` (canonical UUID) or
  `project_import_key` (the metadata import `import_key`, stored in
  `projects.extra_metadata`). `reference_code` is not an identity.
- No project appears twice.
- `logo` and `files` are optional but the whole entry needs at least one of
  them; blank strings anywhere are rejected.
- Logos must be PNG, at most 2 MiB and 1600 by 1600 pixels.
- Project Files follow the Artifact allowlists (type, extension, detected
  content, size) and the per-Project quota is validated for the whole
  operation before any write.

Logos are not Artifacts: they never enter Artifact lists, counts, quotas, or
facets, and they live in their own `project_logos` records.

## Building a bundle from the extractor output

The extractor repository produces `output/enrichment/manifest.json`. Export a
logo-only Project Content manifest with jq from the extractor repository
root:

```sh
jq '{version: 1, projects: [to_entries[]
  | select(.value.logo.output != null)
  | {project_import_key: ("sp-" + .key),
     logo: {file_path: .value.logo.output},
     files: []}]}' \
  output/enrichment/manifest.json > output/enrichment/logo-manifest.json
```

Write the manifest into the same directory as the content it references
(here `output/enrichment/`, whose `logos/<id>.png` paths the export emits).
Project Files can be added later by extending `files` in the same manifest.

Verified 2026-09-12 against extractor commit `153408e`: the export produced
exactly 71 entries, every referenced PNG exists, is a valid PNG at most
1600 pixels and at most 1,430,125 bytes, and a dry-run plan through the
command resolved and validated all 71 entries (14,757,652 bytes total).

## Validation and recovery behavior

Planning validates the complete bundle first: Project resolution, logo
content, file paths (including symlink and escape rejection), quota for the
whole operation, and administrator identity. One invalid entry stops the run
before any write.

Apply commits per item. A runtime failure leaves earlier items committed and
the command exits nonzero with a summary; rerunning the same command is the
recovery procedure because unchanged items are skipped. Published Projects
are queued for normal search reconciliation after each successful item, and
logo changes advance the Project revision so `logo_url?v=` versions strictly
increase.
