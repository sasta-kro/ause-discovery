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

## Maintenance rules

- Read a memory file before changing it.
- Update an existing section instead of adding duplicates.
- Store durable facts, accepted decisions, active constraints, and verified lessons.
- Do not store raw command output, temporary notes, credentials, tokens, private keys, secret values, or full environment configuration.
- Mark superseded decisions explicitly.
- Keep this router short.

## Current checkpoint

As of 2026-09-02, AUSE Discovery is a greenfield monorepo with Git initialized on `main`, a default Vite React frontend scaffold, an empty backend, an approved product specification, and an implementation guide designed for autonomous agentic coding. Application feature implementation has not started.
