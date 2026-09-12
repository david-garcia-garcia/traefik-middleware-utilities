## Context

See proposal.md. Dest `ServeHTTP` runs every public verb on one request and copies many `X-SimpleRedis-*` headers; `Assert-SimpleRedisVerbHeaders` asserts them together. Compose already uses `PathPrefix(`/redis`)` and `PathPrefix(`/dragonfly`)`. Compiled live files already call every public verb except the KEYS Eval case and future MSetEXAt. Yaegi `LiveVerbs` is a subset. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Path-case dispatch in the probe; one Pester `It` per case per engine.
- Compiled Eval KEYS + future MSetEXAt; Yaegi live full public verb set.

**Non-Goals:**
- New compose routers or Redis 8 / native Dragonfly MSETEX.
- Expanding recover or drop-relay to the full verb set.
- Growing fake-TCP Yaegi beyond Init/Get/Set/Del/Incr/Eval/MSetEX.
- Token-bucket / windowcounter / reclaim Pester.

## Decisions

1. **Last path segment is the case.** `/redis` and `/dragonfly` (one segment) are health Set+Get. `/redis/get` → `get`. `/redis-wrong-password` is one segment → health Set+Get on that handshake plugin instance. Alternative: query `?cmd=` — rejected; requester asked path. Alternative: one Traefik router per verb — rejected; PathPrefix already matches.

2. **One result per case.** Each case still sets a header (or 502 with the command error). Isolation is separate `It`s, not deleting HTTP assertions. Unknown remainder → 404.

3. **Eval isolation.** `/eval` runs the Kong KEYS script once per request. EVALSHA miss-then-hit is two GETs after `SCRIPT FLUSH`, not Eval twice in one ServeHTTP. Digest header stays on `/eval`.

4. **Recover / drop / hold.** `/recover`, `/drop`, `/hold` keep DestBranch jobs (Set+Get; Incr+Eval; TIME wait). Query `?recover=1` and `?hold=` on the dump path go away.

5. **Compiled KEYS Eval.** Reuse the probe’s Kong incrby+expireat body (Lua 5.1-safe, keys in KEYS) in `TestLive_Eval` so compiled e2e matches Traefik argv. Keep integer / EVALSHA / FLUSH cases.

6. **Yaegi LiveVerbs.** Add MGet, IncrBy, Expire, ExpireAt, MSetEXAt on the existing interpreted helper. Fake-TCP Yaegi stays; Traefik is Pester.

## Risks / Trade-offs

- [Pester duration] → Mitigation: one HTTP call per case is still cheap vs compose boot; drop the all-headers helper so a fail names the `It`.
- [Health `/redis` no longer runs every verb] → Mitigation: compose wait only needs 200; verb proofs move to `/redis/<case>`.
- [Dragonfly undeclared keys] → Mitigation: KEYS Eval lists the key; same as DestBranch probe.

## Migration Plan

Test-only. Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
