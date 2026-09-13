## Context

Dest `exec` binds `(maxRetries+1)*(DialTimeout+IOTimeout)` onto `ctx` and `defer cancel()` (`simpleredis/commands_exec.go`). `Eval` calls `exec` twice on NOSCRIPT (`commands_eval.go`). `msetex` `exec`s native MSETEX then `msetexEval` → `Eval` on unknown-command (`commands_msetex.go`). Spec `std_go_simpleredis_tcp-session` still says one overall deadline per public command. Human decision: keep per-hop budgets; comments only. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Comments on `Eval`, `MSetEX`, `MSetEXAt`, and `msetex` that each hop is its own `exec` with a full command budget on purpose; do not share remaining time (that can starve EVAL).
- Compiled tests that pass on dest documenting that intent (NOSCRIPT / unknown-command success). Do not assert elapsed against one public-command budget.

**Non-Goals:**
- Sharing remaining time across hops.
- Splitting `execBound` (does not exist) or binding once per public verb.
- Changing `exec` / `bindCommandDeadline`.
- Rewriting `std_go_simpleredis_tcp-session` “Whole command has an overall deadline”.
- Handshake redial; Eval `$-1` as `redis:miss`.
- Tests that fail because elapsed > one public-command budget (including the 85ms-then-stall characterization).

## Decisions

1. **Each hop is its own `exec`.** Alternative: bind once per public verb so Eval/MSetEX share remaining time — rejected; a slow EVALSHA can starve EVAL. Alternative: split `execBound` — rejected; `execBound` does not exist and the ticket forbids it.

2. **Comments, not a deadline change.** The four named functions get the intent in comments. `exec` stays the binder. Dest already stacks; this change does not alter elapsed.

3. **Tests that pass on dest.** Comment `TestEvalArgvAndIntegerReply` and `TestMSetEXUnknownCommandFallsBackAndCaches`. Add a small compiled test only if those comments are not enough for a new assertion. Do not port MaxRetries -1 / DialTimeout 20ms / IOTimeout 100ms / 85ms first-hop delay into CI. Immediate NOSCRIPT then stall still fits 120ms — delay on the first hop is required to see stacking; that is why per-hop budgets stay.

4. **Fold `std_go_simpleredis_resp-commands`.** FindSpecHost: small adjustment to Eval / MSetEX (one added requirement). Do not fold `std_go_simpleredis_tcp-session` (requirement Out of scope; explore assumed).

5. **Usage packet later.** `knowledge/devdocs/std_go_simpleredis.md` still states one overall wait per verb. Implement does not rewrite it (`explore.md` assumed; `opd-devdocsimpact`).

## Risks / Trade-offs

- [tcp-session still says one overall deadline per public command] → Accepted this run; comments next to the hops are the exception record. Usage packet may still understate Eval/MSetEX worst-case wait until devdocsimpact.
- [A later reader lands a failing elapsed>budget test] → The added spec forbids that; tasks say do not add it.

## Migration Plan

None. Comments and tests only. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
