## Context

Dest already has `simpleredis/` (`exec` → `do` → `writeCommand` Flush → `readReply`), `e2e/simpleredisprobe` sequential verbs on `/redis` and `/dragonfly`, and Pester per-verb headers. See proposal.md for why. Research: `knowledge/research/ext_redis_pipelining/`, `knowledge/research/ext_dragonfly_pipelining/`. Proceed policies: `devstate/explore.md`. Usage: `knowledge/devdocs/std_go_simpleredis.md`.

## Goals / Non-Goals

**Goals:**
- One exported `ExecPipeline` that encodes N, flushes once, reads N ordered slots.
- Split encode vs Flush without reshaping `do` into an N-command helper (AUTH/SELECT stay one-command).
- Live mixed-verb proof on Redis and Dragonfly via existing compose routes, plus a flush-counting fake and a Yaegi test.

**Non-Goals:**
- `Pipeline` builder type, MULTI/EXEC, EVALSHA, MSETEX, limiter call-site changes.
- New compose service, route, or Dragonfly `--pipeline_queue_limit` / `--pipeline_buffer_limit`.
- Changing single-command `exec` retry.

## Decisions

1. **Exported `ExecPipeline` only.** Signature `([][][]byte) ([]PipelineSlot, error)` with `PipelineSlot{Values [][]byte; Err error}`. Empty/nil: `nil, nil`, no dial (same as `MGet`). Over 64: `redis:issue?`, no send. Cap `maxPipelineCommands = 64` (under Redis 10k example, Dragonfly queue 10000, and 1s `ioTimeout`). Alternative: a builder type — rejected; callers already hold argv rows; no in-tree pipeline caller (limiters out of scope); Yaegi-safe one method.

2. **Split Flush out of `writeCommand`.** `writeCommand` encodes only. `do` Flushes after one frame (AUTH/SELECT in `dial` keep working). `ExecPipeline` encodes N then Flushes once. Alternative: keep Flush in `writeCommand` — rejected; that is N round trips.

3. **`execPipeline` sibling of `exec`.** Do not change `exec` / `do` retry. Do not fold N commands into `do`. Retry the pipeline once only when the socket was reused, the error is not timeout, and Flush has not succeeded (dead idle fd on encode or Flush). After Flush returns nil, or after any `readReply`, never retry the batch (same double-apply reason as test-05). Partial read → conn not reusable, return I/O/protocol error, do not continue remaining replies.

4. **Flush-counting fake.** Same-package test helper: `net.Pipe` (or wrap the client `net.Conn`) with a `countingConn` that counts `Write`. Seed `sr.idle` with that conn so there is no production dial hook. N small commands → one Write. Element-3 `-ERR` still returns the other 9 and a later command reuses the conn. Mid-pipeline close/truncation destroys the conn and is not retried. Keep `startFakeRedis` loop. Do not add a production flush counter. Fake tests are additional, not a substitute for live engines.

5. **Probe header on existing routes.** Keep every existing `X-SimpleRedis-*` header. After those verbs, one mixed `ExecPipeline` on unique per-request keys: `INCR` and `EXPIRE` and GET of that incr key, plus `EVAL` of `kongIncrbyExpireatScript` with `KEYS[1]` on a distinct eval key (so EVAL returns `3`, not 4). Set `X-SimpleRedis-Pipeline` to `1:ok:1:3`. Pester asserts that value on `/redis` and `/dragonfly`. No new compose service or route. Yaegi: interpreted `ExecPipeline` against the compiled fake (GOPATH, `useunsafe` false).

6. **Dragonfly as Redis-compat RESP2.** Do not set `--pipeline_queue_limit` / `--pipeline_buffer_limit`. Cap 64 is under defaults. Flush-then-read-N avoids huge write-without-read deadlock. Each EVAL lists KEYS; scripts stay Lua 5.1-safe. No MULTI/EXEC. No second code path.

## Risks / Trade-offs

- [Partial Flush on a dead fd could have sent bytes, then retry double-applies] → Mitigation: retry only when Flush has not succeeded; after Flush nil, never retry. Same shape as current `exec`.
- [Dragonfly Lua 5.4 / undeclared keys] → Mitigation: Kong snippet, `KEYS[1]`, distinct eval key in the mixed batch.
- [Fake Write-count ≠ live engines] → Mitigation: Pester `/redis` and `/dragonfly` is required; fake is extra.
- [Yaegi cannot type-assert slot errors] → Mitigation: `PipelineSlot.Err` with `Error()` text only.

## Migration Plan

Library additive API. Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
