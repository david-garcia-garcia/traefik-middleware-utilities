## 1. Eval API

- [x] 1.1 Export `ScriptSHA1Hex`; delete unexported `scriptSHA1Hex`. SHA-1 of script bytes, lowercase 40-char hex. No digest table or mutex.
- [x] 1.2 Change `Eval` to `Eval(script, digest, keys, args)`. Send `EVALSHA` with the caller digest. On `NOSCRIPT` prefix send `EVAL` of the body once. Do not hash. Do not verify digest vs body. Do not export `EvalSha` or `ScriptLoad`.
- [x] 1.3 Package-level digest next to `msetexFallbackScript`; `msetexEval` passes it. Hash `flushScript` and `allowScript` at package init (windowcounter, tokenbucket). Probe: `ScriptSHA1Hex` for Eval calls and `X-SimpleRedis-EvalDigest`; drop `kongScriptDigest`.

## 2. Call sites and tests

- [x] 2.1 Update every remaining `SimpleRedis.Eval` call (simpleredis tests, Yaegi `clientprobe`, windowcounter/tokenbucket tests, e2e probe). One-shot tests may hash at the call. Fake still keys loaded scripts by `ScriptSHA1Hex(body)`.
- [x] 2.2 Compiled: first Eval falls back EVAL after NOSCRIPT; second sends EVALSHA with the caller digest not the body; two scripts two digests; empty keys `numkeys` `0`; `ScriptSHA1Hex` hex length. Existing Get/Set/Del/MGet/Incr still pass.
- [x] 2.3 Yaegi: `clientprobe` calls `ScriptSHA1Hex` then Eval (`stdlib.Symbols`, `useunsafe` false, no Traefik). Keep Incr+Eval and NOSCRIPT fallback.
- [x] 2.4 Run `go test -short ./simpleredis/ ./windowcounter/ ./tokenbucket/` until compiled and Yaegi tests pass.

## 3. Specs

- [x] 3.1 Confirm the change delta `std_go_simpleredis_resp-commands` matches the landed API.
- [x] 3.2 Run `openspec validate simpleredis-eval-caller-digest --type change --strict`
