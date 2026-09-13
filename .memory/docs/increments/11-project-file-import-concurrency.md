# Project file import concurrency increment

## Assignment

Implement and commit bounded concurrency and useful progress output for the bulk Project File importer.

This is one bounded increment. Parallelize work across Projects, preserve sequential uploads within each Project, add one progress line per completed Project, run focused verification, update durable memory, commit, and stop. Do not implement Project Logos in this increment.

The accepted code baseline is commit `3394f0f`. Preserve unrelated working-tree changes and all existing import behavior unless this brief explicitly changes it.

## Required reading before coding

1. `AGENTS.md`
2. `.memory/MEMORY.md`
3. `.memory/CONTEXT.md`
4. `.memory/implementation-status/completed.md`, especially former backlog item 12
5. `.memory/sasta-dev-notes/project-file-import.md`
6. `backend/internal/artifactimport/`
7. `backend/cmd/ausectl/main.go` and its focused tests
8. This brief

Inspect nearby Artifact service and storage-provider code only where needed to preserve revision, cancellation, and error behavior.

## Repository starting state

- `artifactimport.Service.Run` validates and plans the entire operation, then uploads every planned file in one sequential loop.
- Each actual upload reads the current Project revision and calls `artifacts.Service.Upload`, which advances that Project revision and schedules search synchronization.
- Different Projects can be processed independently. Files belonging to one Project cannot be uploaded concurrently without causing revision conflicts.
- `seed-demo` and `import-manifest` both use the same `artifactimport.Service.Run` path.
- Commands are dry-run by default. `--apply` performs writes.
- Safe reruns depend on a final pre-upload duplicate check. Demo imports skip an already-populated Artifact type; manifest imports skip the same type and digest.
- The CLI prints only a final aggregate summary. A successful 836-file B2 run therefore appeared stalled until completion.
- Maintainer testing on 2026-09-11 successfully uploaded 836 files to 209 Projects, then confirmed that a second run skipped all 836.
- A later Project Logo increment will add at most one stored image per Project. The scheduling boundary established here must remain Project-based and must not assume exactly four demo files.

## Required outcome

An apply run starts with a concise line, processes up to four Projects concurrently by default, and prints exactly one serialized completion line for each Project. Each line identifies the completed Project and summarizes all files uploaded, skipped, or failed for that Project. The final aggregate summary remains. Output never produces one line per file.

Files within a Project remain sequential and in plan order. Safe reruns, dry-run behavior, Artifact validation, quota checks, audit events, Project revision updates, and search synchronization remain unchanged.

## Scope

### Included

- Bounded Project-level worker pool in `artifactimport`.
- Default worker count of 4.
- `--workers <n>` on `artifacts seed-demo` and `artifacts import-manifest`, valid from 1 through 8.
- One start line before apply work begins.
- One coordinator-serialized progress line per completed Project.
- Uploaded, skipped, and failed file details grouped on that Project line.
- Aggregate failed Project and failed file counts in the final result and summary.
- Context cancellation and controlled storage-unavailable handling.
- Focused unit, command-parser, and database-backed tests.
- Durable memory update and one coherent implementation commit.

### Excluded

- Concurrent uploads within one Project.
- Unbounded goroutines or one worker per file.
- Changing the Project File manifest headers or demo file definitions.
- Project Logo schema, import, HTTP, or frontend work.
- New retry logic beyond the configured storage client behavior.
- Live B2 load testing, another 836-file seed, stack rebuild, image publication, or VM deployment.
- Storage terminology refactoring and the separate storage review corrections already recorded in the backlog.

## Contract decisions

### Scheduling unit

Group the validated plan by resolved Project ID while preserving:

- first Project appearance order for dispatch;
- file order within each Project;
- every existing planned skip reason.

Run a fixed worker pool over Project groups. A worker processes one Project at a time and handles that Project's files sequentially. No two workers may process the same Project.

`--workers 1` must reproduce sequential Project processing. Values below 1, above 8, malformed values, and repeated conflicting flags must return the existing usage-style error before database or storage work begins.

### Service and progress boundary

The import service must remain independent of standard output. Extend `artifactimport.Options` with the bounded worker count, an apply-start callback carrying the completed plan summary, and a narrowly typed Project-completion callback, or an equivalently small event boundary. Do not place `fmt.Print` calls inside upload workers.

The service invokes the start callback after planning and immediately before worker dispatch. It invokes Project callbacks through one coordinator goroutine. Callback calls must never overlap. The CLI owns formatting and output.

A Project progress value must contain enough structured data for the CLI to render:

- completed position and total Project count;
- Project ID and title;
- uploaded file descriptors;
- skipped file descriptors and reasons;
- failed file descriptor and controlled error text, when present;
- elapsed Project duration.

File descriptors should use Artifact type plus original filename. Source paths, storage keys, credentials, request bodies, and raw provider responses must not be printed.

### Output contract

Apply starts with one line similar to:

```text
Starting Project file import: 209 Projects, 836 planned uploads, 4 workers
```

Each completed Project produces one physical line. An acceptable shape is:

```text
[17/209] Project title: uploaded report (final-report.pdf), slides (presentation-slides.pdf); skipped poster (already present); 2.1s
```

Rules:

- The counter is completion progress, not input-order position.
- Lines may appear in completion order.
- Project titles and filenames must have control characters and line breaks collapsed so one event cannot create multiple terminal lines.
- Long titles and file lists may be bounded without losing Project identity or outcome counts.
- A Project with only planned or final skips still produces one completion line.
- A failed Project produces one line with the failed file and brief error, then no more files are attempted for that Project.
- No worker prints independently.
- Dry-run keeps the current summary-only behavior and must not emit fake completion progress.
- The existing final summary labels remain, with failed counts added when nonzero.

### Failure and cancellation behavior

- An isolated Project failure stops the remaining files for that Project but does not discard successful work already committed for it.
- Other already-running Projects may finish.
- Ordinary isolated failures do not prevent queued Projects from running. The command finishes nonzero after all schedulable Projects complete.
- `context.Canceled`, `context.DeadlineExceeded`, or an error matching `artifacts.ErrStorageUnavailable` stops dispatching new Projects. In-flight workers receive cancellation, finish or stop promptly, and the command exits nonzero.
- The result is updated only by the coordinator. No shared counters or slices may be mutated unsafely by workers.
- Error aggregation must be bounded. The per-Project progress lines contain details; the returned error may summarize the failure count and first safe error.

### Rerun and revision behavior

Keep the final `shouldSkip` check immediately before each upload. Before every actual upload, read that Project's current revision as today. Do not cache one revision for all files in a Project because every successful upload advances it.

Concurrency must not change these cases:

- a second demo seed skips an existing active Artifact type;
- a second manifest import skips an identical active type and digest;
- a partial failed run can be repeated safely;
- two manifest entries for the same Project never race each other;
- quota planning remains whole-operation validation before writes.

## Suggested implementation order

1. Extend `backend/internal/artifactimport/model.go` with worker options, Project work/result models, and aggregate failure counts.
2. Refactor `backend/internal/artifactimport/service.go` so the current single-file behavior becomes a sequential Project-group operation.
3. Add the bounded worker dispatcher and single coordinator. Keep planning synchronous and complete before apply.
4. Extend `backend/cmd/ausectl/main.go` with `--workers`, the start line, Project progress formatting, and final failed counts.
5. Update focused parser and import tests.
6. Run focused verification once the increment is coherent.
7. Record the implemented behavior and exact evidence in `.memory/implementation-status/completed.md`.
8. Commit once and stop.

Do not extract a generic concurrency framework during this increment. If the Project Logo importer in increment 12 creates a verified second use, the smallest shared Project-work helper may be extracted there.

## Focused verification

Automated tests must prove:

1. Worker count defaults to 4 and accepts only 1 through 8.
2. At least two different Projects overlap when workers exceed 1.
3. Observed concurrent Project work never exceeds the selected bound.
4. Files for one Project execute sequentially and in plan order.
5. Project completion callbacks are serialized and fire exactly once per Project.
6. Progress values correctly group uploaded, skipped, and failed files.
7. A partial failure leaves successful uploads intact, returns nonzero, and reruns safely.
8. Storage unavailability cancels queued work without leaking goroutines.
9. Existing dry-run, path validation, quota, and manifest semantics still pass.
10. Race detection passes for the focused package if supported by the pinned Go container.

Run only the smallest relevant repository targets or pinned-container commands for:

- `backend/internal/artifactimport` tests;
- `backend/cmd/ausectl` parser and formatting tests;
- one focused PostgreSQL integration test covering two Projects and multiple files per Project;
- `git diff --check`.

Use fake or in-memory storage adapters for concurrency timing. Do not access the live B2 bucket. Do not rebuild or erase the maintainer's local stacks or volumes.

## Completion criteria

- Apply output provides immediate start feedback and exactly one line per completed Project.
- Project concurrency is bounded and configurable from 1 through 8.
- Same-Project uploads remain sequential and revision-safe.
- Safe reruns and dry-run behavior remain intact.
- Failures are visible, bounded, and return a nonzero exit status.
- Focused tests and race-sensitive checks pass.
- No logo, OpenAPI, frontend, database migration, Compose, release, or deployment changes are present.
- Existing untracked `.env.local-test` and `.DS_Store` files remain unstaged.
- One coherent commit uses a lowercase past-tense message with no prefix and no co-author trailer.

## Required handoff

Stop after the implementation commit. Report:

- commit hash and exact message;
- files changed;
- worker-pool and cancellation design;
- exact output examples for success, skip, and failure;
- focused test commands and results;
- any failed test or residual risk;
- confirmation that no live B2, Docker volume, image publication, or VM state changed;
- final `git status --short`.
