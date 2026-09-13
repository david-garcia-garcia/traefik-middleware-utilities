# Explore
IssueKey: 2026-09-13-simpleredis-eval-fallback-comments

## Concepts

```
Eval(ctx)
  ├─ exec(EVALSHA) ── bindCommandDeadline + defer cancel()
  │     └─ NOSCRIPT  (child ctx cancelled on return)
  └─ exec(EVAL)     ── bindCommandDeadline again (full (maxRetries+1)*(Dial+IO))

MSetEX / MSetEXAt
  └─ msetex
        ├─ exec(MSETEX) ── own bind
        └─ unknown-command → msetexEval → Eval  (one or two more exec binds)
```

- **Hop** = one `exec` send (EVALSHA, EVAL, native MSETEX). Each hop binds its own library overall deadline. That is dest today.
- **Public command** = `Eval`, `MSetEX`, `MSetEXAt`. Spec `std_go_simpleredis_tcp-session` “Whole command has an overall deadline” names one budget at that entry. Dest does not: `exec` is the binder, and fallback is a second `exec`.
- **Starve EVAL** = if the first hop used most of a shared remaining budget (slow EVALSHA / unknown MSETEX), the EVAL body send can hit `redis:timeout` before the engine stores the script. Human: keep a full command budget on the fallback hop.
- Characterization that proved stacking (not a CI test): MaxRetries -1, DialTimeout 20ms, IOTimeout 100ms → 120ms one-command budget. First-hop delay 85ms then stall: Eval ~186ms, MSetEX→Eval ~186ms. Immediate NOSCRIPT then stall still fits 120ms — delay on the first hop is required to see stacking; that is why per-hop budgets stay.

Usage packet `knowledge/devdocs/std_go_simpleredis.md` still says worst-case wait is one `(MaxRetries+1)*(DialTimeout+IOTimeout)` per verb. Eval NOSCRIPT and MSetEX unknown-command can take two or three hops. Research `ext_redis_evalsha` already owns NOSCRIPT wire text; no new research.

## Decisions

- Not a behavior change. Do not share remaining time across hops. Do not split `execBound` (does not exist). Do not bind once per public verb. Do not change `exec` / `bindCommandDeadline`.
- Comments only on `Eval`, `MSetEX`, `MSetEXAt`, and `msetex`. Encode: each hop is its own `exec` and binds its own overall deadline on purpose; the fallback must still have a full command budget after EVALSHA / unknown-command; do not share remaining time (that can starve EVAL).
- Implement order: compiled tests that pass on dest (document per-hop intent) → then those comments → `go test -short ./simpleredis/`.
- Do not land tests that fail because elapsed > one public-command budget. Do not port the 85ms-then-stall characterization into CI.
- Do not rewrite live `std_go_simpleredis_tcp-session` this run (requirement Out of scope). The comments are the exception record next to the hops.

## Open questions

- Q: What compiled-test shape documents the intended per-hop budget while still passing on dest?
  Rank: additive asked — Desired step 1 names compiled tests that pass on current dest or comments-only plus Eval NOSCRIPT success
  Decision: resolved — commented TestEvalArgvAndIntegerReply and TestMSetEXUnknownCommandFallsBackAndCaches; added TestEvalNoscriptFallbackIsOwnExec (EVALSHA then EVAL both run, Eval succeeds). No elapsed assert.
  By: implement

- Q: Does this change rewrite `std_go_simpleredis_tcp-session` “Whole command has an overall deadline” to admit per-hop budgets?
  Rank: additive asked — Desired is comments-only; Out of scope lists rewriting that leaf
  Decision: assumed — do not rewrite the live tcp-session leaf. Comments on the four functions encode the per-hop exception. Spec vs dest tension stays.
  By: explore

- Q: Does implement also update `knowledge/devdocs/std_go_simpleredis.md` (worst-case wait understated for Eval / MSetEX fallbacks)?
  Rank: additive incidental — no Desired line names the usage packet; usage currently states one overall deadline per public verb
  Decision: resolved — updated `knowledge/devdocs/std_go_simpleredis.md` How-to and Gotcha: each hop binds a full command budget; Eval NOSCRIPT and MSetEX unknown-command are separate hops.
  By: devdocsimpact
