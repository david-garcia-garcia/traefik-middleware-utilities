# Eval and MSetEX fallback hops each bind a new overall deadline (comments only)

Eval and MSetEX each bind a new overall deadline on fallback hops. Spec tcp-session “Whole command has an overall deadline” says one budget per public command. Measured: Eval NOSCRIPT then stall EVAL ~186ms vs 120ms budget; MSetEX unknown-command then stall Eval ~186ms. Cause: Eval calls exec twice; each exec bindCommandDeadline + defer cancel(); first hop cancel drops the child ctx; second hop binds a new (maxRetries+1)*(Dial+IO). MSetEX unknown-command calls Eval (another one or two execs).

Agreed how (HUMAN DECISION — do not “fix” by sharing remaining time):
- NOT a behavior change.
- Fix with COMMENTS only on Eval, MSetEX, MSetEXAt, and msetex.
- Each hop is its own exec on purpose so the fallback still has a full command budget after EVALSHA / unknown-command. Sharing remaining time can starve EVAL.
- Do NOT split execBound. Do NOT bind once per public verb.
- Do NOT land tests that fail because elapsed > one public-command budget. That would re-litigate the decision.

Out of scope: handshake redial, Eval $-1 as redis:miss, changing exec/bindCommandDeadline.

Implement order (human override — record in requirement Desired)
1. FIRST add compiled tests that document the intended per-hop budget (they should PASS on current dest code, or be comments-only with a small test that Eval NOSCRIPT fallback still succeeds). Do not add tests that require a single stacked deadline.
2. THEN add the comments on Eval, MSetEX, MSetEXAt, msetex.
3. THEN confirm go test -short ./simpleredis/ still passes.

Characterization that proved stacking (DO NOT turn these into failing CI tests): MaxRetries -1, DialTimeout 20ms, IOTimeout 100ms → 120ms one-command budget. Fake delays matching verb 85ms then stalls later verbs. Eval EVALSHA → NOSCRIPT then stall EVAL elapsed ~186ms. MSetEX MSETEX → unknown-command then stall Eval ~186ms. Immediate NOSCRIPT then stall still fits in 120ms — delay on first hop is required to see stacking; that is WHY we keep per-hop budgets.

Comment intent to encode: each hop is its own exec and binds its own overall deadline on purpose; the fallback must still have a full command budget; do not share remaining time across hops.
