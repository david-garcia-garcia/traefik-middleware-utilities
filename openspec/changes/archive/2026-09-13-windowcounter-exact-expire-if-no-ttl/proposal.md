## Why

Exact `Take` (`sync_rate == 0`) only sends `EXPIRE` when `INCR` returns 1. If that expire fails, Redis already holds the key at count ≥ 1 with no TTL. Later Takes increment to 2+ and never expire, so the window never slides off.

## What Changes

- Exact Take uses one EVAL on `KEYS[1]`: `INCR`, then `EXPIRE` if `PTTL < 0` (no TTL), not only when the increment is 1.
- Do not refresh TTL on every hit. Do not `DEL` on expire failure. Buffered `flushScript` is unchanged.
- Create the failing repro first (`windowcounter/repro_expire_not_retried_test.go`, port fake `failNextExpireCommands` / `expireCommandCount`). Confirm FAIL on unfixed code, then implement the EVAL. The fake may store TTL / serve `PTTL` so a later Take still proves TTL is set (or atomic EVAL never leaves a no-TTL key).
- Keep `TestTake_ExpireOnFirstHit`. Spec and usage packet move from “TTL when increment returns 1” / “INCR + EXPIRE on first hit” to expire-if-no-TTL.

## Capabilities

### New Capabilities

- None. This is a delta on the existing windowcounter exact-sync leaf, not a new package or spec family.

### Modified Capabilities

- `std_go_windowcounter_sync-flush`: Exact mode sets TTL of two window lengths when the current-window key has no TTL (`PTTL < 0`), via one EVAL with the key in `KEYS`. It MUST NOT expire only when the increment is 1, MUST NOT refresh an existing TTL, and MUST NOT `DEL` on expire failure. Buffered flush EVAL is unchanged.

## Impact

- `windowcounter/limiter.go` (`takeExact` → EVAL; `flushScript` not rewritten).
- `windowcounter/fake_redis_test.go` (port expire-fail helpers; PTTL / EVAL expire-if-no-ttl).
- `windowcounter/repro_expire_not_retried_test.go` (create).
- `windowcounter/limiter_test.go` (`TestTake_ExpireOnFirstHit` stays).
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` (after archive).
- `knowledge/devdocs/std_go_windowcounter.md` (exact path is EVAL expire-if-no-TTL).
- No SimpleRedis public `PTTL` verb. No token bucket. No other windowcounter bugs.
