# Explore
IssueKey: 2026-09-11-simpleredis-perf-05-evalsha
Verdict: in progress

## Concepts

- **Eval** — exported `SimpleRedis.Eval(script, keys, args)` (`simpleredis/simpleredis.go`). Dest always sends `EVAL` plus the full body. Public signature stays. Callers pass the Lua source; they do not pass a digest.
- **exec / replyError** — the only wire path. `replyError` maps AUTH prefixes; any other `-` payload (including `NOSCRIPT`) is `errors.New(text)` with the `-` already stripped (`readReply` `line[1:]`). `exec` retries a dead idle socket once; it must not special-case `NOSCRIPT` (pool/retry out of scope).
- **Digest cache** — new client map, SHA-1 of each distinct script (`crypto/sha1` + lowercase hex, Redis `sha1hex`). Mutex-guarded, not the pool `mu`. No `SCRIPT LOAD` at `Init`. Yaegi v0.16.1 `stdlib.Symbols` includes `crypto/sha1` and `encoding/hex`.
- **EVALSHA miss** — Redis 7 and Dragonfly v1.40.2 both reply with a payload that **starts with `NOSCRIPT`**. Redis implementation: `NOSCRIPT No matching script. Please use EVAL.` Dragonfly `kScriptNotFound`: `-NOSCRIPT No matching script. Please use EVAL.` `HasPrefix("NOSCRIPT")` matches both. Cache is volatile (`SCRIPT FLUSH`, restart). (`knowledge/research/ext_redis_evalsha/`, `knowledge/research/ext_dragonfly_evalsha/`)
- **Fake Redis** — `startFakeRedis` understands `EVAL`; unknown commands including `EVALSHA` reply `+OK`. Must gain an `EVALSHA` + `NOSCRIPT` path or later-call tests cannot see the digest.
- **Callers of Eval** (searched `*.go` for `.Eval(`): `tokenbucket/redis.go` (allow script), `windowcounter/limiter.go` (flush script), `e2e/simpleredisprobe/plugin.go` (Kong KEYS snippet), `tokenbucket/live_test.go` (`return 1`), plus `simpleredis/*_test.go`. All keep calling `Eval`. Scripts already list `KEYS` and avoid `table.maxn`.
- **Traefik e2e** — compose `redis:7-alpine` and `dragonfly:v1.40.2`; probe `/redis` `/dragonfly`; Pester asserts `X-SimpleRedis-Eval` == `3`. One Eval per GET today. `redis-cli` lives on the Redis image; Dragonfly hostname is reachable from that container.

```
Caller Eval(script, keys, args)
        │
        ▼
  SHA-1 cache (client)
        │
        ▼
  EVALSHA digest numkeys keys args
        │
        ├─ NOSCRIPT ──► EVAL body numkeys keys args  (once; do not return NOSCRIPT)
        └─ result ──► caller [][]byte
```

## Decisions

- Keep `Eval(script, keys, args)`. Optimistic `EVALSHA` of `crypto/sha1` hex; on `strings.HasPrefix(err.Error(), "NOSCRIPT")` fall back once to `EVAL` with the same keys/args; keep the digest. Do not export `EvalSha` / `ScriptLoad`. Do not `SCRIPT LOAD` at `Init`. Do not map `NOSCRIPT` in `replyError` (Eval inspects the text).
- Digest map has its own mutex. Do not hold the idle-pool `mu` across `exec`.
- Fake: handle `EVALSHA`; first miss `-NOSCRIPT No matching script. Please use EVAL.`; `EVAL` loads that digest; later `EVALSHA` succeeds. Record last argv so tests see digest not body. Two scripts → two digests. Unknown-command `+OK` must not remain the EVALSHA path.
- Yaegi: extend `clientprobe` so interpreted `Eval` hits the NOSCRIPT fallback against the compiled fake (`stdlib.Symbols`, `useunsafe` false, no Traefik). Keep existing Incr+Eval case.
- Spec host: existing `std_go_simpleredis_resp-commands`. Replace "Eval SHALL send EVAL" / "EVALSHA and SCRIPT LOAD MUST NOT be added" with EVALSHA-inside-Eval + NOSCRIPT fallback; public signature unchanged. Yaegi and Traefik-e2e requirements grow to prove the fallback on fake and on both live engines.
- Do not rewrite `std_go_tokenbucket_lua-eval`. "v1 MUST NOT use EVALSHA" stays a caller-surface rule (`tokenbucket` still calls `Eval`). Wire EVALSHA is SimpleRedis's job.
- Live proof without new compose services or routes. Probe keeps the Kong KEYS snippet (Lua 5.1-safe, no `table.maxn`). Add `X-SimpleRedis-EvalDigest` (probe SHA-1 of that const) and a second `Eval` header (`X-SimpleRedis-EvalAgain` == `3`). Pester: `docker compose exec -T redis redis-cli SCRIPT FLUSH` then `SCRIPT EXISTS` of that digest (`0`), GET `/redis` succeeds (`Eval` `3`), `EXISTS` `1`, GET again `3`. Same against Dragonfly via `redis-cli -h dragonfly`. Existing verb headers stay. Reclaim `/a` `/b` stay up.
- Fake tests own the EVALSHA-vs-EVAL argv proof. Live tests own engine NOSCRIPT + SHA agreement (`SCRIPT EXISTS` of the Go digest). Together they satisfy the conductor HARD REQUIREMENT.
- Usage packet `std_go_simpleredis.md` still describes dest Eval. Do not document EVALSHA there until implement/devdocsimpact. No Language gap.
- Out of scope stays: pipelining, MSETEX, `unsafe`, eager `SCRIPT LOAD`, new exports, go-redis/miniredis, Lua rewrites.

## Open questions

- Q: Exact NOSCRIPT error prefix on Dragonfly v1.40.2 vs Redis 7?
  Rank: additive asked — Desired names HasPrefix NOSCRIPT; new match in Eval this change creates
  Decision: resolved — both payloads start with `NOSCRIPT`. Redis 7: `NOSCRIPT No matching script. Please use EVAL.` (docs example omits the trailer; implementation includes it). Dragonfly v1.40.2 `kScriptNotFound`: `-NOSCRIPT No matching script. Please use EVAL.` Match `strings.HasPrefix(err.Error(), "NOSCRIPT")`. Sources: `knowledge/research/ext_redis_evalsha/`, `knowledge/research/ext_dragonfly_evalsha/`.
  By: explore

- Q: How can probe/Pester prove EVALSHA vs a one-shot EVAL on a live engine?
  Rank: additive asked — Desired live Redis+Dragonfly proof; new headers/tests on existing `/redis` `/dragonfly`
  Decision: assumed — no new compose service. Probe sets `X-SimpleRedis-EvalDigest` and a second Eval header. Pester `SCRIPT FLUSH` + `SCRIPT EXISTS` via `redis:7-alpine` `redis-cli` (`-h redis` and `-h dragonfly`): miss (`0`) then GET succeeds then hit (`1`). Fake tests assert EVALSHA argv. Keep Yaegi fallback on the fake.
  By: explore

- Q: Is `std_go_tokenbucket_lua-eval` "v1 MUST NOT use EVALSHA" only a caller-surface rule, or must that spec be rewritten when Eval internally uses EVALSHA?
  Rank: additive asked — Unknowns names this spec question; no tokenbucket caller migrates (they already call `Eval`; searched `*.go` for `.Eval(`)
  Decision: resolved — caller-surface only. Do not rewrite `std_go_tokenbucket_lua-eval`. Propose updates SimpleRedis resp-commands so EVALSHA-inside-Eval is required.
  By: explore
