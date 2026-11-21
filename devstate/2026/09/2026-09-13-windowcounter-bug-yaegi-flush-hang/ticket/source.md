# Intermittent five-minute hang in windowcounter interpreted live tests

Repo: traefik-middleware-utilities (Go 1.21, Traefik plugin library whose packages ALSO RUN INTERPRETED under Yaegi v0.16.1). Base on origin/master. PR host is GitHub.

## The observed failure

`TestYaegiLive_RedisAndDragonfly/dragonfly/bufferedTwoClients` in `windowcounter` hung until the 5-minute test timeout and killed the whole package binary (`FAIL ... windowcounter 300.008s`). Seen on CI run 34766385613, job 103747990562. It is INTERMITTENT: a re-run of the same commit passed.

Two important framing facts:
- It surfaced on PR #69's branch, but #69 changes ZERO lines in `windowcounter` (only 13 lines in `simpleredis/resp.go` plus tests). Treat this as a PRE-EXISTING defect on `master`, not a #69 regression. Do NOT touch #69's branch.
- Because a Go test timeout kills the entire package binary, this hang masks every other `windowcounter` result in that job. The blast radius is bigger than one subtest.

The hang is inside `interp.Eval` → `interp.runCfg`, i.e. blocked in INTERPRETED code, not in a network wait. Stack frames: `windowcounter.evalTakeprobe` at `windowcounter/limiter_yaegi_test.go:51` from `limiter_yaegi_e2e_test.go:21`, which evaluates `takeprobe.BufferedShare(addr, name)`.

## Leading hypothesis — confirm or refute it, do not assume it

This repo has ALREADY documented this exact interpreter defect, but only in a different package. `simpleredis/resp.go` says Yaegi v0.16.1's `interp._select` races when interpreted code selects on a channel from a goroutine. `simpleredis` removed `go` + `select` on `ctx.Done` in favor of `context.AfterFunc`. `windowcounter/limiter.go` on `master` STILL HAS interpreted `go l.flushLoop` plus `select` on ticker and stop, then `l.wg.Wait()` in `stopFlushAndWait`.

Hypothesis: the interpreted `select` in `flushLoop` fails to observe `close(l.stop)`, so `stopFlushAndWait`'s `l.wg.Wait()` never returns and `Sleep`/`Close` deadlock.

Supporting correlation: `exactNThenDeny` PASSED while `bufferedTwoClients` HUNG. The flush goroutine only exists when `syncRate > 0`. Dragonfly-only and intermittent both fit a timing race.

## Task A: deterministic repro and confirm root cause

Stress interpreted buffered subtests (`-count`) against live Dragonfly and Redis. Build a MINIMAL interpreter probe (precedent: `simpleredis/zz_scratch_deferpanic_test.go`) that starts a goroutine which selects on a ticker channel and a stop channel, then closes stop and waits on a WaitGroup. If the evidence REFUTES the hypothesis, stop and report.

## Task B: fix

Remove the interpreted `go` + `select` from the flush path, mirroring `simpleredis`. Evaluate self-re-arming `time.AfterFunc` vs opportunistic flush on Take/Peek. Preserve Sleep/Wake/Close semantics, the stopping guard intent, no goroutine or timer leak, and lastFlushErr / flushFailedAt / lastRedisOK. Add a Yaegi constraint note to `windowcounter/limiter.go`.

## Task C: unmerged branch

`2026-09-13-windowcounter-bug-sleep-wake-hang` is UNMERGED and rewrites `windowcounter/limiter.go`. Read it BEFORE starting. Decide whether to build on it, coordinate with it, or supersede it.

## Task D: sweep siblings

Search `tokenbucket`, `reclaim`, `backendbackoff`, and any other interpreted package for `go` plus `select` with channel cases, especially WaitGroup/ticker/stop shutdown. Report every hit. Fix only trivially identical cheap conversions; record the rest as follow-ups.

## Validation

Repro must FAIL before the change and PASS after. Interpreted live suites against both engines with `-count`, `go test -race`, full local suite, `go vet`, measured CI green on all eight checks.
