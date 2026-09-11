## Context

Dest already has `simpleredis/` (GET/MGET/SET+EX/DEL), `e2e/simpleredisprobe` SET+GET on `/redis`, compose `redis:7-alpine`, and Pester Describe `simpleredis Yaegi e2e`. Wire path is `exec` → `writeCommand` → `readReply`. See proposal.md for why. Research: `knowledge/research/ext_redis_incr/`, `ext_redis_expire/`, `ext_redis_eval/`, `ext_dragonfly_eval/`, `ext_dragonfly_container-image/`. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Thin `exec` wrappers for INCR/INCRBY/EXPIRE/EXPIREAT/EVAL plus integer parse.
- Fake Redis and Yaegi coverage without a Lua VM.
- One compose stack, two engines, one probe plugin, headers per verb.

**Non-Goals:**
- Rate limiter, EVALSHA, pipeline, EXISTS as a Go method.
- Renaming compose project `reclaim-e2e`.
- Host-publishing Dragonfly (Traefik uses compose DNS `dragonfly:6379`).

## Decisions

1. **Reuse `exec`.** New methods convert string keys/args to `[]byte` at the call site. `parseIntegerReply` is unexported. Alternative: a second RESP client — rejected; consume before produce.

2. **Array `*` accepts `$`, `:`, `+`.** Nested `*` or `-` → `redis:issue?` and `reusable false` when the rest cannot be skipped. MGET still only sees bulks from Redis. Alternative: parse nested arrays — out of scope.

3. **Fake Redis, no Lua VM.** INCR/INCRBY store decimal strings and reply `:<n>`. EXPIRE/EXPIREAT record `lastExpire` and reply `:1`. EVAL matches the Kong incrby+expireat script string used by tests; encoder-only tests use `startStaticRedis`. Alternative: miniredis — out of scope.

4. **Dragonfly pin `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`.** Internal hostname `dragonfly`, port 6379, `ulimits.memlock: -1`. Do not use Docker Hub `dragonflydb/dragonfly` (stale). Alternative: compose profiles / second stack — rejected; dest already shares `reclaim-e2e`.

5. **One probe `ServeHTTP`.** `Config.Host` selects the engine. Unique keys per request so `/redis` and `/dragonfly` do not collide. Keep `X-SimpleRedis-Value` for GET after SET. Eval body is the Kong snippet with `KEYS[1]` (Dragonfly forbids undeclared keys). Alternative: one path per verb — extra routers; Pester can fail a missing header instead.

6. **Dragonfly health.** `docker compose exec -T redis redis-cli -h dragonfly ping` (Dragonfly image may omit redis-cli). Wait HTTP `/dragonfly` like `/redis`.

## Risks / Trade-offs

- [Dragonfly Lua 5.4 vs Redis 5.1] → Mitigation: Kong snippet has no `table.maxn`; KEYS declared.
- [Compose still named `reclaim-e2e`] → Mitigation: existing debt; do not rename this change.
- [memlock / RAM on CI] → Mitigation: official `ulimits.memlock: -1`; `--proactor_threads=1` only if CI OOMs.
- [Reclaim Pester stops a/b] → Mitigation: SimpleRedis Describe never stops those services.

## Migration Plan

Library additive API. Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
