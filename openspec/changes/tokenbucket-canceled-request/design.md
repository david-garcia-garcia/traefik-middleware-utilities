# Design

## Context

See `proposal.md` — Why. Explore recorded the units, the Dest `(Status, error)` signature, and that the 500 is not reproduced here. This design transcribes the tech-lead binding; it does not redesign.

Current state that constrains the approach:

- `Memory.Allow` (`tokenbucket/memory.go`) already returns `ctx.Err()` with `Status{}` before the mutex and `consumeOne`. A pre-canceled ctx charges nothing. GoDoc does not say so.
- `Redis.Allow` (`tokenbucket/redis.go`) already returns `ctx.Err()` before `Eval`. If the ctx is still live, it calls `Eval`; an in-flight cancel surfaces as `Canceled` with `Status{}` while bytes already on the wire may still run. Measured 2026-11-22 on the analysis worktree: Redis charged (`tokens=2` of burst 3); Dragonfly left an empty hash. GoDoc does not say a `Canceled` / `DeadlineExceeded` does not prove the script did not run.
- `Status` (`tokenbucket/status.go`) never writes HTTP and never sleeps. The 499 / skip-wait mapping is the middleware's job.
- Live catalog `openspec/specs/std_go_tokenbucket_allow/spec.md` scenario «Error returns the zero status» already covers cancelled context → zero `Status`. This change does not add a SHALL of in-flight charge, refund, or holding EVAL.
- Dest `Allow` returns `(Status, error)` (PR #106). The analysis-worktree repros still call the old `(bool, time.Duration, error)` shape. New tests use `status.Allowed`.
- `TestStatus_ErrorReturnsTheZeroStatus` already asserts `context.Canceled` and zero `Status` on Memory and fake Redis. It does not assert a full burst afterwards or in-flight cancel. It does not replace `TestTraefik14173_*`.
- Live gate: `runForEachLiveEngine` / `waitLiveClient` in `tokenbucket/limiter_e2e_test.go` (`TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`; skip under `-short` or both unset).
- Redis 7 freezes `TIME` inside scripts; a time-based busy Lua loop never ends. Do not use one.

Upstream (existence of the defect): https://github.com/traefik/traefik/issues/14173 https://github.com/traefik/traefik/issues/13879 https://github.com/traefik/traefik/pull/13880 (open) https://github.com/traefik/traefik/pull/14236 (closed as duplicate).

## Goals / Non-Goals

**Goals:**

- Document existing `Allow` cancel behavior in GoDoc (Memory: already-done ctx charges nothing; Redis: `Canceled` / `DeadlineExceeded` does not prove the script did not run).
- Point middleware authors at 499 / no-wait when `wait==0`, citing traefik/traefik#14173.
- Own tests for this issue: Memory pre-canceled (full burst afterwards); live in-flight cancel through a holding TCP proxy (prompt `Canceled`, log charge).

**Non-Goals:**

- Changing `Allow`, refunding an in-flight Redis consume, or skipping EVAL once the ctx is canceled.
- Writing 499 (or any HTTP) from this library.
- Rewriting Traefik's `select` / timer.
- A spec delta, a usage-packet edit, shared helpers, copying other issues' tests, or a Lua `TIME` busy loop.
- Changing `simpleredis` `Eval` / `exec`.

## Decisions

### D1. No production behavior change

GoDoc and tests only. `ctx.Err()` before consume, `Eval` as today, `Status{}` on error, no refund of a script that already ran. Rejected: returning a different error, holding EVAL, or compensating the charged token — Out of scope.

### D2. GoDoc on both `Allow` methods

`Memory.Allow`: a ctx already done returns its error and charges nothing.

`Redis.Allow`: a `context.Canceled` / `DeadlineExceeded` return does not prove the script did not run; the token may have been charged.

Both comments mention that HTTP mapping (499 on `Canceled`, no wait when `wait==0`) belongs to the middleware, citing [traefik/traefik#14173](https://github.com/traefik/traefik/issues/14173). `godot` requires the comments to be sentences that end with a period.

Rejected: putting the HTTP mapping in `status.go` only — Desired names the `Allow` GoDocs; `Status` already says the library never writes HTTP.

### D3. Two self-contained test files, helpers suffixed `14173`

- `tokenbucket/traefik_issue14173_test.go` — `TestTraefik14173_MemoryCanceledContext` plus `countAdmits14173`.
- `tokenbucket/traefik_issue14173_e2e_test.go` — `TestTraefik14173_RedisCanceledInFlight` plus `countingProxy14173`, `startCountingProxy14173`, `forwardHeld14173`, `liveKey14173`, `evalKey14173`.

Reuse `runForEachLiveEngine` and `waitLiveClient` from `limiter_e2e_test.go` (same package). Do not add a shared helper file. Do not copy `traefik_issues_test.go` / `traefik_issues_e2e_test.go` from the analysis worktree (those files mix other issues and the old `Allow` signature). Read those two tests as the behavioral reference, then write Dest-shaped originals.

Reference (read-only): `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros` `tokenbucket/traefik_issues_test.go` (`TestTraefik14173_MemoryCanceledContext`) and `tokenbucket/traefik_issues_e2e_test.go` (`TestTraefik14173_RedisCanceledInFlight`, `countingProxy` / `forwardHeld`). The ticket named `traefik_issue14173_*.go` as the destination in this tree; the analysis tree still keeps them inside the combined files.

### D4. Asserts use `(Status, error)`

`errors.Is(err, context.Canceled)`, `!status.Allowed`, and on a non-nil error the `Status` is the zero value. Burst-afterwards: subsequent `Allow(context.Background(), key)` with `status.Allowed` until deny; `countAdmits14173` counts those admits. Do not reintroduce `(bool, time.Duration, error)`.

Memory case: clock frozen, burst 3, rate `1.0/60`, `maxDelay` 0, TTL 120s; pre-cancel; then `countAdmits14173` of `burst+5` must equal `burst`.

Live case: burst 3, hold 1500ms, cancel after 200ms; assert elapsed `< hold/2`; log `hmget last, tokens` after `hold+300ms`. Do not `Fatal` because Redis charged or Dragonfly left an empty hash.

### D5. Holding TCP proxy, not a Lua `TIME` loop

`startCountingProxy14173` listens on `127.0.0.1:0`, dials the live engine, and `forwardHeld14173` sleeps `holdWrites` before each client-to-server chunk so a write already read is still delivered after the client leaves. Warm the script and pooled socket with one `Allow` on a warmup key before enabling the hold, so the canceled call is a single EVALSHA write. Gate with `runForEachLiveEngine(t, "TOKENBUCKET_LIVE_REDIS", "TOKENBUCKET_LIVE_DRAGONFLY", ...)`.

Rejected: a Lua loop on `TIME` — Redis 7 freezes `TIME` inside scripts and the loop never ends.

### D6. No spec delta

Live catalog: no new promise. `skip_specs: true`. Bound the work here and in `tasks.md`. Journal `devstate/.../specs.md` as `none — no live contract`.

## Risks / Trade-offs

- [In-flight Redis still charges] → Documented in GoDoc (D2). Tests log it (D4). Not refunded (D1).
- [Dragonfly 1.40.2 may still skip the held EVAL] → Log the bucket; do not assert per engine (D4).
- [Helper name clash with parallel issue PRs] → Suffix `14173` in the two new files (D3).
- [Copying the analysis combined test file] → Would pull other issues and the old `Allow` signature. Write Dest-shaped originals (D3).
- [Lua `TIME` busy-wait] → Forbidden (D5).
