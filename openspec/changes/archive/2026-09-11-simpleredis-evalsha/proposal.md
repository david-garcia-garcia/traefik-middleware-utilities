## Why

`SimpleRedis.Eval` ships the full Lua body on every call. Token-bucket Allow and window-counter flush pay that wire cost on every decision even though Redis and Dragonfly already cache the script by SHA-1.

## What Changes

- Keep the public `Eval(script, keys, args)` signature. Callers still pass the script body. No `EvalSha`, no `ScriptLoad`, no `SCRIPT LOAD` at `Init`.
- Cache SHA-1 of each distinct script on the client. Send `EVALSHA` first. On an error whose text starts with `NOSCRIPT`, fall back once to `EVAL` with the same keys and args, then keep using the digest. Do not return that `NOSCRIPT` to the caller.
- Fake Redis understands `EVALSHA` and the NOSCRIPT miss so compiled tests see digest vs body. Yaegi interp proves the fallback against that fake (`stdlib.Symbols`, `useunsafe` false, no Traefik).
- Live proof on both engines: extend `e2e/simpleredisprobe` plus Pester `/redis` and `/dragonfly` so EVALSHA + NOSCRIPT fallback is proven against Redis 7 (Lua 5.1) and Dragonfly v1.40.2 (Lua 5.4). Scripts list keys in KEYS and MUST NOT use `table.maxn`. No new compose services or routes.
- Update `std_go_simpleredis_resp-commands` so EVALSHA-inside-Eval is required, not forbidden. Do not rewrite `std_go_tokenbucket_lua-eval` (caller-surface: tokenbucket still calls `Eval`).

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: Eval SHALL send EVALSHA of a client SHA-1 digest with a one-shot NOSCRIPT→EVAL fallback; public signature unchanged. Yaegi MUST prove the fallback. Traefik e2e on `/redis` and `/dragonfly` MUST prove the miss then hit on both engines.

## Impact

- `simpleredis/simpleredis.go` (Eval; digest map + mutex; no pool/`exec`/`replyError` policy change except Eval inspecting NOSCRIPT text).
- `simpleredis/simpleredis_test.go`, `simpleredis/yaegi_test.go`.
- `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.Tests.ps1`.
- Callers `tokenbucket/` and `windowcounter/` stay on `Eval` (no signature change; Lua already lists KEYS and avoids `table.maxn`).
- Main spec `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` stays dest Eval until implement/devdocsimpact (callers still pass the body).
