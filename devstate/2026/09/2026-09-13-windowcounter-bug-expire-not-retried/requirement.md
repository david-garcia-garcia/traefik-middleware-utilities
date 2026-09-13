# Requirement
IssueKey: 2026-09-13-windowcounter-bug-expire-not-retried

## Problem

Exact `Take` (`sync_rate == 0`) sends `EXPIRE` only when `Incr` returns 1. If that `Expire` fails, Redis already holds the key at count ≥ 1 with no TTL. Later Takes `Incr` to 2+ and never `Expire`. The key never slides off.

## Current (code)

- `windowcounter/limiter.go:155-165` — `takeExact` `Incr`s the current-window key, then `Expire`s only when `current == 1`. Expire error returns; the incremented key is left in Redis.
- `windowcounter/limiter.go:161-165` — a later Take with `current != 1` skips `Expire` entirely.
- `windowcounter/limiter.go:18-24` — buffered `flushScript` is a separate EVAL (`EXISTS` then `INCRBY`, `EXPIREAT` only when the key did not exist). Exact Take does not use it.
- `windowcounter/limiter.go:129-130` — `sync_rate == 0` is the only path into `takeExact`.
- `windowcounter/limiter_test.go:65-82` — `TestTake_ExpireOnFirstHit` Takes once on a healthy fake and asserts a standalone `EXPIRE` argv with TTL 20. Does not fail the first Expire or Take again.
- `windowcounter/fake_redis_test.go:70-96` — fake `EXPIRE` always `:1`; `EVAL` implements `flushScript` only; unknown EVAL returns `:0`. No `PTTL`. No `failNextExpireCommands` / `expireCommandCount` (`not found` on dest).
- `windowcounter/repro_expire_not_retried_test.go` — `not found` on `origin/master`.
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md:8-15` — exact mode SHALL set TTL of two window lengths when the increment returns 1.
- `knowledge/devdocs/std_go_windowcounter.md:22` — zero `sync_rate` is `INCR` + `EXPIRE` on first hit.
- `simpleredis/commands.go` — `Incr` / `Expire` exist; no `PTTL` client verb.
- `simpleredis/commands_eval.go:28-34` — `Eval` EVALSHA then EVAL on `NOSCRIPT`. Exact Take does not call it.

## Desired

- Exact `Take` uses one EVAL on `KEYS[1]`: `INCR`, then `EXPIRE` if `PTTL < 0` (no TTL), not only when the increment is 1. Do not refresh TTL on every hit. Do not `DEL` on expire failure. Buffered `flushScript` unchanged.
- Test must prove a later Take still sets TTL (or the atomic EVAL never leaves a no-TTL key).
- Implement order (do not implement in prepare):
  1. CREATE the failing repro FIRST: `windowcounter/repro_expire_not_retried_test.go`. Port fake helpers `failNextExpireCommands` / `expireCommandCount` from parent `windowcounter/fake_redis_test.go`. Confirm FAIL on unfixed code.
  2. Then implement the EVAL. The fake may need `PTTL` / EVAL expire-if-no-ttl so the test still proves TTL is set.
  3. Confirm that test PASSES and `go test -short -count=1 -timeout 60s ./windowcounter` passes. Keep `TestTake_ExpireOnFirstHit`.

## Affected

- `windowcounter/limiter.go` (`takeExact` → EVAL; `flushScript` not rewritten)
- `windowcounter/fake_redis_test.go` (port `failNextExpireCommands` / `expireCommandCount`; possibly PTTL / EVAL expire-if-no-ttl)
- `windowcounter/repro_expire_not_retried_test.go` (create)
- `windowcounter/limiter_test.go` (`TestTake_ExpireOnFirstHit` stays)
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` (TTL when incr returns 1 vs expire-if-no-TTL)
- `knowledge/devdocs/std_go_windowcounter.md` (exact path is EVAL, not INCR-then-EXPIRE-on-1)

## Out of scope

- Other windowcounter bugs (sleep/wake race, mutex across GET, buffered outage, Peek)
- Changing buffered `flushScript`
- `DEL` on expire failure
- Refreshing TTL on every hit
- Token bucket / SimpleRedis `PTTL` as a public client verb
- Implementing the repro or EVAL in prepare

## Unknowns

- Whether the unit fake stores TTLs and serves `PTTL`, or only records `EXPIRE` inside the new EVAL, to prove a later Take still sets TTL.
- How a “first Expire fails, second Take sets TTL” repro maps onto one atomic EVAL (INCR and EXPIRE cannot split). Ticket allows proving EVAL never leaves a no-TTL key instead.
- Whether `TestTake_ExpireOnFirstHit` still sees a standalone `EXPIRE` argv after Take moves to EVAL (`redis.call("EXPIRE")` is not a client EXPIRE).

## Tensions

- Spec (`std_go_windowcounter_sync-flush` line 8) and usage packet say set TTL when the increment returns 1. Agreed how is `PTTL < 0`, which also covers a key left from a failed expire (`count >= 1`, no TTL). Spec/devdocs must move with Take; do not keep the incr==1-only rule.
- `TestTake_ExpireOnFirstHit` asserts a client `EXPIRE` command. An EVAL-only Take will not send that verb unless the fake treats Lua EXPIRE as one. Keep the test; do not drop it.
- Parent checkout already has a split-command repro (first Take errors, second Take must send EXPIRE). Dest does not. After EVAL, that first-Take error cannot occur as two commands; honour the ticket’s “later Take still sets TTL **or** atomic EVAL never leaves a no-TTL key.”
