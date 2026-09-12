## Context

See proposal.md. Dest `ServeHTTP` runs every public verb on one request and copies many `X-SimpleRedis-*` headers; `Assert-SimpleRedisVerbHeaders` asserts them together. Compose already uses `PathPrefix(`/redis`)` and `PathPrefix(`/dragonfly`)`. Compiled live files already call every public verb except the KEYS Eval case and future MSetEXAt. Yaegi `LiveVerbs` is a subset. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Probe is an HTTP map of SimpleRedis; Pester composes each proof (status + body).
- Compiled Eval KEYS + future MSetEXAt; Yaegi live full public verb set.

**Non-Goals:**
- New compose routers or Redis 8 / native Dragonfly MSETEX.
- Expanding drop-relay beyond Incr+Eval proofs Pester already runs with `drop=1`.
- Growing fake-TCP Yaegi beyond Init/Get/Set/Del/Incr/Eval/MSetEX.
- Token-bucket / windowcounter / reclaim Pester.

## Decisions

1. **Last path segment is the verb.** `/redis` and `/dragonfly` (one segment) are health Set+Get. `/redis/get` → `Get`. `/redis-wrong-password` is one segment → health Set+Get on that handshake plugin instance. Alternative: query `?cmd=` — rejected; requester asked path. Alternative: one Traefik router per verb — rejected; PathPrefix already matches.

2. **HTTP result, not headers.** Success is 200 and the Redis payload in the body. Command errors are 502 and `err.Error()` in the body (same as handshake). The probe does not forward to whoami. Unknown remainder → 404. Query `drop=1` selects `DropHost`.

3. **Eval isolation.** Pester POSTs the Kong KEYS script to `/eval` with query `digest` (SHA-1 of that body). The probe MUST NOT hash. EVALSHA miss-then-hit is two POSTs after `SCRIPT FLUSH`.

4. **Recover / drop / hold live in Pester.** Recover is health warmup, CLIENT KILL, then Set+Get. Drop is `drop=1` on Get warmup then Incr and Eval. Hold is concurrent POST `/eval` with a TIME-wait script (`arg=500000`). Probe `IOTimeout` is 1s so that wait survives the 100ms default.

5. **Compiled KEYS Eval.** Reuse the Kong incrby+expireat body (Lua 5.1-safe, keys in KEYS) in `TestLive_Eval` so compiled e2e matches Traefik argv Pester sends. Keep integer / EVALSHA / FLUSH cases.

6. **Yaegi LiveVerbs.** Add MGet, IncrBy, Expire, ExpireAt, MSetEXAt on the existing interpreted helper. Fake-TCP Yaegi stays; Traefik is Pester.

7. **Pester files by domain; engines are CI jobs.** `scripts/integration-tests.reclaim.Tests.ps1` and `scripts/integration-tests.simpleredis.Tests.ps1`. Helpers in `scripts/integration-tests.utils/` (not `*Tests.ps1`). Redis and Dragonfly are CI jobs that set `-Engine` on the same SimpleRedis file (`INTEGRATION_ENGINE`); no `-TestCases`. Reclaim is `Integration Tests`. Local `./Test-Integration.ps1` runs reclaim then both engines.

## Risks / Trade-offs

- [Pester duration] → Mitigation: a few HTTP calls per `It` is still cheap vs compose boot; a fail names the `It`.
- [Health `/redis` no longer runs every verb] → Mitigation: compose wait only needs 200; verb proofs are Pester sequences on `/redis/<verb>`.
- [Dragonfly undeclared keys] → Mitigation: Pester Eval lists the key in `key=`; same as DestBranch probe KEYS.

## Migration Plan

Test-only. Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
