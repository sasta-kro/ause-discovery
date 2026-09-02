# AGENTS.md

## Wording and Formatting Preferences
In the codebase, and in the documentations:
- no em-dashes
- no emojis
- no personal pronouns such as "you", "your", "I", "we", "us", etc. Only use 3rd person objective tone
- These rules apply only to working code and documentation. They do not apply to:
  - imported documentation or references such as repos
  - Character personality prompts
  - Dialogue strings
  - Test fixtures
  - Generated files

### Codebase Style
- Modular architecture and reusable code. Introduce abstractions only when a real implementation boundary, replacement requirement, or repeated use exists.
- Descriptive variable names
- Descriptive function names
- The code itself should be descriptive enough to explain what it is doing without comments.

- Comments should explain non-obvious rationale, constraints, contracts,
external behavior, or intentional workarounds. Comments should not merely
restate the code.

---

## Memory System

- **Long-term:** `./.memory/` - curated memories, like a human's long-term memory. Be conservative. Don't just put anything in here.
- **Lessons Learned:** so mistakes don't repeat. At `./.memory/lessons_learned.md`
- **Architecture Decisions:** so agents don't go in a refactor spiral of trying out already-tried solutions that do not work. At `./.memory/architectural_decision_logs.md`


Capture what matters. Decisions, context, things to remember. Skip non-important stuff unless asked to keep them.

- Memory File Rule - READ BEFORE WRITING
- **Memory is limited** - if want to remember something, WRITE IT TO A FILE. No "mental notes"

---


- Recoverable unknowns and launch-time values do not block implementation.
- Prefer working software and focused verification.
- ADRs are reserved for difficult, durable architecture decisions.
- Product-owner input is reserved for external authorization, destructive actions, legal matters, secrets, and irreversible product choices.