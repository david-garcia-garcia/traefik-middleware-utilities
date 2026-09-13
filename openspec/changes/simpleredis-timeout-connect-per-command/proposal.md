## Why

A peer slower than `IOTimeout` makes SimpleRedis close the pooled socket on every command (the reply is still outstanding). The next independent command dials. That close is specified and correct; the operational shape is connect-per-command, extra AUTH/SELECT per reconnect, and TIME_WAIT at the churn rate. A circuit breaker or dial-rate limiter would add failure-memory this stdlib-only Yaegi client should not own. Operators need the composition written down, including that the existing "timeout MUST NOT open a second connection" rule is same-command retry, not a cap on later commands.

## What Changes

- Clarify on `std_go_simpleredis_tcp-session` that destroying the timed-out socket leaves idle empty, so a later independent command dials (AUTH/SELECT if configured). No runtime change.
- Add a usage gotcha on `knowledge/devdocs/std_go_simpleredis.md`: timeout → connect-per-command; sequential dial rate ≈ `1/IOTimeout`; concurrent peak Redis-side sockets ≈ `2×PoolSize` per client from overlap; raise `IOTimeout` rather than expecting a client breaker; do not change the 100ms default in this change (**BREAKING** if someone later raises it — not this change).
- Do not add a circuit breaker, dial-rate limiter, cooldown timestamp, background drain, or retry-on-timeout.
- Do not add a default-suite test that bounds dials under a slow peer (that would specify a limiter this change is not adding).

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: the I/O-timeout rule's "MUST NOT open a second connection" applies to retry of that timed-out command. After the socket is destroyed, a later independent command follows the idle-empty dial rule (including AUTH/SELECT once per new dial). The session is not required to rate-limit those later dials.

## Impact

- Spec delta only plus usage `knowledge/devdocs/std_go_simpleredis.md`. No `simpleredis/*.go` edits.
- Callers (`windowcounter`, `tokenbucket`) keep zero-Config 100ms `IOTimeout`; they can raise it per client when a loaded Redis p99 exceeds 100ms.
- Existing large note `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md` stays (dial *failures*, different trigger). This change does not take a timeout-storm breaker.
