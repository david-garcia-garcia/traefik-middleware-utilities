## Why

`SimpleRedis.Eval` SHA-1s the Lua body on every call. Window-counter flush, token-bucket Allow, and the MSETEX Lua fallback reuse one script for the life of the component, but they cannot hash once at init because the digest helper is unexported and Eval does not take a digest.

## What Changes

- **BREAKING.** `Eval` becomes `Eval(script, digest, keys, args)`. Callers pass Redis `sha1hex` (SHA-1 lowercase 40-char hex). Eval MUST NOT hash. EVALSHA still uses that digest; `NOSCRIPT` still falls back once to `EVAL` of the body. Do not verify digest vs body. No `EvalSha`, no `ScriptLoad`, no `SCRIPT LOAD` at `Init`, no client digest table.
- Export `ScriptSHA1Hex` (dest `scriptSHA1Hex`). Reuse callers (windowcounter, tokenbucket, MSETEX fallback, probe) compute and store that hex at component init and pass the stored digest. One-shot tests may hash at the Eval site.
- Probe `X-SimpleRedis-EvalDigest` uses `ScriptSHA1Hex` (drop the local SHA-1 copy). Yaegi `clientprobe` calls `ScriptSHA1Hex`.
- Update `std_go_simpleredis_resp-commands`. Do not rewrite `std_go_tokenbucket_lua-eval`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: Eval SHALL require a caller digest and SHALL NOT hash; `ScriptSHA1Hex` SHALL be the exported Redis `sha1hex`; wire EVALSHA then EVAL on NOSCRIPT stays.

## Impact

- `simpleredis/commands_eval.go` (signature + export). `simpleredis/commands_msetex.go` (fallback Eval).
- In-tree Eval callers: `windowcounter/limiter.go`, `tokenbucket/redis.go`, `e2e/simpleredisprobe/plugin.go`, simpleredis / windowcounter / tokenbucket tests including Yaegi `clientprobe`.
- Main spec `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` (implement/devdocsimpact).
