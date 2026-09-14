# Requirement
IssueKey: 2026-09-13-simpleredis-idle-arrival-desync

## Problem
A reply that arrives while a pooled SimpleRedis socket is idle is invisible to both `Buffered()` checks in `do`. The socket is parked and reused while sitting one reply ahead. Later commands return another key's payload with `err == nil`. Measured off dest: `Get(k2)` returned `"POISONED"`; other runs 3, 4, 6, 17 wrong over 59 commands, and once 4/4 consecutive with no eviction. A rate limiter then admits or denies on another window key's count.

## Current (code)
- `simpleredis/resp.go` `do` — pre-write (`Buffered() != 0` → `reusable=false`, `errUnreachable`) and post-read (`Buffered() != 0` → return the decoded value, `reusable=false`). The comment on the pre-write check states `Buffered()` does not see the kernel receive buffer. A stray that lands after the post-read check and before the next pre-write check is invisible to both.
- `simpleredis/pool.go` `takeIdleConn` / `borrow` — LIFO pop of a young idle socket. No read, no deadline probe, no kernel-queue check before reuse.
- `simpleredis/pool.go` `release` / `parkIdleConn` — `reusable=true` stamps `lastUsed` and parks. `runOnConn` in `simpleredis/commands_exec.go` defers `release` with that flag.
- `simpleredis/commands.go` `Get` — returns the one `exec` slot. No check that those bytes belong to `name`.
- `simpleredis/fake_redis_test.go` `startStrayExtraReplyFake` — writes value and stray in one `Write` so leftover is already in the reader at the reply boundary. Comment points at `knowledge/debt/2026-09-13-simpleredis-idle-arrival-desync.md`.
- `simpleredis/resp_test.go` `TestDesyncedSocketDoesNotServePreviousReplies`, `TestStrayExtraReplyIsNotPooled`, `TestAuthLeftoverIsNotParsedAsSelect` — cover leftover already in the reader. `simpleredis/yaegi_test.go` `TestYaegi_StrayExtraReplyOwnKey` is the interpreted same-write case.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` leftover requirement and `openspec/specs/std_go_simpleredis_resp-commands/spec.md` Get — leftover already in the reader is destroyed. Both explicitly do **not** cover an unsolicited reply that arrives only into the kernel receive buffer while the socket is idle.
- `knowledge/devdocs/std_go_simpleredis.md` — leftover unread RESP after a complete reply is discarded, not drained. No idle-arrival / kernel-queue rule.
- `knowledge/debt/2026-09-13-simpleredis-idle-arrival-desync.md` — exists on dest. Notes `Buffered()` cannot see kernel bytes; closing idle arrival needs a borrow-time readability probe; left as large debt from leftover-in-reader.
- `simpleredis/BUGS.md` — leftover-in-reader is recorded as fixed by `2026-09-13-simpleredis-desync-boundary-check`. Rejected row: no compliant-peer trigger for that leftover desync. Idle-arrival is not an open BUGS.md item.
- `simpleredis/PRODUCTION-BUGS.md` and `simpleredis/bugs_production_test.go` (`TestBugIdleArrivalDesyncReturnsAnotherKeysValue`, `startLateStrayFake`) — not found on dest. Present untracked in the caller workspace (build tag `simpleredis_bugs`).
- `simpleredis/bench_test.go` `BenchmarkGet` (and siblings) — existing hot-path benches; no idle-arrival probe cost on dest.
- `simpleredis/yaegi_test.go` loads `stdlib.Symbols` only. `e2e/simpleredisprobe/.traefik.yml` has no `useUnsafe`. `knowledge/research/ext_traefik_plugins_useunsafe/` — `syscall` symbols register only when both Traefik `useUnsafe` flags are true; this product must not set them.
- `simpleredis/simpleredis_e2e_test.go` `clientIDOnConnForTest` already calls `SetDeadline(time.Time{})` after a test `do` (compiled only).

## Desired
1. Close the silent wrong-data hole: after one unsolicited bulk while the socket is parked, every later Get returns its own key's value or a clean error — never another key's payload with `err == nil`.
2. Evaluate all three directions; pick at most one, or none. If the simplest correct fix is not small, coherent, and elegant, stop after propose with a written recommendation (that stop is success). Option 1 (zero-deadline one-byte read before reuse) is the most likely simple-and-correct candidate; measure `BenchmarkGet` before committing. Option 2 is reply-shape correlation (loud at the wrong-data point). Option 3 is loud failure without preventing desync.
3. Stdlib only, Go 1.21, Yaegi-interpretable. Do not use `syscall.Conn` / `RawConn` / poll unless `TestYaegi_*` proves it. Restore any deadline the probe changes.
4. Only `simpleredis`. Surgical diff (`resp.go` is shared with sibling agents). Comment style: constraint/reason, never restate. Keep in-use-turn sound, `OverFrees() == 0`, no fd or goroutine leaks. Do not regress the hot path.
5. Permanent untagged default-suite test that fails before the fix and passes after: fake peer emits one unsolicited bulk while parked, then every later Get is own-key or a clean error. Prefix any new fake/helper with a bug-specific name (must not collide with `startStrayExtraReplyFake` / untracked `startLateStrayFake`).

## Affected
- `simpleredis/resp.go` `do` and/or `simpleredis/pool.go` `takeIdleConn` / `borrow` (probe-before-reuse vs pre-write in `do`)
- New untagged test + bug-specific fake in `simpleredis/` (default suite)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` leftover exclusion and `std_go_simpleredis_resp-commands` Get exclusion (propose)
- `knowledge/devdocs/std_go_simpleredis.md` leftover gotcha (devdocs-impact)
- `knowledge/debt/2026-09-13-simpleredis-idle-arrival-desync.md` (close if this change takes the follow-up)
- Optional `simpleredis/yaegi_test.go` if the chosen API needs interpreted proof
- `simpleredis/bench_test.go` `BenchmarkGet` (measure; do not regress)

## Out of scope
- Other packages (`reclaim`, `tokenbucket`, `windowcounter`).
- PRODUCTION-BUGS.md bugs 1, 3, 4, 5, 6 and the tagged `simpleredis_bugs` suite (do not land that file unless required to cite).
- Leftover already in the reader (dest already destroys it).
- Draining leftover to resynchronise; RESP3 / HELLO; `unsafe`; cgo; generics; `useUnsafe`.
- Importing `syscall` / `RawConn` without Yaegi proof (research: Traefik dual `useUnsafe` gate; probe uses `stdlib.Symbols` only).
- Forcing a costly or inelegant fix past the simplicity gate.
- Other agents' `resp.go` hunks.

## Unknowns
- Whether option 1's extra syscall per reuse is small enough after `BenchmarkGet`, or the run should stop after propose (simplicity gate).
- Whether `SetReadDeadline(time.Time{})` plus a one-byte `Read` on `net.Conn` (then restore `IOTimeout`) is available and correct under Yaegi v0.16.1. Compiled `SetDeadline(time.Time{})` exists in `simpleredis_e2e_test.go`; no interpreted idle-probe exists.
- Probe placement: `takeIdleConn`/`borrow` vs `do` before `writeCommand`. A `bufio.Reader` read would pull kernel bytes into the reader; a raw `net.Conn.Read` bypasses it. Either is fine only if the socket is discarded on data/EOF.
- Whether a stray that arrives after the probe and before the write is still a residual hole (TCP race vs "parked").
- Whether option 2 can be a small shape check (GET bulk vs integer) without a per-verb correlation table, and whether "loud" without preventing desync is acceptable under the simplicity gate.
- Permanent test file name and fake prefix on dest (repro helpers are untracked / tagged).

## Tensions
- Ticket says `knowledge/debt/2026-09-13-simpleredis-idle-arrival-desync.md` does not exist. Dest has that file (leftover-in-reader change). Ticket still asks to close the hole the debt describes.
- Live specs and usage **exclude** kernel-buffer idle arrival. Ticket asks to close that exclusion. Ticket wins; spec/usage catch up only if a fix is chosen.
- Ticket cites `simpleredis/BUGS.md` as filing this as debt. Dest BUGS.md leftover item is the in-reader case already fixed. Idle-arrival is the debt file plus untracked PRODUCTION-BUGS.md BUG-2.
- Ticket: "needs a proxy" is not enough reason to leave it open. Same ticket: a costly fix is hard to justify, and stopping after propose is success. Propose must pick at most one option or none; do not force option 1 if measurement or Yaegi cost makes it inelegant.
- Ticket option 1 is "before reusing a parked socket" (borrow/idle), not a third `Buffered()` check. Dest already has the two `Buffered()` checks the ticket cites as the failing boundary.
- Five sibling agents; two also touch `resp.go`. Prefer a one-site probe over rewriting leftover destroy.
- Untracked `startLateStrayFake` vs dest `startStrayExtraReplyFake`: new helpers must use a bug-specific prefix so branches do not collide.
