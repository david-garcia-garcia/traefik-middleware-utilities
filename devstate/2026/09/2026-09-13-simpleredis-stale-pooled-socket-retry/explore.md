# Explore
IssueKey: 2026-09-13-simpleredis-stale-pooled-socket-retry

## Concepts

**Idle vintage.** The set of sockets sitting on `idleConns` after a simultaneous warm. They share one fate: a Redis restart, failover, `CLIENT KILL`, or server idle-timeout closes every accepted fd at once. `takeIdleConn` still hands them out because they are younger than `IdleTimeout` (default 30s) and there is no liveness probe.

**Reuse vs dial.** `borrow` either pops a survivor from idle or calls `dial`. Today the three-value return (`conn`, `err`, `handshakeFailed`) does not tell `exec` which path ran. `handedOff` only means the in-use turn transferred to the caller.

**Corpse spend.** `exec` retries `errUnreachable` by calling `borrow` again. That pop is another idle survivor when the list is still full of the same vintage. Each command spends `MaxRetries+1` corpses and then returns `redis:unreachable`. Measured: PoolSize 8 / MaxRetries 1 → 4 sequential failures; PoolSize 32 → 16. Parallel callers hide it because they drain the list together and every retry then misses idle and dials.

**Force-dial.** After an I/O failure on a socket that came from idle, the next borrow on *this command* must skip `takeIdleConn` and dial. Otherwise the retry is another corpse. Hard bound: at most one extra send that does not consume `MaxRetries`, and `skipIdle` stays true only for the rest of this `exec` loop.

**Epoch / generation.** A counter bumped on the first reused EOF so parked sockets of the old vintage are discarded on the next `takeIdleConn` or `parkIdleConn`. Textbook pool invalidation. Not required for sequential recovery.

## Decisions

- **Simplicity gate: proceed.** The cheapest correct fix is small: keep `borrow`'s three-value signature (tests and sibling PRs stay on it), share the body with an unexported `skipIdle` flag, stamp whether the handed socket came from idle, and in `exec` treat one reused-socket `errUnreachable` as not evidence about the peer. No config knob, no generation counter, no idle-list wipe from `exec`. Moving parts: one unexported borrow helper, one bool on the socket or helper return, ~15 lines in `exec`, one default-suite test. That is fewer parts than the bug. Epoch is the stop-and-ask shape; we do not ship it.

- **Do not discard the leftover vintage.** Sequential success does not need a shared idle wipe. Each later quiet command spends one corpse, force-dials, and succeeds. Wipe would race with a concurrent `parkIdleConn` of a fresh socket and would touch the same mutex BUG-6 (idle reaper) is rewriting. Ticket desired is sequential success, not instant vintage invalidation.

- **Do not add a fourth `borrow` return.** The existing nolint keeps `error` before `handshakeFailed` because sibling PRs already unpack three values. Production `borrow` call site is one (`commands_exec.go`); tests that call `borrow` are `pool_test.go`, `pool_e2e_test.go`, `panic_safety_test.go`. Wrapper `borrow` → shared body keeps those call sites.

- **Do not change `takeIdleConn`.** BUG-6 owns that function. This change only skips calling it when `skipIdle` is true.

- **Default-suite proof** uses a bug-prefixed fake (`deadIdlePoolFake` or similar), reuses `holdGetsForTest` / `waitHeldGets` / `pooledIdle` / `assertTurnsFullAndNoOverFrees` from `fake_redis_test.go` / `pool_test.go` where they fit, and must keep its own accepted-socket slice so it can drop every server fd. `peerCloseFake` only closes the first accept; it cannot reproduce N corpses.

## Reproduction

Ran from the caller checkout (tagged files are untracked on dest):

`go test -tags simpleredis_bugs ./simpleredis/ -run 'TestBugDeadIdle' -v -count=1`

- `TestBugDeadIdlePoolFailsRequestsAfterPeerRestart` **FAIL**: `PoolSize=8 MaxRetries=1: 4 consecutive requests failed against a peer that was healthy the whole time`.
- `TestBugDeadIdleBurstScalesWithPoolSize` logs the table (4/-1 → 4; 4/1 → 2; 8/1 → 4; 16/1 → 8; 32/1 → 16; 8/3 → 2) and does not assert zero failures, so it PASSes.

Code on dest matches the report: `pool.go` returns a reused socket with no reuse signal; `commands_exec.go` retries `errUnreachable` by borrowing again.

```
  sequential Get
       │
       ▼
  borrow → takeIdleConn (LIFO)
       │
       ├─ reused corpse ── do ── EOF ── errUnreachable
       │                         │
       │                         ▼
       │              shouldRetry → borrow again
       │                         │
       │                         ▼
       │              next corpse (same vintage)
       │                         │
       │                         ▼
       └──────── attempts exhausted → redis:unreachable
                  leftover idle still corpses
```

After the cheap fix:

```
  sequential Get
       │
       ▼
  borrow (idle ok) → reused corpse ── EOF
       │
       ▼
  do not consume MaxRetries; skipIdle = true
       │
       ▼
  borrow skipIdle → dial (peer accepting) → success
```

## Open questions

- Q: When `MaxRetries` is `-1` (one send on dest), does a reused-socket I/O failure still get one hard-bounded force-dial?
  Rank: bounded asked — 1 production call site of the retry loop (`simpleredis/commands_exec.go` `exec`; searched `simpleredis/*.go` for `.borrow(`: exec plus 3 tests that stay on the wrapper); Desired on requirement.md: "that failure does not consume the retry budget, and the next attempt is forced onto a fresh dial"
	Decision: assumed — no. Remaining attempts of that command skip idle. MaxRetries still counts, so `-1` still fails that command. A free extra send would retry lost-reply Incr (write succeeded, then EOF), which on this platform is the same I/O as a dead unused socket. Default MaxRetries (1 extra) is enough for sequential recovery.
  By: implement
  By: explore

- Q: After a successful force-dial, do leftover idle corpses stay on the list?
  Rank: additive asked — no new shared mechanism; Desired names sequential success and a per-command force-dial, not vintage invalidation
  Decision: assumed — leave them. Each later sequential command spends one corpse then force-dials. Do not `discardIdle` from `exec` (shared-list mutation, park race, BUG-6 mutex).
  By: explore

- Q: How is "force a fresh dial" plumbed into `borrow` without a config knob and without colliding with BUG-6 on `takeIdleConn`?
  Rank: additive asked — Desired: "`borrow` reports whether the handed socket was reused" and "the next attempt is forced onto a fresh dial"; takeIdleConn body stays sibling-owned
  Decision: assumed — unexported shared body `borrowSocket(ctx, skipIdle bool)` (name may shift at apply). Package `borrow(ctx)` stays the three-value wrapper so tests and sibling diffs keep compiling. `skipIdle` true skips `takeIdleConn` and dials. Reuse is a return from that body (or a field set only on the idle-pop path), not a fourth value on `borrow`. After one reused unreachable, `skipIdle` stays true for the rest of that `exec` loop.
  By: explore

- Q: Does a fourth `borrow` return collide with in-flight sibling PRs?
  Rank: additive asked — requirement Unknowns; existing nolint on `pool.go` `borrow`
  Decision: resolved — no fourth return on `borrow`. Wrapper keeps the three-value signature.
  By: explore
