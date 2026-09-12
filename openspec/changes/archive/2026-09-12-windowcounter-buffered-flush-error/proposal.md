## Why

On DestBranch, buffered `windowcounter` (`sync_rate > 0`) discards every `flushPending` error. While a local delta is pending, `Take` and `Peek` never talk to Redis, so a Redis outage looks like a healthy admit. Each instance still stops at its own `limit`; N processes therefore admit about `limit × N` with a nil error. Exact mode already returns `redis:unreachable`. Sliding-take already forbids a silent fallback.

## What Changes

- Retain the flush error on the limiter (`lastFlushErr`, `flushFailedAt`, `lastRedisOK`) instead of `_ = flushPending()` in `flushLoop`, `Sleep`, and `Close`.
- Staleness k=1: if `now.Sub(lastRedisOK) >= syncRate`, Take/Peek probe with one `flushPending` (not GET every call). When `lastFlushErr` is set, return it on the existing `(bool, float64, error)` slot.
- Peek uses the same error surface as Take.
- Buffered Take still increments `localDelta` and still returns the local admit decision beside the error. Exact mode is unchanged.
- Wrap `parseEvalInt` conversion failures so the cause is not the bare `redis:issue?` string.
- Killable in-process RESP fake (close listener and live sockets). Four proofs: pending-delta outage, post-flush outage, exact-mode outage, two-instance combined admit vs `limit`.
- Document exact vs buffered failure next to `syncRate` in `New`, README, and `knowledge/devdocs/std_go_windowcounter.md`.
- No `LastFlushError()` / `Stale()`. No fourth return. No GET on every buffered Take.

## Capabilities

### New Capabilities

- None. This is a delta on the existing windowcounter leaves, not a new package or spec family.

### Modified Capabilities

- `std_go_windowcounter_sliding-take`: Redis errors propagate for buffered Take and Peek as well as exact. Unreachable MUST NOT be a silent nil while a local delta is pending. Unit tests include a killable fake and the pending-delta outage.
- `std_go_windowcounter_sync-flush`: Failed flush is retained. After one missed `sync_rate` with no successful Redis contact, Take/Peek probe with flushPending. EVAL integer parse wraps the conversion cause. `New` and README state per-mode failure. Two-instance outage still cannot exceed `limit` without an error.

## Impact

- `windowcounter/limiter.go` (flush error fields, Take/Peek stale probe, `parseEvalInt`).
- `windowcounter/limiter_test.go`, `fake_redis_test.go` (Kill of listener and live sockets).
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` and `std_go_windowcounter_sync-flush/spec.md` (after archive).
- `README.md`, `knowledge/devdocs/std_go_windowcounter.md`.
- No new package. No other `simpleredisfixes2` findings. No exporting `simpleredis` sentinels. No HTTP/429. No tokenbucket. Take signature unchanged.
