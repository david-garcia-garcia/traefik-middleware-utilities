# Requirement
IssueKey: 2026-09-12-simpleredis-bug-05-windowcounter-hides-outage

## Problem

Buffered `windowcounter` (`syncRate > 0`) discards every `flushPending` error. While a local delta is pending, `Take` does not GET Redis, so a Redis outage is invisible: each instance keeps admitting up to its own `limit` and returns a nil error. Across N instances the global cap becomes roughly `limit × N` for the length of the outage.

## Current (code)

- `windowcounter/limiter.go:344-355` (`flushLoop`) — each tick does `_ = l.flushPending()` and drops the error.
- `windowcounter/limiter.go:283-287` (`Sleep`) and `:299-310` (`Close`) — same `_ = l.flushPending()`.
- `windowcounter/limiter.go:357-391` (`flushPending`) — EVAL INCRBY+EXPIREAT per dirty window; keeps `firstErr`; only sets `localDelta = 0` on success; returns `firstErr`. No caller stores that value.
- `windowcounter/limiter.go:228-251` (`windowLocked`) — GET Redis only on first sight or when `localDelta == 0`. With `localDelta > 0` it never calls `getCount`, so it cannot see an outage.
- `windowcounter/limiter.go:159-176` (`takeBuffered`) — increments `localDelta` and returns `(allowed, estimate, nil)` with no Redis call after seed.
- `windowcounter/limiter.go:140-157` (`takeExact`) — `Incr`/`Expire`/`getCount` errors propagate. Exact mode fails closed. Matches the ticket.
- `windowcounter/limiter.go:209-226` (`peekCountLocked`) — buffered Peek never GETs once the key is in `windows`, including when `localDelta > 0`. Same silent window for Peek; ticket names Take.
- `windowcounter/limiter.go:394-404` (`parseEvalInt`) — `errors.New(simpleredis.RedisIssue)` at `:397` and `:401`; `strconv.ParseInt` cause and the reply are discarded.
- `windowcounter/limiter.go:13-14`, `:45-55` (`New`) — `minSyncRate` is 20 ms; positive values below that floor. Comment does not describe outage behaviour per mode.
- `README.md:15-16`, `:24` — `sync_rate=0` vs `>0` as flush/accuracy only. No per-mode failure contract.
- `knowledge/devdocs/std_go_windowcounter.md` — `sync_rate` and “Match Redis errors by `Error()` text.” No buffered-outage contract.
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` — “Redis errors propagate”: Take/Peek SHALL return `redis:unreachable` / `redis:timeout` and MUST NOT fail-open, fail-close, or health-gate; unreachable MUST NOT admit or deny as a silent fallback.
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` — buffered Take admits from `redis_known + local_delta`; successful flush clears `local_delta`. No requirement on a failed flush or a discarded ticker error.
- `windowcounter/limiter_test.go:97-107` (`TestTake_Unreachable`) and `:486-496` (`TestPeek_Unreachable`) — both construct with `syncRate == 0`. No buffered pending-delta outage case.
- `windowcounter/fake_redis_test.go:21-39` — in-process RESP listener; cleanup closes the listener only. No on-demand kill of listener plus live sockets. No `Kill` in this package (`not found` as a method).
- `knowledge/research/ext_kong_rate-limiting_sliding-sync/notes.md` — Kong `sync_rate` 0 vs >0 and OSS EVAL flush. Does not state what Kong does when that flush fails.

## Desired

- Retain the flush error. Store `lastFlushErr` and `flushFailedAt` on `Limiter` under `l.mu`. Set them in `flushLoop`, `Sleep`, and `Close` instead of `_ =`.
- Let callers see that error. Pick one surface (explore decides; ticket lists all three, does not pick):
  - Return it from `Take` (third value, or a typed error that still carries the admit decision).
  - `LastFlushError()` / `Stale() bool` for the middleware to poll.
  - Staleness deadline: if no flush has succeeded for `k × syncRate`, `Take` itself returns the error. Ticket calls this the best default. Do not force a GET on every buffered Take (that undoes buffering).
- Document exact vs buffered failure semantics next to `syncRate` in `New` and in the README (and the usage packet).
- Wrap `parseEvalInt` conversion failures: `fmt.Errorf("%s: %w", simpleredis.RedisIssue, convErr)` so the cause is not the bare `redis:issue?` string.
- Tests: killable in-process RESP (close listener and live sockets). Control flush timing.
  1. Long `syncRate`, one healthy Take (`localDelta == 1`), kill Redis, further Takes: error is not nil (fails on DestBranch).
  2. Take, successful flush, kill Redis, Take: fails closed with an error (must keep passing).
  3. Exact mode, same outage: error still propagates.
  4. Two `Limiter`s, two clients, one Redis, killed mid-window: combined admitted count vs `limit`.

## Affected

- `windowcounter/limiter.go` (`Limiter` fields, `flushLoop`/`Sleep`/`Close`, `takeBuffered`/`windowLocked` as needed, `parseEvalInt`)
- `windowcounter/limiter_test.go`, `windowcounter/fake_redis_test.go` (killable fake; the four proofs)
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md`, `openspec/specs/std_go_windowcounter_sync-flush/spec.md`
- `README.md`, `New` comment, `knowledge/devdocs/std_go_windowcounter.md`

## Out of scope

- Other `simpleredisfixes2` findings (bug-01 pool panic, api-01 exporting sentinels, and the rest of that folder).
- Forcing a Redis GET on every buffered Take.
- Changing exact mode to ignore Redis errors.
- HTTP, identity, 429, or token-bucket.
- Implementing more than one of the three surfaces.
- Exporting `simpleredis` sentinel vars (api-01). Wrapping with the existing `RedisIssue` string is in scope.

## Unknowns

- Which of the three surfaces this run ships.
- The multiplier `k` if the staleness deadline is chosen.
- Whether buffered Peek with a pending delta must surface the same error (spec says Peek SHALL return unreachable; ticket names Take only).
- Whether a third `Take` return value is acceptable under Yaegi and existing callers.
- Kong Advanced / OSS behaviour when a buffered flush fails (research notes do not say).

## Tensions

- Spec `std_go_windowcounter_sliding-take` forbids a silent fallback on unreachable Redis. DestBranch buffered Take with `localDelta > 0` does that. The ticket’s characterisation (silent gap between ticks, not blanket fail-open) matches the code.
- Same spec says the library MUST NOT fail-open, fail-close, or health-gate inside Take. Ticket option 3 (staleness deadline) is a health-gate inside Take; option 2 is not. Ticket also says keep the decision explicit rather than silently changing Take, then lists option 3 as the best default.
- `peekCountLocked` hides the same outage for Peek; ticket does not ask to change Peek. Spec Peek unreachable scenario does.
- `parseEvalInt` wrap uses `simpleredis.RedisIssue` as today; exporting typed sentinels is a different finding.
