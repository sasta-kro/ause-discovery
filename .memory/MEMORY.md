# Memory router

## Purpose

This directory preserves durable AUSE Discovery project context across sessions. `MEMORY.md` is a router, not a project journal.

## Session bootstrap

Read these files in order before planning or implementation:

1. `../AGENTS.md`
2. `MEMORY.md`
3. `project_context.md`
4. `architectural_decision_logs.md`
5. `mvp_implementation_status.md`, after implementation begins
6. `lessons_learned.md`, when substantive lessons exist

Read `../CONTEXT.md` before changing domain terminology. Read `../docs/ause-discovery_implementation_specification_v1.md` before implementation.

## Memory files

- `project_context.md` contains durable product, repository, and implementation-baseline context.
- `architectural_decision_logs.md` contains accepted architecture decisions and consequences.
- `mvp_implementation_status.md` will contain verified implementation state after application work begins.
- `lessons_learned.md` remains reserved for verified reusable lessons.
- `archives/session_handoff_2026-09-02.md` preserves the historical interrupted core checkpoint and continuation sequence.
- `archives/history-rewrite-2026-09-05.md` preserves the old-to-new commit map for correlating earlier reports and conversations.

## Maintenance rules

- Read a memory file before changing it.
- Update an existing section instead of adding duplicates.
- Store durable facts, accepted decisions, active constraints, and verified lessons.
- Do not store raw command output, temporary notes, credentials, tokens, private keys, secret values, or full environment configuration.
- Mark superseded decisions explicitly.
- Keep this router short.

## Current checkpoint

As of 2026-09-05, the AUSE Discovery technical MVP is complete at commit `1cc5dc6`. Foundation, data contracts, identity and core records, Artifacts, search, imports, audit administration, CI and operator readiness, product coherence, and final product acceptance are verified. Final acceptance covered fresh migration and catalog synchronization, authenticated administration, public discovery, Artifact serving, CSV and XLSX imports, search outage and rebuild recovery, restart persistence, root and subpath deployments, browser accessibility, dependency checks, and runtime image scans. No unresolved technical MVP defect remains. Production ownership, URLs, infrastructure, approved institutional content, and other launch-time inputs remain external launch-readiness work.

## Implementation workflow

Starting after commit `722a6ff`, each remaining vertical uses a split author-review workflow between two different model sessions. A planning and review session (the reviewer model) writes one bounded implementation brief and later inspects the complete diff, commit history, focused verification evidence, and affected integration boundaries before accepting the increment or returning a bounded correction list. A separate implementation session (the coder model) implements the brief, runs focused verification, and commits; it never accepts its own work, publishes, deploys, or starts final acceptance. Only an accepted increment becomes the baseline for the next brief. This workflow carried the repository through the technical MVP; future sessions must read these files before planning further work.
