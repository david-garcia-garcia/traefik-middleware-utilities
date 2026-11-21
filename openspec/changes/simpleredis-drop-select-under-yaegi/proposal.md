# Proposal

## Why

Yaegi v0.16.1 (`interp/run.go` `func _select`) allocates one `[]reflect.SelectCase` per `select` statement and every execution overwrites it without a lock, so two goroutines on the same statement can wait on, or receive from, each other's channels. Under Traefik that turns SimpleRedis' three production `select` statements into cross-goroutine bugs, reproduced on this tree: `borrowSocket` returns `(nil, nil)` and a public `Get` panics the process (`resp.go:19` frame via `release` → `close`), in-use turns leak between two clients with false `simpleredis_over_free`, and a 1 ms retry backoff ends in `DeadlineExceeded` because another goroutine stole its tick. Compiled code is unaffected, which is why the existing unit suite never saw it.

## What Changes

- Replace the in-use-turn channel (`inUseTurns chan struct{}`) and its two `select` sites (fast take + saturated wait in `borrowSocket`, non-blocking give-back in `freeInUseTurn`) with a mutex-guarded turn counter frozen at `New` plus a FIFO queue of waiters. Fast take and give-back stay inline (no new method, no `defer`, no `sync.Once`).
- The saturated pool wait parks on a per-waiter `ready` channel with a plain receive; `context.WithTimeout(ctx, PoolTimeout)` plus one `context.AfterFunc` withdraws the waiter on timeout or cancel. Observable classification is unchanged: turn granted → continue; caller ctx fired → `ctx.Err()` without a log line; otherwise `pool_wait` Warn + `ErrPoolWait`.
- `waitUntil` (retry backoff) waits on `<-child.Done()` of `context.WithTimeout(ctx, delay)` instead of `select` over `ctx.Done()` / `timer.C`.
- Add a source guard: a test fails with `file:line` when any non-test `simpleredis` file contains a `select` statement.
- Keep the interpreted reproductions as permanent regression tests (compiled control + Yaegi pair per site), plus a public-API saturated `Get` pair that covers the panic path. Add compiled semaphore tests (FIFO handoff vs abandon races, ctx wake, `PoolTimeout`, over-free, at-rest full).
- Not a breaking change: no exported API, error text, sentinel, log event, or `Config` knob changes.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: requirements that name the "in-use-turn channel" (New, full pool wait, client not from New, peer-closed idle vintage, extra turn return, chaos pool, close cycles, panic between borrow and release, panic inside idle keep-or-close) now name the in-use-turn semaphore; ADDED: session source contains no `select` statement, and interpreted concurrency (two clients, saturated wait, retry backoff) never crosses turns, deadlines, or wake-ups between goroutines.
- `std_go_simpleredis_resp-commands`: "Concurrent MSetEX unknown-command fallback stays balanced" names the in-use-turn semaphore instead of the channel.

## Impact

- Code: `simpleredis/pool.go` (`liveCap`, `ensureInUseTurns`, `freeInUseTurn`, `borrowSocket`), `simpleredis/simpleredis.go` (fields), `simpleredis/commands_exec.go` (`waitUntil`).
- Tests: new `simpleredis/repro_yaegi_select_crosstalk_test.go` + `repro_yaegi_select_probe_test.go`; new guard in `simpleredis/simpleredis_test.go`; new semaphore tests in `simpleredis/pool_test.go`; five test files that read `inUseTurns` migrate to a test reader.
- Docs: `knowledge/devdocs/std_go_simpleredis.md` lines that describe the channel.
- Performance: expected Yaegi +≈331 ns per uncontended turn (≈ +0.3 % of `BenchmarkYaegiGet`), compiled neutral; measured before/after.
- Out: `reclaim/table.go` `waitCtx` (follow-up `knowledge/debt/2026-11-21-reclaim-waitctx-select.md`), CI jobs (unchanged), `zz_selectbench_*` (not committed), Yaegi upgrade or patch, `windowcounter` `flushAfterTick` recover.
