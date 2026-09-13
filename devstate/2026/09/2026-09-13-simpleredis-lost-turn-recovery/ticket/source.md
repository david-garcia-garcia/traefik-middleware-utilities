# Fix: one lost in-use turn permanently bricks the SimpleRedis client

Root cause: `exec` in `simpleredis/commands_exec.go` releases the pooled connection WITHOUT `defer`, deliberately — the comment cites PR #29. `inUseTurns` is a fixed buffered channel created once in `New` via `ensureInUseTurns` (`simpleredis/pool.go`) and NOTHING ever refills it. There is no reaper, no generation counter, and no recovery on the idle sweep. `OverFrees` counts EXTRA returns and cannot help with MISSING ones. So every panic between `borrow` and `release` shrinks the client's effective concurrency by one, forever, and after `PoolSize` of them the client is dead for the process lifetime. The symptom is `redis:unreachable` on every command with a HEALTHY Redis and zero open sockets, which looks exactly like a Redis outage.

This is the Traefik shape precisely: Traefik recovers a panicking middleware per request and keeps serving, so the process survives and quietly loses a turn each time. A plugin that panics once an hour with PoolSize 8 is fully dead in 8 hours.

Measured: PoolSize 2, one warm socket, then two recovered panics between borrow and release → `len(inUseTurns)` = 0/2, idle 0, TCP accepts 2, next `Get` = `redis:unreachable`, permanent.

Be honest about severity in the write-up: NO reachable panic was found in compiled Go. `readReply` was fuzzed 7.04M execs over 91s with no panic, and `parseLen` cannot overflow `length+2` because `maxBulkLength` caps it before the allocation. So this is a LATENT hazard with unbounded blast radius, not a live crash. But Yaegi panics are demonstrated in this package three times over in code comments (`errors.As` on a package-local struct, a `net.Error` type assert, and `interp._select` racing on a context channel), so the hazard is real.

Spec: openspec/specs/std_go_simpleredis_tcp-session/spec.md, "Idle connections are pooled" — live sockets (idle plus checked out) SHALL not exceed PoolSize, and when idle is empty and live sockets are at PoolSize a caller waits for a released socket instead of dialing. With ZERO sockets actually live, refusing to dial is not backpressure, it is a dead client.

Reference material already pushed — read this first:
  Branch `origin/bugfixes20260913` carries the bug report and a VERIFIED FAILING reproduction:
  git show origin/bugfixes20260913:simpleredis/BUGS.md
  git show origin/bugfixes20260913:simpleredis/bugs_repro_test.go
BUGS.md **section 2** is this bug. bugs_repro_test.go is build-tagged `//go:build bugrepro` and holds `TestBugLostInUseTurnBricksPoolPermanently` plus its helper `bugPanicAfterBorrow`, confirmed to fail on current code for the right reason. Reuse that code as the starting point. Neither file exists on `master`; we are porting from that branch, not deleting anything. Do NOT create or edit simpleredis/BUGS.md.

Agreed fix direction (improve on it if you can, but justify on the card):
Do NOT simply switch to `defer sr.release(...)`. PR #29 rejected that. Read it first (`gh pr view 29 --repo david-garcia-garcia/traefik-middleware-utilities`), and if you still believe defer is correct, argue it on the delivery card against what PR 29 actually said.

Otherwise fix the PERMANENCE, which is orthogonal and cheap:
- Cheapest sufficient version: when `borrow` is about to fail with `errPoolWait`, check whether the client actually owns any socket (`len(idleConns)` plus a counter of dialed-but-not-yet-released sockets). Zero live sockets with zero free turns is impossible unless turns leaked → refill to the frozen cap and count it on a new `LostTurns()` metric.
- Or make the turn a leased resource so a lost lease is recoverable.
- Expose the leak: `LostTurns()` next to `OverFrees()` in `simpleredis/simpleredis.go` so a bricked client is diagnosable instead of looking like a Redis outage.

Invariants you MUST preserve:
- `PoolSize` stays the frozen live-socket cap. Recovery must NOT fire when sockets genuinely ARE live and busy — that would breach PoolSize under load. You must distinguish "no live sockets" from "all live sockets busy". This is the crux of the change; get it right and test it.
- `TestOverFreeAccountingStaysBalanced` must still end with `len(inUseTurns) == cap(inUseTurns)` and `OverFrees() == 0`.
- `TestPoolWaitTimesOutWithoutExtraDial` requires a saturated pool to return `redis:unreachable` within ONE PoolTimeout and open no extra connection.
- Keep green: `TestConcurrentCommandsStayWithinPool`, `TestBurstGetsStayWithinLiveCap`, `TestOverlappingCallersDoNotDialPastLiveCap`, `TestReleaseKeepsSocketWhenLiveUnderCap`, `TestGetCancelWhileWaitingForTurn`, `TestOverFreeOnFullSemaphoreReturns`.

Tests you must land (permanent and untagged, running in the default suite):
1. Port `TestBugLostInUseTurnBricksPoolPermanently` into a permanent regression test that PASSES after the fix. It takes a turn via `sr.borrow(...)` then panics with a `recover()` in the caller, which is byte-for-byte what Traefik's per-request recovery does to `exec`.
2. A test that recovery does NOT fire while sockets are genuinely busy: a saturated pool with live in-flight commands must still cap at PoolSize and still return pool-wait `redis:unreachable`, never dial past the cap.
3. A test that `LostTurns()` (or whatever you expose) reports the leak so it is diagnosable.

Package constraints (from openspec/specs/std_go_simpleredis_tcp-session/spec.md):
- simpleredis source imports ONLY the Go standard library. No go-redis, no miniredis, no vendor pkgs.
- No `unsafe`, no cgo, no type parameters (generics).
- Retry jitter stays stdlib `math/rand` `Int63n`, NOT `math/rand/v2`.
- The package runs interpreted under Yaegi. Avoid constructs Yaegi breaks on: `errors.As` on a package-local struct, a `net.Error` type asserts, and interpreted code selecting on a context channel from a goroutine (hence `context.AfterFunc` in resp.go). Keep those workarounds intact. If you add a helper on the client, prefer plain fields, atomics, and mutexes over anything reflective.
- Pool/timeout/retry knobs are frozen at New on Config; SimpleRedis must not export writable ones. A new READ-ONLY accessor like `LostTurns()` is fine; a writable field is not.

NOTE: another agent is concurrently reworking the `handshakeFailure` type in `pool.go` and `shouldRetry` in `commands_exec.go` (removing that struct and passing a handshake bool out of band from `dial` through `borrow`). Keep hunks away from that code. This change is about turn accounting, not error classification.
Do NOT touch `simpleredis/resp.go`.
