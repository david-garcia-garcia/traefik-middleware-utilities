## Context

Dest `exec` calls `do` then `release` with no `defer`. Dest `borrow` frees the turn only on the closed-idle and dial-error returns. Session source is stdlib-only, no generics, no goroutines added, Yaegi workarounds kept. Proceed policies: `devstate/explore.md`. See proposal.md for why.

`origin/proto-defer-net` already has this design on an older master. Copy those two product edits onto dest. Do not merge that branch (it predates #68 and would delete `simpleredis/BUGS.md`).

## Goals / Non-Goals

**Goals:**
- Deferred `release` so a panic inside `do` returns the turn and closes the socket.
- Deferred `freeInUseTurn` on `borrow` paths that never hand a socket to the caller.
- Permanent compiled panic test and Yaegi defer-on-panic probe with package-conventional names.

**Non-Goals:**
- Leak-detection / turn-refill machinery (closed #66 / #70).
- Editing `simpleredis/BUGS.md` (#68).
- Pre-empting OPEN #67's `handshakeFailed` on `borrow`.
- Merging `origin/proto-defer-net` wholesale.

## Decisions

1. **`runOnConn` with `reusable` starting false, declared before the defer.** The closure must capture the variable, and `values, reusable, err = sr.do(...)` (plain `=`, named returns) so the closure sees the update. Alternative: defer with a pointer to a stack bool — same capture, more noise. Alternative: always `release(conn, false)` on panic via recover in `exec` — that is a second recover next to Traefik's, and Yaegi may convert the panic to an Eval error so the recover would not run; defer still runs.

2. **`borrow` `handedOff` plus one defer; remove the two explicit `freeInUseTurn()` calls.** Keeping both double-frees and inflates `OverFrees`. Alternative: keep the explicit calls and only defer on panic — then the error paths and the panic path diverge, and a panic after an explicit free still over-frees if both run.

3. **Tests in new files, not `pool_test.go`.** `panic_safety_test.go` owns the panicking-writer injection (`TestPanicInDoReturnsTurnAndClosesSocket`). `yaegi_defer_test.go` owns `TestYaegi_DeferRunsOnPanic` using existing `writeGopathFile`. Alternative: append to `pool_test.go` / `yaegi_test.go` — those files already mix unrelated proofs; a dedicated file names the domain (`std_go_test-suites.md`).

4. **Keep dest `contextStop` after `do` inside `runOnConn`.** Dest currently releases with `reusable=false` when the deadline passes after a successful read. That specified cancel-closes-socket behavior stays: `runOnConn` sets `reusable = false` on `contextStop` then returns.

5. **Yaegi pin is dest's pin.** `go.mod` and Traefik v3.7.11 both require Yaegi v0.16.1. No defensive detection if some other Traefik ships an older interpreter.

## Risks / Trade-offs

- [Mechanical conflict with OPEN #67 on `borrow`] → Mitigation: apply `handedOff` on dest's current signature; do not take #67's `handshakeFailed` bool.
- [Double-free if explicit `freeInUseTurn` is left in] → Mitigation: delete those two calls; panic test asserts `OverFrees()==0`.
- [Existing tests fail if release timing changes] → Mitigation: do not edit pre-existing tests; investigate any red as a behavioural regression.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy key.

## Open Questions

None. The Yaegi version question is resolved on `devstate/explore.md`.
