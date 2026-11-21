# Tasks

Interpreted (Yaegi v0.16.1) coverage is the developer's top priority: every replaced `select` site gets a `_Yaegi` regression test beside a `_Compiled` control, plus the deterministic source guard. Design refs: `design.md` D1–D8.

## 1. Interpreted repro on the worktree (red before the fix)

- [ ] 1.1 Copy `repro_yaegi_select_probe_test.go` and `repro_yaegi_select_crosstalk_test.go` from the main checkout (`D:\repositories\traefik-middleware-utilities\simpleredis\`) into the worktree `simpleredis/`; do not copy `zz_selectbench_*`, `zz_scratch_*`, `bugs_production_test.go`, or any `BUGS.md`. Verify: `go test ./simpleredis/ -run 'TestReproSelect_' -count=1 -v` → all `_Compiled` PASS, `_Yaegi` FAIL (WaitUntil `shortErr`, BorrowWait `nilNil`, TwoPools `free`/`overFrees`), matching explore.
- [ ] 1.2 Add the test-only turn helpers to the probe file (`inUseTurnsForTest(sr) (free, cap int)`, `holdTurnForTest(sr)`), written against the current channel for now, and switch the probes to them; replace the `select` watchdog in `SelectProbeBorrowWait` with a `done` channel closed by `time.AfterFunc` + plain receive. Verify: 1.1 command gives the same PASS/FAIL pattern.
- [ ] 1.3 Add a `cancelLate` counter to `SelectProbeBorrowWait` (callers whose ctx is cancelled ~50 ms in must return `context.Canceled` well before the 800 ms `PoolTimeout`) and assert it in `borrowWaitAnomalies`. Verify: `TestReproSelect_BorrowWait_Compiled` PASS.
- [ ] 1.4 Add the `SaturatedGet` pair: `SelectProbeSaturatedGet(host)` drives public `Get` from 24 goroutines on a client with `PoolSize` 2, `PoolTimeout` 20 ms, ctx 1–5 ms, for a bounded duration; reports `calls`, `panics` (recovered per call), `unexpectedErr` (not pool wait / ctx error / classified unreachable), `free`, `cap`, `overFrees`, `hung` (watchdog without `select`). Add `TestReproSelect_SaturatedGet_Compiled` / `_Yaegi` (`_Yaegi` skips on `raceDetectorOn`). Verify: `_Compiled` PASS; `_Yaegi` FAIL or panics on current code (record which in the run).

## 2. Source guard (deterministic gate)

- [ ] 2.1 Add `TestSessionSourceHasNoSelect` to `simpleredis/simpleredis_test.go` beside `TestSessionSourceImports`: parse every non-test `.go` in the package, `ast.Inspect` for `*ast.SelectStmt`, fail with `file:line` per hit, fail when zero files scanned; plus a table case that parses an in-memory source containing a `select` and asserts the walker reports it. Verify: on current code it FAILS naming `pool.go:79`, `pool.go:126`, `pool.go:131` and `commands_exec.go:143` (four `select` statements); the in-memory case PASSES.

## 3. In-use-turn semaphore (D1–D3) with compiled tests

- [ ] 3.1 Replace `inUseTurns chan struct{}` in `simpleredis/simpleredis.go` with `inUseMu`, `freeTurns`, `turnCap`, `turnWaiters []*turnWaiter` and the `turnWaiter` type (`ready`, `granted`); update field comments (no "channel"). `ensureInUseTurns` sets `turnCap = freeTurns = liveCap()` once; `liveCap` returns `turnCap` when non-zero. Verify: `go build ./simpleredis/`.
- [ ] 3.2 Rewrite `freeInUseTurn` per D2 (inline Lock/Unlock, hand-off to `turnWaiters[0]` with `granted = true` + `close(ready)`, else `freeTurns++` below cap, else over-free; `overFrees.Add(1)` + `simpleredis_over_free` log outside the lock; no-op when `turnCap == 0`). Verify: existing over-free tests in `pool_test.go` and `slog_test.go` PASS.
- [ ] 3.3 Rewrite the turn take in `borrowSocket` per D1–D3: `turnCap == 0` → `errNotFromNew`; one `Lock` that either takes a free turn or enqueues a waiter; saturated path `context.WithTimeout(ctx, inUseTurnWait())` + `context.AfterFunc` abandon (removes `w` under lock and closes `ready` only if still queued) + `<-w.ready` + `stop()` + `cancel()`; classification granted → continue / `ctx.Err()` / `contextStop` / `pool_wait` Warn + `errPoolWait`. Keep the existing `handedOff` defer for the post-take path. No `select`, no new method, no `sync.Once`. Verify: `go test -short ./simpleredis/ -run 'Pool|Borrow|Deadline|Cancel|NotFromNew|Panic|Slog|Chaos' -count=1` PASS.
- [ ] 3.4 Migrate the five test files that read `inUseTurns` (`commands_deadline_test.go:179`, `not_from_new_test.go:12-13`, `panic_safety_test.go:44,46,68`, `pool_test.go:714-718`, comment in `chaos_pool_test.go:159-162`) and the probe helpers from 1.2 to the new fields (free/cap read under `inUseMu`; `turnCap == 0` for not-from-New). Verify: `grep -rn inUseTurns simpleredis/` returns nothing; package compiles.
- [ ] 3.5 Add compiled semaphore tests to `simpleredis/pool_test.go` (D7): FIFO handoff to the oldest waiter; many goroutines racing grant vs timeout/cancel abandon → at rest `free == cap`, `OverFrees() == 0`, no waiter left queued; cancel wakes a waiter before `PoolTimeout` with `context.Canceled`; `PoolTimeout` → `ErrPoolWait` and no extra dial; extra return counted once and `free` stays `cap`; `New` with `PoolSize` 3 → cap 3 and full. Verify: `go test -short ./simpleredis/ -run 'Turn|Pool' -count=1` PASS (CI `Unit race` runs them under `-race`; local Windows has no gcc).

## 4. Retry backoff without select (D4)

- [ ] 4.1 Rewrite `waitUntil` in `simpleredis/commands_exec.go` as `WithTimeout(ctx, delay)` + `<-Done()` + `cancel()`; `ctx.Err()` first, then `contextStop(ctx)`; keep `delay <= 0 → contextStop(ctx)`. Verify: `go test -short ./simpleredis/ -run 'Retry|Backoff|Deadline|WaitUntil' -count=1` PASS and `TestSessionSourceHasNoSelect` now PASSES.

## 5. Interpreted regression green (Yaegi)

- [ ] 5.1 Run every interpreted pair after the fix: `go test ./simpleredis/ -run 'TestReproSelect_' -count=1 -v` → all 8 tests (4 `_Compiled` + 4 `_Yaegi`) PASS, 3 rounds each.
- [ ] 5.2 Stability pass before committing: `go test ./simpleredis/ -run 'TestReproSelect_.*_Yaegi' -count=20` with zero failures. If a timing threshold (`shortSlow` > 100 ms, `patientEarly` < 700 ms, `longEarly` < 550 ms, cancel window) fails under load, widen the threshold, never drop the check; record the run in the change's verification notes.
- [ ] 5.3 Existing interpreted suites stay green under the new semaphore: `go test -short ./simpleredis/ -run 'Yaegi' -count=1 -v` (`yaegi_test.go`, `yaegi_errorpath_test.go` incl. `TestYaegiErrorpath_PoolWait`, `yaegi_defer_test.go`, `interpretedcost_test.go` conversion matrix, `TestYaegi_StructuredLogging`). Leave the `-race` skip on `TestYaegiErrorpath_PoolWait` as is.
- [ ] 5.4 Time budget: `go test -short -count=1 ./simpleredis/` stays well under the 2-minute `Unit` timeout (baseline 18.9 s + ≈ 15 s for the repros); record the measured duration.

## 6. Usage docs

- [ ] 6.1 Update `knowledge/devdocs/std_go_simpleredis.md` where it describes the in-use-turn channel (pool bullets around lines 87–88: `New` builds the semaphore, over-free wording, `len(inUseTurns)` at-rest note) and add a Gotcha: no `select` in session source under Yaegi v0.16.1 (guard test name, research link). Update `knowledge/devdocs/std_go_test-suites.md` mentions of `sr.inUseTurns` / "inUseTurns length" (lines ~170, ~199) to the semaphore wording. Verify: `grep -n "inUseTurns\|in-use-turn channel" knowledge/devdocs/` returns nothing.

## 7. Integration checks

- [ ] 7.1 Full unit suite: `go vet ./...` and `go test -short -timeout 2m -count=1 ./...` PASS; `golangci-lint run` (v1.63.4) if available locally, else rely on CI `lint`.
- [ ] 7.2 Default (non `-short`) SimpleRedis run for chaos and close-cycle stress: `go test -count=1 ./simpleredis/` PASS (goroutine count flat, at-rest semaphore full, `OverFrees() == 0`).
- [ ] 7.3 Before/after performance (D8), median of 5 × 2 s on this machine: `go test ./simpleredis/ -run XXX -bench 'BenchmarkGet$|BenchmarkGetParallel$|BenchmarkYaegiGet$' -benchmem -count=5 -benchtime=2s`. Accept: `BenchmarkGet` / `BenchmarkGetParallel` medians within ±5 % of the explore baseline (19 338 / 16 611 ns/op) and `Get` 26 allocs/op unchanged; `BenchmarkYaegiGet` median inside 92 523–142 421 ns/op. Record before/after table in the run (and on the card); if the D3 fallback was measured, record both.
- [ ] 7.4 `openspec validate simpleredis-drop-select-under-yaegi` passes; `rg -n "select \{|select\{" simpleredis/*.go -g '!*_test.go'` returns nothing.

## Workflow follow-up

- `reclaim/table.go` `waitCtx` select and a repo-wide guard stay in `knowledge/debt/2026-11-21-reclaim-waitctx-select.md`.
- Archive syncs the two spec deltas into `openspec/specs/`.
