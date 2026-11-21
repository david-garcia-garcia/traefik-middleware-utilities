# Design

## Context

See `proposal.md` (Why). Evidence, measurements and rejected options live in the run's `explore.md` (`devstate/2026/11/2026-11-21-simpleredis-bug-yaegi-select/`). Constraints that shape the approach:

- The session runs compiled (Go callers, unit tests) and interpreted under Yaegi v0.16.1 (Traefik local/plugin path). Interpreted, calling a method costs ≈ 1 µs and `select` is shared state across goroutines (`knowledge/research/ext_traefik_plugins_yaegi-select/`).
- `context.AfterFunc`, `context.WithTimeout`, `sync.Mutex` resolve under Yaegi v0.16.1 and `context.AfterFunc` already runs interpreted in `simpleredis/resp.go` `watchConnClose` (`knowledge/research/ext_traefik_plugins_yaegi-afterfunc/`). CI pins Go 1.21, which has `context.AfterFunc`.
- A plain channel receive or send (no `select`) is not affected by the shared-cases bug and measured 71 ns interpreted.
- Session source is stdlib-only and carries no build constraints (`std_go_simpleredis_tcp-session`).

## Goals / Non-Goals

**Goals:**

- Zero `select` statements in non-test `simpleredis` source, enforced by a test.
- Same observable contract: `ErrPoolWait` after `PoolTimeout`, ctx error on cancel/deadline (no log), over-free dropped + counted + `simpleredis_over_free`, backoff honours ctx.
- Interpreted behaviour proven inside `go test` for every replaced site, each with a compiled control.
- Hot path: Yaegi ≤ ≈ +0.3 % per `Get`, compiled neutral.

**Non-Goals:**

- `reclaim/table.go` `waitCtx` and a repo-wide guard (follow-up `knowledge/debt/2026-11-21-reclaim-waitctx-select.md`).
- Waking pool waiters on `Close` (not done today either).
- New CI job, `yaegi test` step, or new Pester case.
- Dropping the `-race` skip of `TestYaegiErrorpath_PoolWait` (its race stack also names `itype.refType`).

## Decisions

### D1. In-use-turn semaphore = mutex + counter + frozen cap + FIFO waiters

Fields on `SimpleRedis` (replacing `inUseTurns chan struct{}`):

```go
inUseMu     sync.Mutex
freeTurns   int           // free in-use turns
turnCap     int           // frozen at New; 0 = not from New
turnWaiters []*turnWaiter // FIFO; non-empty only while freeTurns == 0
```

```go
type turnWaiter struct {
	ready   chan struct{} // closed exactly once, under inUseMu
	granted bool          // written under inUseMu before close(ready)
}
```

Invariant: `freeTurns > 0 ⇒ len(turnWaiters) == 0`. A give-back with waiters hands the turn over in place, so the fast take never overtakes a waiter, and every `close(ready)` happens once under `inUseMu` (no `sync.Once`).

`turnCap == 0` replaces `inUseTurns == nil` in `liveCap`, `borrowSocket` (`errNotFromNew`) and `freeInUseTurn` (no-op). `ensureInUseTurns` sets `turnCap = freeTurns = liveCap()` once. `PoolSize()` / `OverFrees()` / `PoolTimeout()` keep their API.

Alternatives (measured, see explore): channel kept under a mutex (dead synchronisation, +70 ns Yaegi, keeps the term only); atomic CAS fast take (598 ns Yaegi, still needs a mutex + queue for the wait); a semaphore type with `tryTake`/`give` methods (≈ 2 300 ns Yaegi per turn: interpreted method call).

### D2. Fast take and give-back inline, no defer

- Fast take, inside `borrowSocket`: `Lock; if freeTurns > 0 { freeTurns--; took = true }; Unlock`.
- Give-back, body of the existing `freeInUseTurn`: `Lock`; waiters → pop `[0]`, `granted = true`, `close(ready)`; else `freeTurns < turnCap` → `freeTurns++`; else over-free (flag); `Unlock`; then, outside the lock, `overFrees.Add(1)` + `logger.Error("simpleredis_over_free")` when flagged.
- Explicit `Lock`/`Unlock`, no `defer`, no new method: under Yaegi each extra interpreted call or defer costs ≈ 1–2 µs (explore table: inline mutex 503 ns vs method 2 272–2 430 ns). The two critical sections cannot panic (int arithmetic, slice pop, `close` of a channel only this code closes).

### D3. Saturated wait: queued waiter + `WithTimeout` + one `context.AfterFunc` + plain receive

```go
w := &turnWaiter{ready: make(chan struct{})}
sr.inUseMu.Lock(); sr.turnWaiters = append(sr.turnWaiters, w); sr.inUseMu.Unlock()
waitCtx, cancel := context.WithTimeout(ctx, sr.inUseTurnWait())
stop := context.AfterFunc(waitCtx, func() { /* Lock; if w still queued: remove it, close(w.ready); Unlock */ })
<-w.ready
stop()
cancel()
// w.granted (read after <-ready: happens-after the close) → turn held, continue
// ctx.Err() != nil → return ctx.Err() (no log; exec owns that decision)
// contextStop(ctx) != nil → return it
// else Warn "borrowSocket: redis:unreachable" reason=pool_wait; return errPoolWait
```

- The fast take must re-check under the same lock before queueing (`freeTurns > 0` → take instead of queueing), so one `Lock` covers "take or enqueue".
- Abandon vs grant race: both run under `inUseMu`; whichever runs first removes `w` from the queue, the other sees it gone and does nothing. A granted waiter whose ctx also fired keeps the turn and continues (the later ctx checks in the command path release it), same as today's `select` choosing the turn case.
- `stop()` returning false means the callback ran or is running; it only touches the queue under the lock and `w` is already gone or closed, so no extra synchronisation is needed.
- Only this path pays for `WithTimeout` + `AfterFunc` (a waiter past `PoolSize`); the uncontended borrow allocates nothing new.
- Fallback (explore Q1): if implement measures `WithTimeout`+`AfterFunc` worse than `time.AfterFunc(PoolTimeout)` + `context.AfterFunc(ctx)` without `sync.Once`, use the latter with the same abandon callback. Record the numbers in the run.

Alternatives: `time.AfterFunc` ×2 + `sync.Once` (3 304 ns Yaegi; `Once` is redundant given the lock); timer only (cancel would not wake the waiter).

### D4. `waitUntil` via `WithTimeout` + `<-Done()`

```go
if delay <= 0 { return contextStop(ctx) }
waitCtx, cancel := context.WithTimeout(ctx, delay)
<-waitCtx.Done()
cancel()
if err := ctx.Err(); err != nil { return err }
return contextStop(ctx)
```

Same shape as D3 (child ctx with a deadline, wake on its `Done`). `time.Sleep` was rejected: it ignores cancel during backoff.

### D5. Source guard next to the imports guard

`TestSessionSourceHasNoSelect` in `simpleredis/simpleredis_test.go`, beside `TestSessionSourceImports`: same `os.ReadDir(".")` walk over non-test `.go`, full `parser.ParseFile`, `ast.Inspect` for `*ast.SelectStmt`, fail with `file:line` for each; fail when zero files scanned. A small table case parses an in-memory source with a `select` and asserts the walker reports it, so the guard cannot pass vacuously. Deterministic: this is the regression gate; the interpreted repros are behavioural evidence.

### D6. Interpreted regression tests (developer priority)

Files: `simpleredis/repro_yaegi_select_probe_test.go` (probes, no `testing` import, exported `SelectProbe*` funcs) and `simpleredis/repro_yaegi_select_crosstalk_test.go` (harness + `TestReproSelect_<Case>_Compiled` / `_Yaegi` pairs). The harness copies the probe file into the interpreted package as `zz_selectprobe.go` next to the sources `writeGopathSimpleredis` copies, so probes call the real unexported code under Yaegi v0.16.1.

| Pair | Covers | Pass criteria (each of `selectProbeRounds` = 3 rounds) |
|---|---|---|
| `WaitUntil` | D4 | `shortErr == 0`, `shortSlow == 0` (>100 ms), `longEarly == 0` (<550 ms) |
| `BorrowWait` | D3 timeout + ctx paths, turn held | `nilNil == 0`, `patientNotPoolWait == 0`, `patientEarly == 0` (<700 ms), `cancelLate == 0` (new: cancelled caller returns `context.Canceled` before `PoolTimeout`), `hung == 0` |
| `TwoPools` | D2 fast take + give-back on two clients | each client `free == cap`, `overFrees == 0` |
| `SaturatedGet` (new) | D2+D3 grant / abandon / timeout races through public `Get`; the path that panicked the process | no panic (interpreter `Eval` error or recovered panic counted), `nilNil`-equivalent 0, every error ∈ {pool wait, ctx error, classified unreachable}, at rest `free == cap`, `overFrees == 0`, `hung == 0` |

- Probes read and hold turns through helpers defined in the probe file itself (e.g. `inUseTurnsForTest(sr) (free, cap int)` under `inUseMu`, `holdTurnForTest(sr)`), because the interpreted package only sees what is copied. The five existing test files that read `inUseTurns` use the same helper.
- Probes MUST NOT use `select` themselves (the `hung` watchdog in `BorrowWait` uses a `done` channel closed by `time.AfterFunc` and a plain receive), so the interpreted probe does not depend on the primitive under test.
- `_Yaegi` tests skip on `raceDetectorOn` (race is inside `interp`); `_Compiled` run in `-short` and `-race`. Budget: ≈ 15 s added to `go test -short ./simpleredis/` (limit 2 min per test binary).
- Timing thresholds may be widened if load makes them flaky; the check itself stays.

### D7. Compiled semaphore tests in `pool_test.go`

FIFO handoff; racing grant vs abandon (timeout and cancel) with many goroutines → no lost/duplicate turn, at rest `free == cap`, `OverFrees() == 0`; cancel wakes a waiter before `PoolTimeout` with `ctx.Err()`; `PoolTimeout` → `ErrPoolWait` + no extra dial; over-free counted once and `free` stays `cap`; `&SimpleRedis{}` has `turnCap == 0`. These run under `Unit race`, which is where the new mutex is watched.

### D8. Measurement

Before numbers are in `explore.md` (`BenchmarkGet` 19 338 ns/op, 26 allocs/op; `BenchmarkGetParallel` 16 611 ns/op; `BenchmarkYaegiGet` 123 681 ns/op, range 92 523–142 421). After: same commands, median of 5 × 2 s. Acceptance: compiled medians within ±5 % and `Get` allocs/op unchanged; `BenchmarkYaegiGet` median inside the before range. `zz_selectbench_*` is not committed.

## Risks / Trade-offs

- [Grant/abandon race loses or duplicates a turn] → single lock for both, invariant D1, compiled race test under `-race`, interpreted `SaturatedGet`.
- [`context.AfterFunc` goroutine per saturated wait] → only past `PoolSize`; `stop()` + `cancel()` on every exit; close-cycle goroutine test stays flat.
- [Interpreted repros are probabilistic] → deterministic gate is the AST guard; local `-count=20` of the `_Yaegi` tests with zero failures before commit.
- [Yaegi hot path +≈331 ns per turn] → accepted by the requirement (≈ +0.3 %); measured after.
- [Spec/devdoc wording "in-use-turn channel" drifts] → delta renames to "in-use-turn semaphore"; usage doc updated in the same change.
- [Waiters are not woken by `Close`] → unchanged behaviour; they leave on `PoolTimeout` or ctx.

## Migration Plan

Internal only; no API or config change. Rollback = revert the change commit(s).

## Open Questions

None that change specs or tasks. D3 fallback is decided by measurement in implement.
