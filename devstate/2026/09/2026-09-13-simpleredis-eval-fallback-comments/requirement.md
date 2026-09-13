# Requirement
IssueKey: 2026-09-13-simpleredis-eval-fallback-comments

## Problem
Eval and MSetEX each bind a new overall deadline on fallback hops. Spec `std_go_simpleredis_tcp-session` “Whole command has an overall deadline” says one budget per public command. Measured stacking (MaxRetries -1, DialTimeout 20ms, IOTimeout 100ms → 120ms one-command budget; first-hop delay 85ms then stall): Eval EVALSHA → NOSCRIPT then stall EVAL ~186ms; MSetEX unknown-command then stall Eval ~186ms. Cause: each `exec` calls `bindCommandDeadline` and `defer cancel()`; the first hop’s cancel drops the child ctx; the next hop binds a fresh `(maxRetries+1)*(Dial+IO)`. Human decision: not a behavior change; comments only.

## Current (code)
- `simpleredis/commands_exec.go:16-25` — `exec` binds one library overall deadline onto `ctx` (`bindCommandDeadline`) then `defer cancel()`.
- `simpleredis/commands_exec.go:67-76` — `bindCommandDeadline` uses `(maxRetries+1)*(DialTimeout+IOTimeout)` when that instant is sooner than the parent; `defer cancel()` in `exec` ends that child when the hop returns.
- `simpleredis/commands_eval.go:26-32` — `Eval` calls `exec` for EVALSHA; on `NOSCRIPT` prefix calls `exec` again for EVAL. Two hops, two binds. No comment that the second bind is intentional.
- `simpleredis/commands_msetex.go:33-39` — `MSetEX` / `MSetEXAt` delegate to `msetex` with no per-hop deadline comment.
- `simpleredis/commands_msetex.go:42-58` — `msetex` `exec`s native MSETEX; on unknown-command calls `msetexEval` → `Eval` (another one or two `exec`s).
- `simpleredis/commands_msetex.go:61-68` — `msetexEval` is the Lua fallback via `Eval`.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md:346-347` — “Each command SHALL compute one overall deadline at entry equal to `(maxRetries+1)*(DialTimeout+IOTimeout)`…”.
- `execBound` — not found.
- Tests that fail (or assert) because fallback elapsed exceeds one public-command budget — not found. Dest already has NOSCRIPT fallback success: `simpleredis/commands_eval_test.go:8-26` `TestEvalArgvAndIntegerReply`; `simpleredis/yaegi_test.go:42`.

## Desired
Not a behavior change. Implement order (human override):
1. FIRST add compiled tests that document the intended per-hop budget. They SHALL pass on current dest code, or be comments-only plus a small test that Eval NOSCRIPT fallback still succeeds. Do not add tests that require a single stacked deadline, or that fail because elapsed > one public-command budget.
2. THEN comments on `Eval`, `MSetEX`, `MSetEXAt`, and `msetex`. Encode: each hop is its own `exec` and binds its own overall deadline on purpose; the fallback must still have a full command budget after EVALSHA / unknown-command; do not share remaining time across hops (that can starve EVAL). Do not split `execBound`. Do not bind once per public verb.
3. THEN confirm `go test -short ./simpleredis/` still passes.

Characterization that proved stacking (do not turn into failing CI tests): MaxRetries -1, DialTimeout 20ms, IOTimeout 100ms → 120ms one-command budget. Fake delay matching verb 85ms then stall later verbs. Immediate NOSCRIPT then stall still fits in 120ms — delay on the first hop is required to see stacking; that is why per-hop budgets stay.

## Affected
- `simpleredis/commands_eval.go` (`Eval`).
- `simpleredis/commands_msetex.go` (`MSetEX`, `MSetEXAt`, `msetex`).
- New or extended `simpleredis/*_test.go` that documents per-hop budget without asserting one stacked public-command deadline.

## Out of scope
- Handshake redial.
- Eval `$-1` as `redis:miss`.
- Changing `exec` / `bindCommandDeadline`.
- Splitting `execBound` (does not exist) or binding once per public verb.
- Sharing remaining time across hops.
- Tests that fail because elapsed > one public-command budget.
- Rewriting `std_go_simpleredis_tcp-session` “Whole command has an overall deadline” (ticket did not ask; listed as tension).

## Unknowns
- Exact compiled-test shape that documents per-hop budget while still passing on dest (comments-only vs success-path without elapsed>budget).
- Whether a later propose updates the live tcp-session spec to admit per-hop budgets. This ticket is comments-only.

## Tensions
- Live spec `std_go_simpleredis_tcp-session` “Whole command has an overall deadline” (one budget per public command) vs dest `Eval` / `msetex` stacking a fresh `exec` deadline per hop vs human decision: keep stacking, comments only, do not share remaining time.
- Characterization measured ~186ms vs 120ms stacking, but landing a CI test that fails on that stacking would re-litigate the decision.
