## Why

`exec` retries every verb after a dead reused socket. When Redis already applied `INCR` / `INCRBY` / `EVAL` and the reply was lost, a retry double-applies. Rate-limit counters then over-count. Dest has no stored-value pin after a lost reply and no verb exception on the tcp-session retry rule.

## What Changes

- Retry only `GET`, `MGET`, `SET`, `DEL`, `EXPIRE`, and `EXPIREAT` after a dead reused socket. `INCR`, `INCRBY`, and `EVAL` return `redis:unreachable` without a second send. Timeouts stay non-retried for every verb.
- Pass a bool at each of the nine `exec` call sites (`true` for the six retry-safe verbs, `false` for `INCR` / `INCRBY` / `EVAL`). No command-descriptor table. No client option.
- Fake Redis: mutate, then close before writing the reply. One `Incr` leaves stored `1` and returns `redis:unreachable`. A retry-safe verb still retries and the caller sees success.
- Live proof: compose RESP drop-relay in front of Redis and Dragonfly. Same `/redis` and `/dragonfly` requests set extra headers. Pester asserts unreachable plus stored `1` (Incr) and stored script result (Eval, Kong KEYS snippet, Lua 5.1-safe).
- Yaegi stays happy-path. Do not change windowcounter, tokenbucket, pipelining, EVALSHA, go-redis, or miniredis.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: narrow the dead-pool retry rule by verb. GET/MGET/SET/DEL/EXPIRE/EXPIREAT stay retried once unless timeout. INCR/INCRBY/EVAL MUST NOT be retried; they return `redis:unreachable`. Prove on the in-process fake and on live Redis and Dragonfly via compose drop-relay + Pester `/redis` `/dragonfly`. Do not put this rule on `resp-commands`.

## Impact

- `simpleredis/simpleredis.go` (`exec` and the nine wrappers). Exported `Incr` / `Eval` signatures stay.
- `simpleredis/simpleredis_test.go` (close-before-reply fake + compiled tests). `yaegi_test.go` unchanged besides staying green on happy-path Incr/Eval.
- `e2e/respdroprelay/` (compose sidecar), `e2e/simpleredisprobe/`, `docker-compose.yml`, `scripts/integration-tests.Tests.ps1`.
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Reclaim e2e `/a` `/b` and happy-path SimpleRedis headers stay. SimpleRedis Describe still MUST NOT stop those whoami services.
