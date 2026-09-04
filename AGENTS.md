# AGENTS.md

## Wording and Formatting Preferences

In the codebase and documentation:

- No em-dashes.
- No emojis.
- No personal pronouns such as "you", "your", "I", "we", or "us". Use objective third-person tone.
- These rules do not apply to:
  - Imported documentation or references.
  - Dialogue strings.
  - Test fixtures.
  - Generated files.

## Codebase Style

- Prefer modular architecture and reusable code.
- Introduce abstractions only when a real implementation boundary, replacement requirement, or repeated use exists.
- Try not to reinvent the wheel if not needed. If a function/feature has been done well by a library, use the library rather than reinventing the same feature from scratch.
- Use descriptive variable and function names.
- Prefer self-explanatory code over explanatory comments.
- Comments should explain non-obvious rationale, constraints, contracts, external behavior, or intentional workarounds. Comments should not merely restate the code.

## Implementation and Verification
Do not over-test and waste time.

- Prefer completing coherent vertical increments over repeatedly re-verifying the entire system.
- For each increment, run focused tests for changed behavior and a targeted integration or smoke test for affected boundaries.
- Expand verification when focused checks expose failures, architectural risk, data-loss risk, security-sensitive behavior, or cross-system uncertainty.
- Reserve exhaustive test suites, fresh-environment validation, restart/recovery testing, and full product acceptance for major checkpoints and final completion unless earlier evidence requires them.
- Fix discovered regressions before building further on the affected area, but avoid unrelated cleanup and speculative hardening.
- After a coherent increment passes focused verification, record its state and continue to the next planned increment.
- Avoid broad repository rereads, repeated full-suite runs, and repeated environment rebuilds when relevant state is already known and unchanged.

## Memory System

- **Long-term:*- `./.memory/` for curated durable context. Be conservative.
- **Lessons Learned:*- `./.memory/lessons_learned.md` for mistakes and practices that should not repeat.
- **Architecture Decisions:*- `./.memory/architectural_decision_logs.md` for durable decisions that should prevent repeated architectural experimentation.

Capture decisions, durable context, constraints, and information worth preserving. Skip transient implementation details unless they are needed for recovery.

- Read the relevant memory files before writing to them.
- Memory is limited. Durable information that must survive context loss should be written to a file rather than treated as a mental note.

## Execution Principles

- Recoverable unknowns and launch-time values do not block implementation.
- Prefer working software and focused verification.
- ADRs are reserved for difficult, durable architecture decisions.
- Product-owner input is reserved for external authorization, destructive actions, legal matters, secrets, and irreversible product choices.
