# Memory router

## Purpose

This directory preserves durable AUSE Discovery project context across sessions. `MEMORY.md` is a router, not a project journal.

## Session bootstrap

Read these files in order before planning or implementation:

1. `../AGENTS.md`
2. `MEMORY.md`
3. `CONTEXT.md`, whose Project orientation section carries the durable
   product summary, canonical boundaries, and MVP exclusions
4. `architectural_decision_logs.md`
5. `implementation-status/README.md`, then `implementation-status/backlog.md`
   for current work or `implementation-status/completed.md` for accepted history
6. `lessons_learned.md`, when substantive lessons exist

Read `CONTEXT.md` before changing domain terminology. The record of what
was actually built is `implementation-status/completed.md` with the accepted
briefs in `docs/increments/`. The historical MVP scope record is
`archives/mvp-specification-v1.md`.

## Memory files

- `CONTEXT.md` contains the domain glossary plus the durable project
  orientation: product summary, canonical boundaries, and MVP exclusions.
- `architectural_decision_logs.md` contains accepted architecture decisions and consequences.
- `implementation-status/README.md` routes status reading;
  `implementation-status/backlog.md` contains current or incomplete work; and
  `implementation-status/completed.md` contains accepted history and evidence.
- `lessons_learned.md` remains reserved for verified reusable lessons.
- `archives/history-rewrite-2026-09-05.md` preserves the old-to-new commit map
  for correlating earlier reports and conversations. Superseded session and
  deployment archives were removed on 2026-09-14 because their content was
  stale or fully superseded by the current notes.

## Maintenance rules

- Read a memory file before changing it.
- Update an existing section instead of adding duplicates.
- Store durable facts, accepted decisions, active constraints, and verified lessons.
- Do not store raw command output, temporary notes, credentials, tokens, private keys, secret values, or full environment configuration.
- Mark superseded decisions explicitly.
- Internal notes, plans, scraps, agent workflow, and operator-only guidance belong only in `.memory/`; `docs/` is public-facing documentation changed on request, never mixed with internal content (see `../AGENTS.md` Documentation Boundaries).
- Keep this router short.

## Current checkpoint

As of 2026-09-13, the AUSE Discovery technical MVP is complete at commit
`1cc5dc6`, the application is deployed at
`https://life.au.edu/ause-discovery/`, and post-MVP implementation through
Search Filter Suggestions Increment 18 was implemented at commit `9278808` and
corrected through commit `7cfc668`, which is the latest accepted implementation
baseline. Read
`implementation-status/backlog.md` for current work and constraints. Read
`implementation-status/completed.md` only for accepted history, detailed
behavior, or verification evidence.

## Implementation workflow

Starting after commit `722a6ff`, each remaining vertical uses a split author-review workflow between two different model sessions. A planning and review session (the reviewer model) writes one bounded implementation brief and later inspects the complete diff, commit history, focused verification evidence, and affected integration boundaries before accepting the increment or returning a bounded correction list. A separate implementation session (the coder model) implements the brief, runs focused verification, and commits; it never accepts its own work, publishes, deploys, or starts final acceptance. Only an accepted increment becomes the baseline for the next brief. This workflow carried the repository through the technical MVP; future sessions must read these files before planning further work.
