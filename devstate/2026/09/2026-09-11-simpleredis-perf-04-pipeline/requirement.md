# Requirement
IssueKey: 2026-09-11-simpleredis-perf-04-pipeline

## Problem
SimpleRedis sends every verb as one RESP command and one round trip. `exec` borrows a connection, `do` writes one frame, flushes, and reads one reply. Callers that need a mixed batch (counter plus TTL, N distinct flush keys) pay N network round trips and hold the pooled socket the whole way. There is no pipeline entry point.

## Current (code)
- `simpleredis/simpleredis.go` `exec` — one `do` per call; retries once when a reused idle socket is dead (not on timeout).
- `simpleredis/simpleredis.go` `do` — `SetDeadline`, `writeCommand`, one `readReply`.
- `simpleredis/simpleredis.go` `writeCommand` — encodes one RESP array of bulk strings and always `Flush`es.
- `simpleredis/simpleredis.go` exported verbs — `Get`, `MGet`, `Set`, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval` each call `exec` once. No `ExecPipeline` / `Pipeline`.
- `simpleredis/simpleredis.go` `readReply` — `-` error replies are clean (`reusable`); I/O or protocol errors are not.
- `simpleredis/simpleredis_test.go` `fakeRedis` / `serve` — reads one command then writes that reply; no flush/round-trip counter.
- `simpleredis/yaegi_test.go` — interpreted Init/Set/Get/Del/Incr/Eval against the compiled fake; no pipeline.
- `e2e/simpleredisprobe/plugin.go` — sequential Set/Get/MGet/Del/Incr/IncrBy/Expire/ExpireAt/Eval per request; one header per verb; Kong INCRBY+EXPIREAT Eval with `KEYS[1]`.
- `docker-compose.yml` — `redis:7-alpine` at `redis:6379`; `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379`; `/redis` and `/dragonfly` whoamis already wired to the probe.
- `scripts/integration-tests.Tests.ps1` — Pester asserts per-verb headers on `/redis` and `/dragonfly`; no pipeline header.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — GET/MGET/SET/DEL/INCR/EXPIRE/EVAL and Traefik per-verb headers on both engines; no pipeline requirement.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — dead pooled connection retried once unless timeout (the `exec` policy).

## Desired
- Add `ExecPipeline(commands [][][]byte) ([][][]byte, error)` or a small `Pipeline` type that accumulates then `Exec`s. Borrow one connection, write all frames, flush once, read exactly N replies in order.
- Reuse `writeCommand` per frame and `readReply` per reply. One `SetDeadline` for the whole batch.
- `-ERR` on element 3 of 10 must not abandon remaining replies: read all N, return per-element errors; connection stays reusable. Only I/O or protocol error marks the connection unusable.
- Cap batch size and document the cap.
- Do not retry a pipeline wholesale after a partial read (same double-apply reason as test-05).
- Prove with a fake server that counts flushes: N commands → one flush and N ordered replies; element-3 `-ERR` still returns the other 9 and keeps the conn; mid-pipeline truncation destroys the conn.
- Tests MUST run against both Redis and Dragonfly. A pipeline of mixed verbs MUST be proven live on both engines (not only the flush-counting fake). Extend compose (existing `/redis` and `/dragonfly` whoamis) + Pester those routes + `e2e/simpleredisprobe`. Lua 5.1-safe. Dragonfly KEYS required for any Eval in the mixed batch.

## Affected
- `simpleredis/simpleredis.go`, `simpleredis/simpleredis_test.go`
- `e2e/simpleredisprobe/plugin.go`
- `scripts/integration-tests.Tests.ps1`
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` (and the tcp-session spec if pipeline retry differs from single-command `exec`)
- `docker-compose.yml` only if a new route is required; dest already has Redis and Dragonfly services for `/redis` and `/dragonfly`

## Out of scope
- EVALSHA / SCRIPT LOAD (perf-05).
- MSETEX (feat-01).
- MULTI/EXEC transactions.
- Changing single-command `exec` retry for INCR/INCRBY/EVAL (test-05) except the pipeline-wholesale-retry ban above.
- Pool cap, I/O timeout, idle reaper, encode/decode, `unsafe` (other perf-* findings).
- Rate limiter / leaky bucket / token-limiter implementations.
- go-redis, miniredis, TLS, Unix sockets.
- New compose engines or a second Traefik stack.

## Unknowns
- Numeric batch-size cap (ticket says cap and document it; no number).
- `ExecPipeline` function vs accumulating `Pipeline` type (ticket says or).
- Go shape of per-element errors (slice of error, sentinel plus index, wrapped).
- Whether `writeCommand` keeps its Flush or splits encode vs flush.
- Dragonfly pipeline-specific quirks: research indexes cover EVAL/KEYS/Lua 5.1 and the container image, not pipelining.

## Tensions
- Finding proof is a flush-counting fake; caller HARD REQUIREMENT also requires a live mixed-verb pipeline on Redis and Dragonfly — both are Desired.
- Index suggested order groups perf-04 with perf-05 and feat-01; this ticket is only pipelining.
- `tcp-session` spec requires one retry on a dead pooled conn; this ticket forbids wholesale pipeline retry after a partial read.
