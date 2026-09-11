## Why

SimpleRedis on dest only speaks GET, MGET, SET+EX, and DEL. A later Kong-style window counter needs INCR/INCRBY, EXPIRE/EXPIREAT, and EVAL (atomic incr plus expire-if-new). Until those verbs exist and are proven under Yaegi against Redis and Dragonfly, that limiter cannot be built on this client.

## What Changes

- Add `Incr`, `IncrBy`, `Expire`, `ExpireAt`, and `Eval` on `SimpleRedis` as thin `exec` wrappers. Missing INCR keys are not `redis:miss`. Expire integer `0` or `1` is success. No EVALSHA, pipeline, limiter, `go-redis`, or miniredis.
- Extend `readReply` array parsing so each element may be bulk, integer, or status. Nested arrays stay `redis:issue?`. Do not weaken GET/MGET/SET/DEL.
- Extend the in-process fake Redis for INCR/INCRBY/EXPIRE/EXPIREAT and a fixed Kong EVAL script (no Lua VM). Compiled tests plus Yaegi Incr+Eval against that fake.
- Traefik e2e: every SimpleRedis verb on both `redis:7-alpine` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` in the existing `reclaim-e2e` compose. Do not rename the compose project.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: add INCR/INCRBY, EXPIRE/EXPIREAT, and EVAL; extend array replies; prove Incr+Eval under Yaegi; Traefik e2e MUST exercise every verb against Redis and Dragonfly.

## Impact

- `simpleredis/simpleredis.go`, `simpleredis_test.go`, `yaegi_test.go` (stdlib only; pool/Init/Close behaviour unchanged except Close comments listing new verbs).
- `e2e/simpleredisprobe/`, `docker-compose.yml`, `scripts/integration-tests.Tests.ps1`, `Test-Integration.ps1`, CI failure logs for Dragonfly.
- Main spec `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- Reclaim e2e routes `/a` `/b` stay; SimpleRedis Describe still MUST NOT stop those whoami services.
