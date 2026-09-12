# Requirement
IssueKey: 2026-09-11-simpleredis-test-06-malformed-reply

## Problem
`readReply` / `readLine` defensive branches for impossible or truncated RESP, plus `Get`/`parseIntegerReply` reply-count checks and `exec`'s retry-borrow failure, are unexercised. Today only nested-array and garbage-integer cases exist. A malformed reply (proxy, HTTP on a Redis port, truncated array) must fail as `redis:issue?` (or I/O) and leave the connection unusable. RESP2 null array `*-1` is a legal reply that dest currently treats as the same `count < 0` violation.

## Current (code)
- `simpleredis/simpleredis.go` `readReply` (`:329-386`) — empty line → `errIssue` `clean=false`; `*` count unparseable or `< 0` (includes `*-1`) → `errIssue` `clean=false`; array element line I/O / empty / unknown type → `errIssue` or I/O, `clean=false`; unknown top-level type byte → `errIssue` `clean=false`; top-level `$` bulk I/O (non-miss) → I/O `clean=false`. Null bulk `$-1` in `readBulk` (`:397-398`) is `errMiss` `clean=true`.
- `simpleredis/simpleredis.go` `readLine` (`:408-417`) — line not ending in `CR` before `LF` → `errIssue`.
- `simpleredis/simpleredis.go` `Get` (`:93-95`) — `len(values) != 1` → `errIssue`. `parseIntegerReply` (`:171-173`) — same for INCR/INCRBY.
- `simpleredis/simpleredis.go` `exec` (`:195-197`) — after a dirty reused conn, `borrow` failure on the retry attempt returns that error. `do` (`:299-304`) + `release` (`:245-248`) close a non-clean conn, so it is not returned to `idle`.
- `simpleredis/simpleredis_test.go` `TestEvalNestedArrayIsIssue` (`*1\r\n*0\r\n`) and `TestIncrGarbageIntegerPayload` (`:not-an-int\r\n`) — only malformed cases; they do not assert `len(redis.idle) == 0`. `startStaticRedis` (`:228-256`) already replays a canned reply.
- `docker-compose.yml` — `redis:7-alpine` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` already present; `whoami-redis` `/redis`, `whoami-dragonfly` `/dragonfly`.
- `scripts/integration-tests.Tests.ps1` — Pester already asserts every verb header on `/redis` and `/dragonfly`.
- `e2e/simpleredisprobe/plugin.go` — live Eval uses `kongIncrbyExpireatScript` (KEYS declared, no `table.maxn`).
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — Traefik e2e on both engines; no malformed-reply scenarios. Null bulk GET is `redis:miss`; null array is not specified.

## Desired
- Table-driven compiled test via `startStaticRedis` over canned malformed replies (unknown type including HTTP-shaped, missing CR, empty line, bad array count including `*-1`, truncated array elements, bad element type) asserting the expected error **and** `len(redis.idle) == 0`. Cover the listed `Get` / `parseIntegerReply` count-mismatch branches the same way.
- Malformed cases stay fake-server. Keep live happy-path coverage on **both** Redis and Dragonfly (`/redis` and `/dragonfly` Pester + compose) so a decoder change cannot ship unproven. Extend those routes if a decoder change would otherwise leave them stale; do not replace them with fake-server-only coverage.
- Tests MUST run against both Redis and Dragonfly. Both are supported backends.
- Decide null-array (`*-1`) deliberately in explore (keep as protocol violation + comment at `:354`, or map to `errMiss` like null bulk). Do not leave it as an accident of `count < 0`.
- Any Eval script in live tests stays Lua 5.1-safe and lists keys in KEYS (Dragonfly).

## Affected
- `simpleredis/simpleredis_test.go` (table-driven malformed replies; idle invariant)
- `simpleredis/simpleredis.go` — only if explore maps `*-1` or adds the `:354` comment
- `docker-compose.yml`, `scripts/integration-tests.Tests.ps1`, `e2e/simpleredisprobe/` — keep/extend live `/redis` `/dragonfly`
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` if the malformed or null-array contract is specified

## Out of scope
- Other `simpleredisfixes/` findings (perf-*, test-01–05, test-07, feat-01).
- MULTI/EXEC, BLPOP, EVALSHA, pipelining, pool-cap / I/O-timeout product changes.
- go-redis, miniredis, TLS.
- Driving malformed replies from live Redis or Dragonfly (those engines will not emit them on the happy path).
- Changing AUTH/SELECT, Init, or Close behaviour except as a side effect of a decided `*-1` mapping.

## Unknowns
- Null-array (`*-1`): keep as `redis:issue?` + destroy conn (document at `:354`), or map to `errMiss` like null bulk. Record the decision on explore.md. Official RESP2 says the client should return a null object (`knowledge/research/ext_redis_resp_null-array/`). No current SimpleRedis verb is specified to receive a null array.
- How to exercise `exec` borrow-failure on the retry attempt (`:195`) with a canned reply; the finding lists the block, how-to-fix does not give a recipe.

## Tensions
- Finding how-to-fix is a compact fake-server table; conductor HARD REQUIREMENT also keeps live happy-path on Redis and Dragonfly — both apply (fake malformed + live happy-path).
- RESP2 null array is a legal nil reply (BLPOP timeout, EXEC abort); dest `count < 0` treats it as a protocol violation and destroys the socket. Explore must choose; prepare does not.
- `Get` / `parseIntegerReply` count checks are called lowest-value in the finding but remain in the branch table and Desired.
