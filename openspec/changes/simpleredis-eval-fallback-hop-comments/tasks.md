## 1. Tests that pass on dest

- [ ] 1.1 Comment `TestEvalArgvAndIntegerReply` so it records that EVALSHA then EVAL are separate `exec` hops, each with a full command budget on purpose. Do not assert elapsed against one public-command budget
- [ ] 1.2 Comment `TestMSetEXUnknownCommandFallsBackAndCaches` so it records that native MSETEX then Eval are separate hops, each with a full command budget on purpose. Do not assert elapsed against one public-command budget
- [ ] 1.3 Add a small compiled test only if 1.1 is not enough that Eval NOSCRIPT fallback still succeeds on dest. Do not add first-hop delay + stall. Do not fail because elapsed > one public-command budget

## 2. Comments

- [ ] 2.1 Comment `Eval`: each hop is its own `exec` and binds its own overall deadline on purpose; the EVAL fallback must still have a full command budget after EVALSHA; do not share remaining time (that can starve EVAL)
- [ ] 2.2 Comment `MSetEX`, `MSetEXAt`, and `msetex`: native MSETEX is its own `exec`; unknown-command then Eval starts with a full command budget (one or two further hops); do not share remaining time

## 3. Prove

- [ ] 3.1 Run `go test -short ./simpleredis/` until it passes. Do not change `exec` / `bindCommandDeadline`
