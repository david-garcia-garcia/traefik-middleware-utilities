## Context

Dest `Eval` always sends `EVAL` plus `[]byte(script)`. `replyError` maps AUTH prefixes; other `-` payloads (including NOSCRIPT after `readReply` strips `-`) are `errors.New(text)`. `exec` retries a dead idle socket once and must not special-case NOSCRIPT. Fake `startFakeRedis` understands EVAL; unknown commands including EVALSHA reply `+OK`. Probe `/redis` `/dragonfly` run one Eval per GET; Pester asserts `X-SimpleRedis-Eval` == `3`. Research: `knowledge/research/ext_redis_evalsha/`, `ext_dragonfly_evalsha/`. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Optimistic EVALSHA of a client SHA-1 cache; one EVAL fallback on NOSCRIPT.
- Fake argv proof (digest vs body) plus Yaegi fallback on that fake.
- Live SCRIPT FLUSH / SCRIPT EXISTS on Redis and Dragonfly via existing compose routes.

**Non-Goals:**
- Pipelining, MSETEX, `unsafe` zero-copy, eager `SCRIPT LOAD`, new exports.
- `go-redis`, miniredis, TLS, Unix sockets.
- Pool/timeout/retry policy except Eval matching NOSCRIPT.
- Rewriting token-bucket or window-counter Lua.
- New compose services or routes. Renaming `reclaim-e2e`.

## Decisions

1. **Keep `Eval(script, keys, args)`.** Optimistic `EVALSHA` of `crypto/sha1` + lowercase hex (Yaegi `stdlib.Symbols` includes `crypto/sha1` and `encoding/hex`). On `strings.HasPrefix(err.Error(), "NOSCRIPT")` fall back once to `EVAL` with the same keys/args; keep the digest. Do not export `EvalSha` / `ScriptLoad`. Do not `SCRIPT LOAD` at `Init`. Alternative: SCRIPT LOAD at Init — rejected; cache is volatile and Init must not dial.

2. **Eval inspects NOSCRIPT; `replyError` stays AUTH-only.** `exec` does not special-case NOSCRIPT (pool/retry out of scope). Alternative: map NOSCRIPT in `replyError` — rejected; that would leak a dedicated error to every verb.

3. **Digest map has its own mutex.** Do not hold the idle-pool `mu` across `exec`. Alternative: reuse pool `mu` — rejected; would serialize all commands behind SHA lookup.

4. **Fake: handle EVALSHA.** First miss `-NOSCRIPT No matching script. Please use EVAL.`; EVAL loads that digest (SHA-1 of the body); later EVALSHA succeeds. Record last argv so tests see digest not body. Two scripts → two digests. Unknown-command `+OK` MUST NOT remain the EVALSHA path. Encoder-only `startStaticRedis` tests still work: Eval sends EVALSHA first and consumes the canned reply. Alternative: miniredis — out of scope.

5. **Yaegi: extend `clientprobe` so interpreted Eval hits the NOSCRIPT fallback** against the compiled fake (`stdlib.Symbols`, `useunsafe` false, no Traefik). Keep existing Incr+Eval case.

6. **Live proof without new compose services.** Probe keeps the Kong KEYS snippet. Add `X-SimpleRedis-EvalDigest` (SHA-1 of that const) and a second Eval header (`X-SimpleRedis-EvalAgain` == `3`). Pester: `docker compose exec -T redis redis-cli SCRIPT FLUSH` then `SCRIPT EXISTS` of that digest (`0`), GET `/redis` succeeds, `EXISTS` `1`, GET again. Same against Dragonfly via `redis-cli -h dragonfly`. Fake tests own EVALSHA-vs-EVAL argv. Live tests own engine NOSCRIPT + SHA agreement.

7. **Do not rewrite `std_go_tokenbucket_lua-eval`.** "v1 MUST NOT use EVALSHA" is a caller-surface rule. Wire EVALSHA is SimpleRedis's job.

## Risks / Trade-offs

- [SCRIPT FLUSH races other compose clients] → Mitigation: Pester owns the sequence per engine; probe keys are per-request; FLUSH only drops scripts, not keys.
- [Dragonfly Lua 5.4 vs Redis 5.1] → Mitigation: Kong snippet lists KEYS and has no `table.maxn`; tokenbucket/windowcounter already match.
- [Client digest disagrees with engine] → Mitigation: live `SCRIPT EXISTS` of the Go hex after GET; v1.40.2 treated as Redis SHA-1 (`ext_dragonfly_evalsha`).
- [NOSCRIPT from a truncated SHA] → Mitigation: always send 40-char lowercase hex from `sha1` of the body.

## Migration Plan

Library-internal wire change. Public `Eval` unchanged. Rollback is revert. Callers keep passing the script body.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
