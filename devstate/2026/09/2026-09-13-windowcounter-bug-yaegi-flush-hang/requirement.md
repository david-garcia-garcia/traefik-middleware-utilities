# Requirement
IssueKey: 2026-09-13-windowcounter-bug-yaegi-flush-hang

## Problem
Interpreted live `windowcounter` buffered tests hang until the 5-minute package timeout. CI run 34766385613 job 103747990562 failed `TestYaegiLive_RedisAndDragonfly/dragonfly/bufferedTwoClients` (`FAIL windowcounter 300.008s`). The hang is inside Yaegi `interp.Eval` of `takeprobe.BufferedShare` (`windowcounter/limiter_yaegi_e2e_test.go:21` → `evalTakeprobe` at `limiter_yaegi_test.go:51`), not a network wait. It is intermittent. A Go test timeout kills the whole package binary, so one hung subtest masks every other `windowcounter` result. Surfaced on PR #69, which does not change `windowcounter`; treat as a pre-existing defect on `origin/master`. Do not touch #69.

## Current (code)
- `windowcounter/limiter.go` `Limiter` — `stopping`, `ticker *time.Ticker`, `stop chan struct{}`, `wg sync.WaitGroup`. `stopping` already on dest (`origin/master` `c230315`).
- `windowcounter/limiter.go` `New` — `syncRate > 0` calls `startFlushLocked` (`go l.flushLoop(l.ticker, l.stop)`).
- `windowcounter/limiter.go` `flushLoop` — interpreted `for { select { case <-ticker.C: flushPending; case <-stop: return } }` with `defer wg.Done()`.
- `windowcounter/limiter.go` `stopFlushAndWait` — sets `stopping`, `close(stop)` via `takeFlushTickerLocked`, unlocks, then `l.wg.Wait()`. Sleep and Close both call this after `flushPending`.
- `windowcounter/limiter.go` `Wake` — no-op when `closed || syncRate==0 || stop!=nil || stopping`.
- `windowcounter/limiter.go` — no Yaegi note. Contrast `simpleredis/resp.go` `watchConnClose`: Yaegi v0.16.1 `interp._select` races when interpreted code selects on a channel from a goroutine; use `context.AfterFunc` (real stdlib).
- `windowcounter/limiter_yaegi_test.go` `takeprobe.BufferedShare` — `New(..., time.Hour)` (buffered, flush goroutine exists), Takes, then `Sleep`/`Close`. `UntilDeny` uses `syncRate==0` (no flush goroutine).
- `windowcounter/limiter_yaegi_e2e_test.go:20-24` — live `bufferedTwoClients` evaluates `BufferedShare`. Exact `exactNThenDeny` passed in the failing job.
- Dest has no minimal Yaegi ticker+stop select probe. Precedent: `simpleredis/zz_scratch_deferpanic_test.go` (caller tree; not on dest).
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` — Sleep flushes then stops the ticker; Wake starts when not stopping; Close is idempotent. Spec still names “flush goroutine”.
- `knowledge/devdocs/std_go_windowcounter.md` — Sleep/Wake/Close and stopping intent. No Yaegi select constraint.
- `knowledge/research/ext_traefik_plugins_yaegi-afterfunc/notes.md` — Yaegi v0.16.1 exports `context.AfterFunc`; interpreted call runs. No `time.AfterFunc` probe on dest.
- Unmerged `2026-09-13-windowcounter-bug-sleep-wake-hang` (PR #61) is the compiled Sleep/Wake race (`stopping`). Dest already has that guard. That branch does not remove interpreted `go`+`select`.

## Desired
1. Confirm or refute the hypothesis with a real hang dump (`flushLoop` parked in Yaegi select AND `stopFlushAndWait` in `wg.Wait()`), plus a minimal interpreter probe with no `windowcounter` logic.
2. If confirmed: remove interpreted `go`+`select` from the flush path. Prefer self-re-arming `time.AfterFunc` (compiled stdlib timer, periodic flush preserved). Opportunistic Take/Peek flush is a behavior change (idle deltas sit unflushed) — do not take silently.
3. Preserve: Sleep flushes then stops; Wake restarts only when not closed, `syncRate > 0`, and not already running or stopping; Close is idempotent, flushes, refuses to restart; stopping-guard intent; no goroutine or timer leak; `lastFlushErr` / `flushFailedAt` / `lastRedisOK`.
4. Yaegi constraint note on `windowcounter/limiter.go` in the spirit of `simpleredis/resp.go`.
5. Sweep `tokenbucket`, `reclaim`, `backendbackoff`, other interpreted packages for the same `go`+channel-`select` shutdown pattern. Fix only trivially identical cheap hits; note the rest.
6. Supercede PR #61 for this deadlock (different root cause). Do not build on that branch. Do not touch PR #69.
7. Repro fails before the change and passes after. Interpreted live suites on both engines with `-count`; `go test -race`; full local suite; `go vet`; measured CI green.

## Affected
- `windowcounter/limiter.go` (flush start/stop; Yaegi note)
- `windowcounter/` tests (minimal Yaegi select probe; keep Sleep/Wake/Close tests)
- Sibling interpreted packages (sweep; cheap identical fixes only)
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` (timer vs goroutine wording if the flush owner changes)
- `knowledge/devdocs/std_go_windowcounter.md` (Yaegi constraint; Sleep/Wake/Close still true)

## Out of scope
- PR #69 (`simpleredis/resp.go` handshake)
- PR #61 Sleep/Wake compiled race (already on dest; do not merge that branch into this one)
- Changing buffered vs exact Take/Peek math
- Changing `lastFlushErr` fail-closed contract
- Traefik/Pester as the behaviour proof
- Opportunistic idle-only flush (decision required; default is AfterFunc)

## Unknowns
- Whether the hypothesis is true until a hang dump and a minimal probe exist (explore).
- Whether `time.AfterFunc` under Yaegi v0.16.1 maps to stdlib the same way `context.AfterFunc` does (research exists only for context).
- How many sibling packages still ship interpreted `go`+`select` shutdown.

## Tensions
- Caller: dest already has `stopping`. Unmerged PR #61 still shows that hunk against its merge-base. Supercede #61 for the Yaegi deadlock; leave #61’s remaining spec/test delta alone.
- Spec “flush goroutine” vs AfterFunc timer: same Sleep/Wake/Close contract; wording catches up in propose if the owner changes.
- Caller named `simpleredis/yaegi_defer_test.go`; dest has `simpleredis/zz_scratch_deferpanic_test.go` as the probe shape.
