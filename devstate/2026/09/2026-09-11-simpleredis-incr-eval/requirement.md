# Requirement
IssueKey: 2026-09-11-simpleredis-incr-eval

## Problem
SimpleRedis on `master` only implements GET, MGET, SET+EX, and DEL. A later Kong-style rate-limit store needs INCR/INCRBY, EXPIRE/EXPIREAT, and EVAL (atomic incr+conditional expire). Without these commands the client cannot talk to Redis or Dragonfly for window counters.

## Current (code)
- `simpleredis/simpleredis.go` — exported `Init`, `Get`, `MGet`, `Set`, `Del`, `Close`; no `Incr`, `IncrBy`, `Expire`, `ExpireAt`, or `Eval`.
- `simpleredis/simpleredis.go` `readReply` — `*` arrays accept only `$` bulk elements; `:`/`+` elements in arrays → `redis:issue?`.
- `simpleredis/simpleredis_test.go` `startFakeRedis` — handles AUTH, SELECT, GET, MGET, SET; unknown commands (including DEL) reply `+OK`; no INCR/EXPIRE/EVAL or integer replies.
- `simpleredis/yaegi_test.go` — Yaegi probe exercises Init/Set/Get/Del only.
- `e2e/simpleredisprobe/plugin.go` — Traefik probe SET+GET on each request; no Incr/Eval/Expire.
- `docker-compose.yml` — `redis:7-alpine` at `redis:6379`; no Dragonfly service.
- `scripts/integration-tests.Tests.ps1` — Pester asserts `/redis` SET+GET header only.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — requirements for GET/MGET/SET/DEL and Yaegi Init/Get/Set/Del; no INCR/EXPIRE/EVAL.

## Desired
- Add `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval` on `SimpleRedis` per handoff wire shapes; thin wrappers over existing `exec`/`writeCommand`/`readReply`.
- Extend `readReply` `*` parsing so elements may be `$`, `:`, or `+` (null bulk stays nil slot); nested `*`/`-` → `redis:issue?`; MGET unchanged.
- Extend `startFakeRedis` for INCR/INCRBY/EXPIRE/EXPIREAT and fixed-script EVAL (no Lua VM, no miniredis); compiled unit tests per handoff scenarios.
- Extend Yaegi tests to prove interpreted `Incr` and `Eval` against the compiled fake.
- Update `std_go_simpleredis_resp-commands` spec with the new commands; do not weaken existing GET/SET/DEL scenarios.
- **E2E (human addendum):** Pester/compose tests must exercise every SimpleRedis interaction (Incr, IncrBy, Expire, ExpireAt, Eval, plus existing Get/Set/Del/MGet as needed) against **both** Redis and Dragonfly backends.

## Affected
- `simpleredis/simpleredis.go`, `simpleredis/simpleredis_test.go`, `simpleredis/yaegi_test.go`
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` (+ change folder during propose/implement)
- `e2e/simpleredisprobe/` (extend probe routes/headers for new commands)
- `docker-compose.yml` (add Dragonfly service or matrix)
- `scripts/integration-tests.Tests.ps1`

## Out of scope
- Rate limiter, window counter, sync_rate flusher, token bucket.
- EVALSHA, SCRIPT LOAD, pipeline, MULTI/EXEC.
- go-redis, miniredis, TLS, Unix sockets.
- EXISTS as a Go method; GET of counters.
- Raising pool/timeouts; changing Init/Close/pool behaviour.
- Traefik token-bucket Lua / HGETALL / HSET.

## Unknowns
- Exact Dragonfly image/tag and compose wiring for this repo (not present on `master`).
- How to run the same Pester Describe against Redis and Dragonfly (separate compose profiles, parameterized host, or two services) without breaking existing reclaim e2e.
- Which response headers or routes best prove each new command under Yaegi in Traefik (probe design TBD in explore).

## Tensions
- Handoff § Tests calls Pester/compose Redis **optional** and prefers compiled + Yaegi coverage; human addendum **requires** E2E for all interactions on **both** Redis and Dragonfly — follow the addendum.
- Handoff says extend probe only if the change touches `e2e/simpleredisprobe`; human addendum requires E2E — probe and compose must be extended.
