## Context

Dest `Eval(script, keys, args)` hashes via unexported `scriptSHA1Hex` each call (`simpleredis/commands_eval.go`). Wire is already EVALSHA then EVAL on NOSCRIPT. Fake, Yaegi, live e2e, and Pester already prove that path. Research: `knowledge/research/ext_redis_evalsha/`. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Caller-supplied digest on Eval; exported `ScriptSHA1Hex`; reuse callers hash once next to the script const.
- Same NOSCRIPT → EVAL fallback; no digest-vs-body check.

**Non-Goals:**
- `EvalSha`, `ScriptLoad`, `SCRIPT LOAD`, client digest table/mutex.
- Verifying digest equals `ScriptSHA1Hex(script)`.
- Rewriting `std_go_tokenbucket_lua-eval`.
- Pipelining, pool/retry, Lua rewrites, new compose services.

## Decisions

1. **`Eval(script, digest, keys, args)`.** Script stays first; keys/args stay last. Digest is required. Alternative: digest-first like EVALSHA — rejected; would reorder every existing arg after a swap, not an insert. Alternative: hash inside Eval when digest is empty — rejected; the ask is caller control, not an optional fast path.

2. **Export `ScriptSHA1Hex`.** Delete unexported `scriptSHA1Hex`. Same SHA-1 lowercase hex. Alternative: keep an unexported alias — rejected; one owner.

3. **Eval does not verify digest vs body.** EVALSHA uses the caller string; NOSCRIPT still EVAL(body). Wrong digest repeats EVAL. Alternative: reject mismatch — out of scope on the requirement.

4. **Package-level digest next to each reused script const.** `flushScript`, `allowScript`, `msetexFallbackScript`, probe script consts. One-shot tests call `ScriptSHA1Hex(script)` at the Eval site. Probe `X-SimpleRedis-EvalDigest` uses `ScriptSHA1Hex`; drop `kongScriptDigest`. Alternative: field on every limiter — rejected; scripts are package consts.

5. **Do not rewrite `std_go_tokenbucket_lua-eval`.** Caller-surface: tokenbucket still calls `Eval`.

## Risks / Trade-offs

- [Wrong digest on every call] → Mitigation: usage gotcha; Eval does not fail closed; callers that reuse a const hash next to that const.
- [Yaegi cannot see the export] → Mitigation: `clientprobe` calls `ScriptSHA1Hex`; stdlib.Symbols already include `crypto/sha1` for dest hash; the export is this package.
- [Probe digest header drifts from Eval] → Mitigation: both use `ScriptSHA1Hex` of the same const.

## Migration Plan

Breaking Go signature. In-tree callers migrate in this change. External plugins recompile with the extra arg. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
