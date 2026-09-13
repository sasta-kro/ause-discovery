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
Project Logo loading Increment 16 was implemented at commit `af53e63` and
corrected through commit `bfb1cff`, the latest Increment 16 implementation
baseline. No implementation item is currently recorded as in progress.

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

