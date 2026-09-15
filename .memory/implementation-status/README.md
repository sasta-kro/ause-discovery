# Implementation status

## Purpose

This directory separates current implementation decisions from completed history
so routine planning does not require loading the full technical record.

## Reading routes

- Read `backlog.md` for current constraints, recurring operations, incomplete
  work, suspected defects, and parked possibilities.
- Read `completed.md` only when verifying existing behavior, tracing why a
  feature exists, or locating an accepted increment and its evidence.
- Read `../MEMORY.md` first for the broader memory routing contract.
- Read `../CONTEXT.md` for domain terminology and
  `../architectural_decision_logs.md` for accepted architecture decisions.

## Current checkpoint

The technical MVP was completed at commit `1cc5dc6`. Post-MVP work through
Search Filter Suggestions Increment 18 was implemented at commit `9278808`,
corrected through commit `7cfc668`, and received its maintainer-directed visual
correction at commit `95bd9fc`. Post-release frontend work through filter-panel
ordering and expansion focus Increment 20 is independently accepted at commit
`eedeecd`, the current accepted implementation baseline. Increment 21 was
implemented at `0591de0` and corrected at `170f5a8`, pending independent review.
API and web release 0.5 from `95bd9fc` were deployed successfully through the
production-preserving upgrade path on 2026-09-15.

## Status lifecycle

For now, status is divided into two durable records:

- `backlog.md`: not completed, partially completed, recurring, externally
  blocked, suspected, or deliberately parked.
- `completed.md`: implemented, accepted, superseded, resolved, or otherwise
  closed work.

A future `in-progress.md`, `halted.md`, or `discarded.md` may be added only
when a real status category needs a separate reading route. Do not create empty
category files.

When work is accepted, move its complete entry from `backlog.md` to
`completed.md`, add the date and increment or direct-request reference, and
avoid leaving a duplicate active entry. Keep detailed verification evidence in
`completed.md`; keep `backlog.md` focused on decisions still required.
