## Why

SimpleRedis sends every verb as one RESP command and one round trip. Callers that need a mixed batch (counter plus TTL, N distinct keys) pay N network round trips and hold the pooled socket the whole way. There is no pipeline entry point, so the client cannot prove a one-flush mixed batch on Redis or Dragonfly.

## What Changes

- Add exported `ExecPipeline(commands [][][]byte) ([]PipelineSlot, error)` on `SimpleRedis`. No accumulating `Pipeline` builder type.
- Borrow one connection, encode N frames, flush once, read exactly N replies in order under one `SetDeadline`. Cap is `maxPipelineCommands = 64`. Empty or nil `commands` returns nil, nil and does not dial. Over the cap returns `redis:issue?` and does not send.
- Per-element `-` / `redis:miss` / `redis:noauth` go on `PipelineSlot.Err`. Batch `error` is only I/O, protocol, cap, unreachable, or timeout. Element `-ERR` does not fail the batch error and does not stop remaining reads.
- Split `writeCommand` encode vs Flush: `do` still flushes after one frame; `ExecPipeline` flushes once after N encodes. Leave single-command `exec` retry as-is.
- Prove with a flush-counting fake **and** a live mixed-verb pipeline on Redis and Dragonfly (existing `/redis` and `/dragonfly`; header `X-SimpleRedis-Pipeline`). Yaegi unit test against the compiled fake. Fake flush-count tests are additional, not a substitute for live proof.
- No EVALSHA, MSETEX, MULTI/EXEC, limiter changes, new compose route, or `pipeline_queue_limit` flag.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: add ExecPipeline (cap, empty batch, per-element errors, one flush); prove under Yaegi; Traefik probe MUST send a mixed INCR+EXPIRE+GET+EVAL pipeline and set `X-SimpleRedis-Pipeline` on both Redis and Dragonfly.
- `std_go_simpleredis_tcp-session`: add the pipeline retry rule (retry once on a dead reused idle socket only before any successful Flush; after Flush or any read, no wholesale retry). Do not rewrite the existing single-command `exec` dead-conn sentence.

## Impact

- `simpleredis/simpleredis.go`, `simpleredis_test.go`, `yaegi_test.go`.
- `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.Tests.ps1`.
- Compose routes `/redis` and `/dragonfly` stay; no new service or Traefik stack.
- Main specs `openspec/specs/std_go_simpleredis_resp-commands/spec.md` and `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Limiters, EVALSHA, MSETEX, MULTI/EXEC, pool/timeout/reaper, `go-redis`, miniredis, TLS, Unix sockets: out of scope.
