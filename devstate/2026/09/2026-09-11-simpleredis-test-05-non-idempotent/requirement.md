# Requirement
IssueKey: 2026-09-11-simpleredis-test-05-non-idempotent

## Problem
`exec` retries every command after a dead reused socket, including `INCR`, `INCRBY`, and `EVAL`. When Redis already applied the write and the reply was lost, those verbs run twice. Rate-limit counters then over-count. Dest has no test that pins the stored value after a lost reply, and no stated policy. The ask is: retry only idempotent commands; return `redis:unreachable` for `INCR`/`INCRBY`/`EVAL` instead of double-applying; prove that on the in-process fake and on live Redis and Dragonfly.

## Current (code)
- `simpleredis/simpleredis.go` `exec` (lines 181–201) — after `do` fails on a reused connection, retries once unless the error is `redis:timeout`. The args slice is sent again with no verb check.
- `simpleredis/simpleredis.go` `Incr` / `IncrBy` / `Eval` (132–164) — all call `exec` with no retry flag. `Get`/`MGet`/`Set`/`Del`/`Expire`/`ExpireAt` share the same `exec`.
- `simpleredis/simpleredis.go` `do` (291–307) — `reusable` is false on write or read IO failure. A successful write plus a failed read still looks like a dead socket.
- `simpleredis/simpleredis_test.go` `TestStaleConnectionIsRetried` — only `Get` after the client closes its idle socket (write never lands). No assert of store after a server-side apply-then-drop.
- `simpleredis/simpleredis_test.go` `fakeRedis.serve` — `INCR`/`INCRBY`/`EVAL` mutate `store` then write the reply; no execute-then-close-before-reply path. Truncated-bulk / test-04 harness: not found.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — “A dead pooled connection SHALL be retried once unless the error is a timeout.” No verb exception.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — command shapes for INCR/EVAL; no retry/idempotency rule.
- `e2e/simpleredisprobe/plugin.go` and `scripts/integration-tests.Tests.ps1` — `/redis` and `/dragonfly` happy-path headers only. `docker-compose.yml` already has `redis:7-alpine` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`.
- `knowledge/research/ext_redis_incr/notes.md` — INCR/INCRBY mutate in place (missing key starts at 0). `knowledge/research/ext_dragonfly_eval/notes.md` — undeclared KEYS rejected; Lua 5.4 vs Redis 5.1 (`table.maxn` absent).

## Desired
- Retry only idempotent verbs (`GET`, `MGET`, `SET`, `DEL`, `EXPIRE`, `EXPIREAT` as named in the finding). `INCR`, `INCRBY`, and `EVAL` MUST NOT be retried after a reused-socket failure; they return `redis:unreachable`.
- Tag retry-safety at the `exec` call site (bool or small descriptor). Timeouts stay non-retried for every verb.
- Fake: execute, mutate store, close before writing the reply. One `Incr` leaves stored `1` and returns `redis:unreachable`, not silent `2`. Mirror test: an idempotent verb still retries and the caller sees success.
- Prove the same Incr/Eval policy on live Redis and Dragonfly as well as the fake. Extend compose + Pester `/redis` `/dragonfly`. Eval scripts Lua 5.1-safe; keys listed in KEYS.
- Explore writes the policy Decision on `explore.md` (prepare does not write that file).

## Affected
- `simpleredis/simpleredis.go` (`exec` and command wrappers)
- `simpleredis/simpleredis_test.go` (fake lost-reply harness + compiled tests)
- `simpleredis/yaegi_test.go` if interpreted Incr/Eval must see the same error
- `e2e/simpleredisprobe/`, `docker-compose.yml`, `scripts/integration-tests.Tests.ps1`
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (retry rule)
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` if command-level retry is specified there

## Out of scope
- Other `simpleredisfixes/` findings (perf-01–08, test-01–04, test-06–07, feat-01).
- Pipelining (perf-04 is not on dest; do not add a pipeline to attach this policy).
- Distinguishing write-flush failure from read failure as the retry gate.
- A client option that lets callers opt into retrying INCR/EVAL.
- Making limiter Lua idempotent with a request token; changing windowcounter or tokenbucket.
- EVALSHA / SCRIPT LOAD; go-redis; miniredis.

## Unknowns
- How to inject “applied then drop the reply” on live Redis and Dragonfly (those servers have no such hook; a compose RESP proxy or equivalent is not on dest).
- Whether the test-04 truncated-bulk fake (not on dest) should be invented here or only a lost-reply close-before-reply path.
- Exact `exec` tagging shape (bool vs descriptor) — propose owns the design.
- Whether Yaegi tests must cover the lost-reply error, or compiled + Pester suffice.

## Tensions
- Finding lists four policies; caller assumed “retry only idempotent commands” unless dest contradicts. Dest retries every verb — that is the gap, not a contradiction of the assumed policy.
- Finding “How to prove it” is fake-only; caller HARD REQUIREMENT adds live Redis and Dragonfly. Both are Desired.
- Finding says extend the test-04 fake; test-04 is not applied on dest.
- tcp-session spec currently requires one retry for any dead pooled connection; Desired narrows that by verb.
