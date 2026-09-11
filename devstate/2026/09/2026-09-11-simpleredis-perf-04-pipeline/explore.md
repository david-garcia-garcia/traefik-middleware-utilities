# Explore
IssueKey: 2026-09-11-simpleredis-perf-04-pipeline

## Concepts

- **SimpleRedis wire today** — `exec` borrows, `do` sets one deadline, `writeCommand` encodes one RESP array and always `Flush`es, `readReply` reads one value (`simpleredis/simpleredis.go`). Exported verbs (`Get` … `Eval`) each call `exec` once. No `ExecPipeline` / `Pipeline`. `-` replies are `clean` (conn reusable); I/O and protocol errors are not.
- **Dead-idle retry** — `exec` retries once when a reused idle socket is dead and the error is not timeout (`simpleredis.go` `exec`; `TestStaleConnectionIsRetried`). `tcp-session` names that rule for commands. Out of scope: changing that path for INCR/INCRBY/EVAL.
- **Fake Redis** — `fakeRedis.serve` already loops `readCommand` then one reply on the same socket (`simpleredis_test.go`), so a pipelined client would be consumed, but nothing counts client flushes. `startStaticRedis` replies the same canned blob per command.
- **Call sites of `writeCommand`** — one: `do` at `simpleredis/simpleredis.go` (`writeCommand(` in `*.go`). `do` is also AUTH/SELECT in `dial`.
- **Consumers** — `windowcounter/limiter.go` `takeExact` does `Incr` then `Expire` (two round trips); buffered flush loops `Eval` per key. Token-bucket uses `Eval`. **Out of scope:** changing those limiters. This ticket only adds the client entry point.
- **E2E already dual-engine** — compose `redis:7-alpine` + `dragonfly:v1.40.2`; probe `e2e/simpleredisprobe/plugin.go` sequential verbs; Pester `/redis` and `/dragonfly` headers `X-SimpleRedis-*`. No pipeline header. No new Traefik stack (out of scope).
- **Specs** — `std_go_simpleredis_resp-commands` has per-verb + Traefik headers, no pipeline. `std_go_simpleredis_tcp-session` has dead-conn retry for single `exec`.
- **Usage packet** — `knowledge/devdocs/std_go_simpleredis.md` Language lists Init/Get/…/Eval/Close. No pipeline usage. Gap expected until the method exists; no Language term invented (unattended).
- **Research this phase** — `knowledge/research/ext_redis_pipelining/` (N writes then N replies; mixed verbs including EVAL; `-ERR` is per-element; I/O desyncs the stream). `knowledge/research/ext_dragonfly_pipelining/` (Redis-compat pipeline; `--pipeline_queue_limit` default 10000; `--pipeline_buffer_limit` 128MiB; huge write-without-read can deadlock). KEYS / Lua 5.1 already in `ext_redis_eval` + `ext_dragonfly_eval`.
- **Gap measured** — missing API, not a crashing test. `go test` was not re-run this phase (unattended explore of an absent entry point).

```
  today (N verbs):                  desired pipeline:
  borrow ── do ── Flush ── read     borrow ── write×N ── Flush once ── read×N
            × N round trips                    one deadline
```

## Decisions

- Add `ExecPipeline(commands [][][]byte) ([]PipelineSlot, error)` on `SimpleRedis`. No accumulating `Pipeline` type. Empty / nil `commands`: return nil, nil and do not dial (same as `MGet` empty). Over the cap: `redis:issue?`, do not send.
- Cap `maxPipelineCommands = 64`. Document on the method and in `resp-commands`. Official Redis example batch is 10k; Dragonfly default queue is 10000; one `ioTimeout` (1s) covers the whole batch — 64 stays under both engines and the 1s deadline.
- `PipelineSlot` is `{Values [][]byte; Err error}`. Batch `error` is only I/O, protocol, cap, unreachable, timeout. Per-element `-` / `redis:miss` / `redis:noauth` go on `slot.Err` via existing `replyError` / `readReply`. Length of the slice is N when every reply was read. Do not type-assert (Yaegi matches `Error()` text). Not a Language term until implement/devdocs; Go type only.
- Split Flush out of `writeCommand`: encode only; `do` Flushes after one frame; `ExecPipeline` encodes N frames then Flushes once. Migrate the one `do` call site.
- New `execPipeline` sibling of `exec`. Do not change `exec` / `do` retry. Do not reshape `do` into an N-command helper (AUTH/SELECT stay one-command).
- Dead pooled conn: retry the pipeline **once** only when the socket was reused, the failure is not timeout, and **no frame has been Flushed** (write failed on a dead idle fd). After any Flush, or after any `readReply` (including a first `-ERR`), never retry the batch. Partial read → conn not reusable, return the I/O/protocol error, do not continue remaining replies (stream is broken). `-ERR` on element 3 of 10: read all 10, conn reusable.
- Fake: wrap the accepted `net.Conn` so tests count `Write`s (flush-counting). N small commands → one flush / one Write coalesced; element-3 `-ERR` still returns the other 9 and a later command reuses the conn; mid-pipeline close/truncation destroys the conn and is not retried. Keep `startFakeRedis` loop; do not add a production flush counter.
- Probe: keep every existing `X-SimpleRedis-*` header. After those verbs, one mixed `ExecPipeline` on unique per-request keys: `INCR`, `EXPIRE`, `GET` of that incr key, `EVAL` of `kongIncrbyExpireatScript` with `KEYS[1]` (Lua 5.1-safe). Set `X-SimpleRedis-Pipeline` to `1:ok:1:3`. Pester asserts that value on `/redis` and `/dragonfly`. No new compose service or route. Extend `yaegi_test.go` with an interpreted `ExecPipeline` against the compiled fake (same GOPATH/`useunsafe` false pattern).
- Spec: grow `resp-commands` with ExecPipeline, cap, per-element errors, dual-engine header. Grow `tcp-session` with the pipeline retry rule; leave the single-command dead-conn sentence as-is.
- Limiters, EVALSHA, MSETEX, MULTI/EXEC, pool/timeout/reaper, go-redis, miniredis, TLS, Unix sockets: out of scope.

## Open questions

- Q: What numeric batch-size cap does ExecPipeline enforce?
  Rank: additive asked — new constant on the method this change creates; Desired "Cap batch size and document the cap"
  Decision: assumed — `maxPipelineCommands = 64`; empty batch does not dial; over cap is `redis:issue?` without sending. 64 is under Redis's 10k example batch, Dragonfly `pipeline_queue_limit` 10000, and the 1s `ioTimeout` for the whole batch.
  By: explore

- Q: `ExecPipeline` function vs accumulating `Pipeline` type?
  Rank: additive asked — Desired names "ExecPipeline or a small Pipeline type"; this change creates the entry point; existing Get/Set/Eval callers keep working
  Decision: assumed — exported `ExecPipeline` only. Callers already hold `[][][]byte`. A builder type would be a second object with no in-tree caller (limiters are out of scope). Yaegi-safe: one method.
  By: explore

- Q: What Go shape carries per-element errors?
  Rank: additive asked — Desired "return per-element errors" and the sketched `([][][]byte, error)` conflict; new return contract this change creates
  Decision: assumed — `([]PipelineSlot, error)` with `PipelineSlot{Values, Err}`. Batch error is I/O/protocol/cap only. Element `-ERR` does not fail the batch error and does not stop remaining reads. Do not invent a Language term for the slot; implementers use the type. Sketch `([][][]byte, error)` cannot carry per-element errors without a type assert Yaegi must not need.
  By: explore

- Q: Does `writeCommand` keep its Flush or split encode vs flush?
  Rank: bounded asked — Desired "Reuse writeCommand per frame" and "write all frames, flush once"; 1 existing call site (`do` at `simpleredis/simpleredis.go`), searched `*.go` for `writeCommand(`
  Decision: assumed — split: `writeCommand` encodes; `do` Flushes once per command; ExecPipeline Flushes once after N encodes. AUTH/SELECT in `dial` keep working because they go through `do`.
  By: explore

- Q: How does pipeline retry relate to tcp-session dead-conn retry?
  Rank: additive asked — Desired "Do not retry a pipeline wholesale after a partial read"; Out of scope forbids changing single-command `exec` retry; Affected allows tcp-session if pipeline retry differs
  Decision: assumed — leave `exec` as-is. `execPipeline` may retry once on a dead reused idle socket only before any Flush (same double-apply reason as test-05 after bytes hit the server). After Flush or any read, no wholesale retry. Spec adds that requirement; does not rewrite the existing `exec` sentence.
  By: explore

- Q: What live mixed-verb probe header proves Redis and Dragonfly?
  Rank: additive asked — Desired live mixed-verb pipeline on both engines; Affected `e2e/simpleredisprobe`, Pester, compose only if a new route is required
  Decision: assumed — no new route. One header `X-SimpleRedis-Pipeline: 1:ok:1:3` from INCR + EXPIRE + GET + EVAL(`KEYS[1]`, Kong snippet) after the existing sequential headers. Pester both `/redis` and `/dragonfly`. Unique keys per request. Yaegi unit test also calls ExecPipeline against the compiled fake.
  By: explore

- Q: What Dragonfly pipeline quirks change the client or compose?
  Rank: additive asked — Unknowns name Dragonfly pipelining; Desired live proof on Dragonfly; research indexes had EVAL/KEYS/image only
  Decision: resolved — none that need a compose flag or a second code path. Treat as Redis-compat RESP2: one flush, N ordered replies, `-ERR` does not drop later replies. Do not set `--pipeline_queue_limit` / `--pipeline_buffer_limit` (defaults 10000 / 128MiB; cap 64 is under both). Huge write-without-read deadlock is avoided by flush-then-read-N. Each EVAL in the batch still lists KEYS; scripts stay Lua 5.1-safe (`ext_dragonfly_eval`). No MULTI/EXEC.
  By: explore
