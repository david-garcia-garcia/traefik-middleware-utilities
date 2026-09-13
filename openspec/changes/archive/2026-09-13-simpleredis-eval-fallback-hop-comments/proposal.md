## Why

Eval and MSetEX each start a fresh overall deadline on fallback hops (`exec` bind + `defer cancel()`). Dest already does this so EVAL still has a full command budget after EVALSHA / unknown-command. Sharing remaining time can starve EVAL. The next reader can treat that stacking as a bug unless comments (and tests that pass on dest) record it.

## What Changes

- Comments on `Eval`, `MSetEX`, `MSetEXAt`, and `msetex`: each hop is its own `exec` and binds its own overall deadline on purpose; do not share remaining time across hops.
- Compiled tests that document the intended per-hop budget and pass on current dest (NOSCRIPT / unknown-command success). Do not add tests that fail because elapsed exceeds one public-command budget.
- **Not this change:** sharing remaining time; splitting `execBound`; binding once per public verb; rewriting `std_go_simpleredis_tcp-session` “Whole command has an overall deadline”; handshake redial; Eval `$-1` as `redis:miss`; changing `exec` / `bindCommandDeadline`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: Eval NOSCRIPT and MSetEX unknown-command fallback hops each keep a full command budget (comments + tests that pass on dest). Not a wire-behavior change.

## Impact

- `simpleredis/commands_eval.go` (`Eval`).
- `simpleredis/commands_msetex.go` (`MSetEX`, `MSetEXAt`, `msetex`).
- `simpleredis/commands_eval_test.go`, `simpleredis/commands_msetex_test.go`.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- Yaegi stdlib only. No go-redis, miniredis. No change to `exec` / `bindCommandDeadline`.
