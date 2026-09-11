## Context

Dest `simpleredis/` already speaks GET/MGET/SET+EX/DEL/INCR/EXPIRE/EVAL over `exec`. `Eval` builds `EVAL` + `numkeys` + keys + args. `mu` guards pool/closed only. Fake unknown verbs reply `+OK` (including `MSETEX`). Compose and CI already run Redis 7 (`redis:7-alpine`) and Dragonfly v1.40.2. See proposal.md for why. Research: `knowledge/research/ext_valkey_msetex`, `ext_redis_msetex`, `ext_dragonfly_msetex`, `ext_dragonfly_eval`. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Two exported wrappers sharing one unexported group-write (validate, native-or-lua, cache).
- Fake native argv plus unknown-command fallback without a Lua VM.
- Compiled live file on both CI engines (TTL landed) plus Yaegi both paths and Pester headers.

**Non-Goals:**
- EVALSHA / SCRIPT LOAD / NOSCRIPT (sibling perf-05).
- Valkey or Redis 8 compose/CI service.
- Changing `MGet` empty-input, `Expire` integer-0 success, or `Eval`’s `[]string` args.
- Limiter flush callers.

## Decisions

1. **One unexported owner.** `MSetEX` / `MSetEXAt` validate then call one function with the EX/EXAT token. Native argv is `MSETEX`, decimal pair count, pairs, token, TTL. Success is `parseIntegerReply` then require `1`. Integer `0` is `errIssue` (Expire’s `:0` success does not apply). Alternative: two copied methods — rejected; symmetry.

2. **Capability cache on `SimpleRedis`.** One tri-state field (unknown / native / lua), read and written only while holding `mu`. Do not put it on `pooledConn`. Do not reset on `dial`. Concurrent first calls may both send native; both then cache. Alternative: `INFO server` — rejected (no extra probe). Alternative: reset after reconnect — rejected (Lua works on Valkey too; Desired is once per client).

3. **Unknown-command match.** After `replyError` (AUTH-class already `redis:noauth`), `strings.HasPrefix(err.Error(), "ERR unknown command")`. Do not match `Error running script`. Other `-` errors (WRONGTYPE, undeclared key) return as-is and do not recache Lua.

4. **Reuse `Eval`.** Fallback does not add EVALSHA. ARGV is each value as string (existing `Eval` surface; bytes stay opaque), then `EX` or `EXAT`, then the decimal TTL. Script:

```lua
local token = ARGV[#ARGV - 1]
local ttl = ARGV[#ARGV]
for i = 1, #KEYS do
  redis.call('SET', KEYS[i], ARGV[i], token, ttl)
end
return 1
```

No `unpack` / `table.unpack` / `table.maxn`. `#KEYS` is the pair count. Alternative: two scripts — rejected (one body to special-case in the fake).

5. **Fake Redis.** Add `case "MSETEX"`: record argv, store pairs, reply `:1`; on `EXAT` with a timestamp in the past, delete those keys and still reply `:1`. Test-only `rejectMSetEX` replies `-ERR unknown command 'MSETEX'` and does not store. Default `+OK` for other unknown verbs stays. EVAL special-cases the fallback script string (same pattern as Kong) and applies SET+store. `lastMSetEX` plus an MSETEX send count prove the second cached call skips native. Alternative: miniredis — out of scope.

6. **Pair cap 1024.** Constant next to `maxIdleConns`. Over cap, empty, and length mismatch → `errIssue` before `exec`. Zero/negative TTL pass through like `Set`.

7. **Live tests.** New `simpleredis/live_test.go` table-driven on `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY` (same skip/`waitLiveClient` shape as `windowcounter/live_test.go`). CI `test` job sets both to `127.0.0.1:6379` and `127.0.0.1:6380`. Native argv is fake-only.

8. **Probe and Pester.** `ServeHTTP` calls `MSetEX` and `MSetEXAt` on per-request keys, then `Eval` `return redis.call('TTL', KEYS[1])`. Headers `X-SimpleRedis-MSetEX` (`ok`) and `X-SimpleRedis-MSetEX-TTL` (positive decimal). Same on `/redis` and `/dragonfly`. Compose services and labels unchanged. Do not stop whoami-a/b.

9. **Yaegi.** Extend `clientprobeSrc`: native MSetEX against default fake; Lua path needs a reject-MSETEX fake. The compiled test starts the fake with `rejectMSetEX` before interp. GOPATH / stdlib / `useunsafe` false, no Traefik.

## Risks / Trade-offs

- [Redis 7 / Dragonfly have no MSETEX] → Mitigation: live tests prove Lua+TTL on both; detector still tries native first for 8.4+/Valkey.
- [Dragonfly undeclared keys / Lua 5.4] → Mitigation: names in KEYS; numeric `for`; no unpack/maxn.
- [Fake has no Lua VM] → Mitigation: special-case the fallback script string; live engines prove real Lua.
- [First-call race sends native twice] → Mitigation: accept; cache still ends native or lua; Lua is correct on every engine.
- [Past EXAT undocumented on MSETEX pages] → Mitigation: live Get-miss; do not hide Lua/native `1`.

## Migration Plan

Library additive API. Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
