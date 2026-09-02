# Lessons learned

## Goose procedural statements

Goose splits SQL migrations at semicolons unless procedural function bodies are enclosed by `StatementBegin` and `StatementEnd`. Fresh-database migration tests must cover every migration containing PL/pgSQL.

## Open-ended PostgreSQL ranges

An inclusive `int4range` upper-bound sentinel at the maximum integer overflows during canonicalization. A `NULL` upper bound represents an unbounded historical validity range safely and preserves overlap exclusion behavior.
