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
7. `session_handoff_2026-09-02.md`, while the interrupted MVP working tree remains uncommitted

Read `../CONTEXT.md` before changing domain terminology. Read `../docs/ause-discovery_implementation_specification_v1.md` before implementation.

## Memory files

- `project_context.md` contains durable product, repository, and implementation-baseline context.
- `architectural_decision_logs.md` contains accepted architecture decisions and consequences.
- `mvp_implementation_status.md` will contain verified implementation state after application work begins.
- `lessons_learned.md` remains reserved for verified reusable lessons.
- `session_handoff_2026-09-02.md` contains the interrupted working-tree state and single-agent continuation sequence.

## Maintenance rules

- Read a memory file before changing it.
- Update an existing section instead of adding duplicates.
- Store durable facts, accepted decisions, active constraints, and verified lessons.
- Do not store raw command output, temporary notes, credentials, tokens, private keys, secret values, or full environment configuration.
- Mark superseded decisions explicitly.
- Keep this router short.

## Current checkpoint

As of 2026-09-02, foundation and data-contract increments are verified and committed. Identity, People, Project, HTTP, and frontend core work exists in an interrupted uncommitted working tree. The next session must follow `session_handoff_2026-09-02.md` and continue with one agent only.
