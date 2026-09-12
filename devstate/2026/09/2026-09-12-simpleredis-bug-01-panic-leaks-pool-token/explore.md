# Explore
IssueKey: 2026-09-12-simpleredis-bug-01-panic-leaks-pool-token

## Concepts

**In-use-turn token**: one slot on `SimpleRedis.inUseTurns` (`chan struct{}`, cap = `PoolSize`). `borrow` takes one before idle reuse or `dial`. `freeInUseTurn` is the only put-back. `release` is the sole caller after a command holds a socket.

**exec attempt**: `commands_exec.go` `exec` loop. After a successful `borrow`, it calls `do` then `release` with no `defer`. A panic in `do` (or anything it calls) skips `release`.

**reusable**: `do` returns whether the socket may return to idle. Zero value `false` means close the socket, then free the token. On panic that is the right default: framing is unknown.

**Panic source used for proof**: `readReply` `make([][]byte, count)` after `parseLen` accepts a non-negative length that still overflows `makeslice`. Canned header `*1000000000000000000\r\n` via `startStaticRedis`. Parser cap is bug-02, out of scope.

**Same-attempt release**: `defer` must run when this attempt’s `do` returns or panics, not at the end of `exec`. A function-scoped `defer` in `exec` would hold the token across retries. Ticket IIFE and a named helper are the same job.

```
borrow token
    |
    v
  do() ---- panic ---->  (today: token gone)
    |                     (wanted: release(reusable=false))
    v
release(reusable)
```

## Decisions

- Return the token from `exec` after a successful `borrow` even when `do` panics. Scope the `defer` to that attempt. On panic leave `reusable` false so `release` closes the socket.
- Do not `recover` in `exec`. Let the panic propagate (Desired 2).
- Implement the attempt-scoped defer as a named helper next to `exec` (job: run `do` then always `release`). Do not use a function-scoped defer in the retry loop. Ticket IIFE is the same job; a named owner matches One job, one owner.
- Prove in `simpleredis/pool_test.go` with `startStaticRedis` and `PoolSize: 2`: recover `PoolSize` panics, assert `len(inUseTurns) == cap(inUseTurns)`, then `borrow` succeeds. `MaxRetries: -1` so retries do not hide the invariant. Ticket’s `test-02-idle-cap-and-release-after-close.md` is not on dest.
- Do not wrap `dial` AUTH/SELECT `do` calls. Out of scope; noted as follow-up.
- Do not cap parser allocations (bug-02). Do not make `freeInUseTurn` non-blocking (risk-05).
- Usage packet `std_go_simpleredis.md` already names borrow/release and dirty sockets. Propose adds the panic-unwind scenario on `std_go_simpleredis_tcp-session`. No Language write this phase (terms already defined).

## Open questions

- Q: Does Traefik recover a plugin panic per request so the process keeps serving after this leak?
  Rank: additive asked — Desired 2 names do-not-recover; Traefik recovery is environment motivation, not a new branch
  Decision: assumed — do not recover in `exec`; let the panic propagate. Do not write Traefik-recovery research this run; the code change does not depend on that fact.
  By: explore

- Q: Does Yaegi interpret a deferred `release` the same as compiled Go for this helper?
  Rank: additive asked — Desired 1 names token return on panic unwind; dest already uses `defer` in `takeIdleConn` which Yaegi tests interpret
  Decision: assumed — compiled unit test is the invariant proof; existing Yaegi suite already interprets `defer` in this package. Do not add a Yaegi-only panic case unless implement’s Yaegi run fails.
  By: explore
