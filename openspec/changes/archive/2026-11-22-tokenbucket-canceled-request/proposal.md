# Proposal

## Why

Traefik answers 500 when the client is gone because its `select` cannot tell cancellation from a server error. The existence of that defect was taken from these upstream reports, not from an independent local finding: [traefik/traefik#14173](https://github.com/traefik/traefik/issues/14173), [traefik/traefik#13879](https://github.com/traefik/traefik/issues/13879), [traefik/traefik#13880](https://github.com/traefik/traefik/pull/13880) (open), and [traefik/traefik#14236](https://github.com/traefik/traefik/pull/14236) (closed as duplicate). This library already returns `context.Canceled` with `Allowed` false and does not write HTTP, so the 500 is not reproduced here. What is missing is GoDoc that a Redis `Canceled` / `DeadlineExceeded` does not prove the script did not run (the token may have been charged), GoDoc that Memory charges nothing on an already-done ctx, and issue-owned tests for those paths.

## What Changes

- No behavior change. `Allow` signatures, admit/deny, refund, Lua, and `simpleredis` stay as they are.
- GoDoc on `Redis.Allow`: a `context.Canceled` / `DeadlineExceeded` return does not prove the script did not run; the token may have been charged.
- GoDoc on `Memory.Allow`: a ctx already done returns its error and charges nothing.
- Both GoDocs mention that the HTTP mapping (499 on `Canceled`, no wait when `wait==0`) belongs to the middleware, citing [traefik/traefik#14173](https://github.com/traefik/traefik/issues/14173).
- New unit file `tokenbucket/traefik_issue14173_test.go`: `TestTraefik14173_MemoryCanceledContext` asserts `errors.Is(err, context.Canceled)`, `!status.Allowed`, and a full burst afterwards.
- New live file `tokenbucket/traefik_issue14173_e2e_test.go`: `TestTraefik14173_RedisCanceledInFlight` cancels through a holding TCP proxy, asserts `Canceled`, `!status.Allowed`, return within half the hold, and logs whether the token was charged. Same live gate as `TestLive_Limiter`. No time-based busy Lua loop.
- Helpers live in those two files with suffix `14173`. Reuse `runForEachLiveEngine` and `waitLiveClient` from `tokenbucket/limiter_e2e_test.go`. Do not create shared helper files. Do not copy other issues' tests.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

(none — no live contract. `std_go_tokenbucket_allow` already covers cancelled context → zero `Status`. GoDoc and tests bound this work. `skip_specs: true`.)

## Impact

- **Code**: GoDoc only on `tokenbucket/memory.go` (`Memory.Allow`) and `tokenbucket/redis.go` (`Redis.Allow`). No production logic, signature, or Lua change.
- **Tests**: new `tokenbucket/traefik_issue14173_test.go` (unit) and `tokenbucket/traefik_issue14173_e2e_test.go` (live Redis/Dragonfly). Existing `TestStatus_ErrorReturnsTheZeroStatus` stays as the Status-zero control; it does not replace these tests.
- **Docs**: GoDoc on `Allow`. Usage packet `knowledge/devdocs/std_go_tokenbucket.md` is out of this apply's minimum; `opd-devdocsimpact` measures it later.
- **Catalog**: no spec delta (`skip_specs: true`).
- **Unchanged**: `simpleredis` `Eval`/`exec`, HTTP from this package, Traefik's `select`/timer, other upstream issues, shared helpers, a Lua `TIME` busy loop.
