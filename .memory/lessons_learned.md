# Lessons learned

## Goose procedural statements

Goose splits SQL migrations at semicolons unless procedural function bodies are enclosed by `StatementBegin` and `StatementEnd`. Fresh-database migration tests must cover every migration containing PL/pgSQL.

## Open-ended PostgreSQL ranges

An inclusive `int4range` upper-bound sentinel at the maximum integer overflows during canonicalization. A `NULL` upper bound represents an unbounded historical validity range safely and preserves overlap exclusion behavior.

## Optional audit values

Optional audit UUIDs require nullable PostgreSQL conversion. A zero UUID is a real value and violates foreign keys when no actor exists. Absent audit metadata must serialize as an empty JSON object rather than JSON `null` when the schema requires object metadata.

## Reverse-proxy trust

The original Host header, including a non-default port, must reach same-origin validation. Forwarded client addresses must be evaluated from the immediate trusted peer toward the client, selecting the first untrusted address. Forwarded protocol is trusted only when the immediate peer is configured as trusted.
