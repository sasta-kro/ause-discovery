# Memory router

## Purpose

This directory preserves durable project context across sessions. `MEMORY.md` is a router, not a project journal or a storage location for detailed findings.

## Session bootstrap

Read the following files in order before planning or implementation:

1. `../AGENTS.md`
2. `MEMORY.md`
3. `project_context.md`
4. `architectural_decision_logs.md`
5. `mvp_implementation_status.md`, while the MVP remains active
6. `lessons_learned.md`, when present

## Memory files

- `project_context.md` contains the product intent, repository findings, MVP boundary, current plan, deferred work, and unresolved checks.
- `architectural_decision_logs.md` contains accepted architecture decisions, reasons, alternatives, and consequences.
- `mvp_implementation_status.md` contains the implemented runtime, verified behavior, and remaining external prerequisites.
- `lessons_learned.md` is created only after a verified mistake or repeated failure produces a reusable lesson.

## Maintenance rules

- Read the relevant file before changing it.
- Update an existing section instead of adding a duplicate entry.
- Store durable facts, accepted decisions, active constraints, and verified lessons.
- Do not store raw command output, temporary debugging notes, speculative ideas, credentials, tokens, private keys, or `.env` values.
- Record implementation details only when required to preserve intent or prevent repeated investigation.
- Mark superseded decisions instead of silently rewriting their history.
- Keep this router short.

## Current checkpoint

As of 2026-08-27, `riko_sasta` exists as an independent Git repository and the MVP runs end to end with Groq as the default language-model provider. Native MPS GPT-SoVITS synthesis, MLX Whisper transcription, service integration, browser behavior, microphone detection, and one complete spoken turn are verified. The active milestone adds observability, runtime status feedback, and subtitles. Read `mvp_implementation_status.md` before resuming implementation or verification.
