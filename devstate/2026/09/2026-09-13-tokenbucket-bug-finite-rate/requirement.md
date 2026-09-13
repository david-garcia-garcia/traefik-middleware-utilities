# Requirement
IssueKey: 2026-09-13-tokenbucket-bug-finite-rate

## Problem
`validateClock` only rejects `rate <= 0`. `math.NaN() <= 0` is false and `+Inf > 0` is true, so `NewMemory` and `NewRedis` accept NaN and Inf. A NaN rate makes every `Allow` true. A +Inf rate does `Inf*0 = NaN` on the second `Allow` at elapsed=0 and fail-opens with `maxDelay=0`. This package has no unlimited-rate constructor; passthrough is skipping `New`.

## Current (code)
- `tokenbucket/clock.go` `validateClock` — `if rate <= 0 { return errRate }`. No `math.IsNaN` / `math.IsInf`. `clock.go` does not import `math`.
- `tokenbucket/clock.go` `errRate` — `"tokenbucket: rate must be greater than 0"`.
- `tokenbucket/memory.go` `NewMemory` — calls `validateClock` then stores `rate` on `clockConfig`.
- `tokenbucket/redis.go` `NewRedis` — same `validateClock` after the nil-redis check.
- `tokenbucket/clock.go` `limitPerMicro` — `rate / 1e6`. NaN stays NaN; +Inf stays +Inf.
- `tokenbucket/clock.go` `consumeOne` — `tokens = tokens + limitPerMicro*elapsed`. At elapsed=0, Inf*0 is NaN; `tokens < 0` is false for NaN, wait stays 0, `allowedFromWait(0, 0)` is true.
- `tokenbucket/limiter_test.go` `TestNewMemory_RejectsInvalidClock` — covers rate `0`, not NaN or Inf.
- Dest `tokenbucket/` has no `repro_nan_rate_test.go` or `repro_hunt_inf_rate_nan_test.go` (`not found` on dest). Those files exist only on the caller’s dirty tree.
- `openspec/specs/std_go_tokenbucket_allow/spec.md` — construction SHALL fail when `rate <= 0`. Scenario names rate 0 or a negative rate. Does not name NaN or Inf.
- `knowledge/devdocs/std_go_tokenbucket.md` — `rate <= 0` fails New; passthrough is skipping construction. No finite-rate rule.

## Desired
1. Land product tests in `tokenbucket/` that **fail on current dest** (`go test ./tokenbucket`). Tests first, then the fix. Do not weaken them to pass.
2. Copy/adapt `TestRepro_NaNRateRejected` (`NewMemory` / `NewRedis` NaN → `errRate`, nil limiter) and `TestRepro_NaNRateAllowFailOpen` (NaN construction then Allow fail-open on dest). Dest `Allow` is `(bool, time.Duration, error)`; tests must use that signature.
3. Do **not** keep `TestRepro_NaNRateInfAllow` as “Inf should always allow”. Inf is rejected at `New`.
4. Copy/adapt `TestRepro_InfRateElapsedZeroNaNFailOpen`: dest today accepts +Inf and fail-opens on the second Allow; after the fix, `New` must error (`errRate`) for +Inf (and `-Inf` via `math.IsInf(rate, 0)`).
5. Agreed how: `validateClock` rejects any non-finite rate as `errRate`: `rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0)`. Both constructors already go through this. Do not special-case `Allow` or `consumeOne` for NaN tokens. Do not treat +Inf as unlimited.
6. Then those tests PASS and existing `tokenbucket` tests stay green.

## Affected
- `tokenbucket/clock.go` (`validateClock`)
- `tokenbucket/` tests (new failing-then-green coverage; named repro files or `limiter_test.go`)
- `openspec/specs/std_go_tokenbucket_allow/spec.md` (construction fail for non-finite rate, not only `<= 0`)
- `knowledge/devdocs/std_go_tokenbucket.md` (gotcha currently `rate <= 0`)

## Out of scope
- Allow wait mapping (`allowedFromWait` Duration vs microseconds)
- Eval wait NaN/Inf (`errEvalWait` after ParseFloat)
- TTL seconds truncation
- Lua fill / new-key burst (`last=0` approximation)
- last rewind / stale now
- Cap eviction fresh burst
- Special-casing `Allow` or `consumeOne` for NaN tokens
- Treating +Inf as unlimited / adding an unlimited-rate constructor
- Other packages (`simpleredis`, `reclaim`, `windowcounter`)

## Unknowns
- Whether new tests land as the named `repro_*.go` files or are folded into `limiter_test.go`.
- Whether `-Inf` gets its own constructor test or is covered only by `math.IsInf(rate, 0)` plus +Inf.
- Whether `TestRepro_NaNRateAllowFailOpen` stays as a skip-after-fix witness or is rewritten to only assert New reject (ticket says copy/adapt; after the fix New rejects so Allow fail-open is not reachable).

## Tensions
- Ticket: reject NaN and Inf at `New` as `errRate`. Spec `std_go_tokenbucket_allow` and usage gotcha only name `rate <= 0` (0 or negative). Ticket wins; spec/usage must catch up in propose.
- Caller dirty-tree `TestRepro_NaNRateInfAllow` asserts Inf always allows. Ticket: do not keep that; Inf is rejected at New.
- Caller dirty-tree repro files still call 2-value `Allow`; dest `Allow` returns three values. Tests copied onto dest must match dest’s signature.
- Ticket: do not special-case `Allow`/`consumeOne`. Dest fail-open is only reachable because `validateClock` lets non-finite rate through; the fix is the constructor gate.
