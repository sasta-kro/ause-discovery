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


## Sub-Agent Delegation

The root **Sol High** agent is primarily an orchestrator. Preserve its context for understanding the overall problem, planning, architectural reasoning, coordination, escalation, and integrating results. 


Delegate only when independent work, isolated review, or specialized
investigation materially reduces time or risk. Prefer direct work for
small, serial, tightly coupled, or time-sensitive MVP tasks.

The goal is to keep the cost down. 

### Autonomous Agentic Implementation

The product specification defines product requirements. The implementation guide adds architecture, technical decisions, and sequencing without restating the full product specification.

The root Sol agent decides architecture, cross-cutting contracts, security posture, persisted-data semantics, and other major implementation choices. Workers decide routine local implementation details inside their assigned boundaries. Multiple reasonable local approaches are not a reason to stop and request product-owner input.

Recoverable unknowns and launch-time values do not block local implementation. GitHub ownership, repository URL, registry ownership, production hostname, VM details, branding, and final legal content affect only the work that directly consumes them. Neutral local configuration and fixtures allow unrelated work to continue.

Escalate to the root agent when a decision changes product behavior, a fixed architecture boundary, a public contract, persisted compatibility, security posture, or another worker's owned area. Product-owner input is reserved for secrets, external authorization, destructive external actions, legally sensitive content, and genuinely irreversible product choices.

Implementation guidance should be detailed enough to prevent drift without prescribing every private function, file, test helper, or coding step. Prefer working software and focused verification over process artifacts, compliance checklists, mandatory pull-request templates, or artificial work-package gates.

ADRs are reserved for hard-to-reverse architecture decisions with meaningful tradeoffs. Verified recurring failure patterns belong in lessons learned. Ordinary bug fixes and routine local choices require neither.

### Model Routing

* **Terra High** is the default worker for well-scoped engineering work: implementation, repository exploration, tests, refactoring, routine debugging, verification, and other bounded tasks.
* **Sol Medium** is the stronger worker for tasks that are unusually difficult, ambiguous, cross-cutting, reasoning-intensive, or likely to exceed Terra High's capabilities.
* **Sol High** should handle the hardest diagnosis, architecture, and decision-making. When possible, it should turn its conclusions into a clear task and delegate the resulting implementation rather than carrying it out itself.

Choose the worker based on the actual difficulty of the task. Do not repeatedly retry an unsuitable model merely to avoid escalation. When specific models are not available, try other similar models.

### Delegation

When delegating, provide enough information for the worker to operate independently:

* the objective and expected outcome;
* relevant constraints and architectural decisions;
* useful starting points or known affected areas;
* acceptance criteria and required verification;
* important context the worker could not reasonably discover itself.

Do not unnecessarily restate repository contents or prescribe every implementation detail. Let workers inspect the codebase and determine routine details themselves.


Prefer **0-1 active workers** for normal work. Use up to roughly **2-3 workers** when there are genuinely independent tasks that benefit from parallel execution. Avoid overlapping assignments, duplicated investigation, or multiple workers modifying the same area without a clear reason.

Assign exclusive file ownership before delegated edits begin.
Do not run concurrent edits in the same project area.
Parallel workers should normally perform read-only investigation or edit clearly separate areas.

The root owns coordination between workers and should keep task boundaries clear.

### Context Preservation

Keep the root context compact.

While read-only workers are running, the root should wait for final or milestone reports instead of polling or ingesting routine progress output. Coding workers may receive targeted milestone checks when early correction can prevent architectural or cross-module drift.

Workers should return concise, decision-relevant results rather than raw exploration output. Reports should normally include:

* what was done or discovered;
* important decisions or findings;
* files or areas changed;
* verification/tests performed and their result;
* unresolved issues, risks, or decisions needed from the root.

Avoid bringing large logs, command output, exhaustive diffs, or unnecessary implementation details into the root context. Delegate additional investigation or review when that information does not need to be processed directly by the root.

### Verification and Review

Workers should verify their own work whenever practical.

For changes that warrant independent review, prefer assigning a separate worker to review or test them and return a concise assessment rather than loading all review material into the root context.

The root should perform only the amount of direct inspection necessary to coordinate the overall solution and resolve important uncertainties.

### Direct Work by the Root

Do not delegate when delegation would clearly cost more than performing the task directly.

Sol High may directly handle trivial, context-light work such as a simple value change, obvious one-line adjustment, short command, tiny follow-up edit, or similarly inexpensive operation.

Judge this by **coordination and context cost**, not merely by the number of changed lines. A small-looking change that requires substantial investigation or reasoning should still be delegated or analyzed appropriately.

### Escalation

Use this general progression:

**Terra High → Sol Medium → Sol High reasoning**

Escalate when the current worker lacks the required judgment, becomes stuck, produces uncertain results, or the task turns out to be substantially harder than expected.

If Sol High must solve the difficult reasoning itself, it should preferably delegate the mechanical implementation of that solution afterward.

Optimize for effective use of model capability and root-context preservation, not delegation for its own sake. Correctness takes priority over usage savings.

---
