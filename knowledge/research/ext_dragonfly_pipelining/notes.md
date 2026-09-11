# Dragonfly pipelining

Dragonfly accepts Redis-style pipelining on the same RESP TCP connection: the client may write several commands without waiting, then read one reply per command. Official flags describe a per-connection pipeline queue and buffer; vendor docs present pipelining as a supported Redis-compat batch method, not MULTI/EXEC. ([Dragonfly flags — pipeline_queue_limit / pipeline_buffer_limit](https://www.dragonflydb.io/docs/managing-dragonfly/flags), [.sources/flags-pipeline.md](.sources/flags-pipeline.md); [Dragonfly blog — Batch operations](https://www.dragonflydb.io/blog/batch-operations-in-dragonfly), [.sources/batch-operations.md](.sources/batch-operations.md))

## Defaults on the compose image (v1.40.x flags page)

| Flag | Default | What it does |
|------|---------|----------------|
| `--pipeline_queue_limit` | `10000` | Stop reading the client socket once queued pipeline commands exceed this; resume after draining. Huge pipelines may need a higher value to avoid deadlock. |
| `--pipeline_buffer_limit` | `128.00MiB` | Memory for pipeline requests per IO thread. Excessively huge pipelines may deadlock themselves. |

([flags](https://www.dragonflydb.io/docs/managing-dragonfly/flags), [.sources/flags-pipeline.md](.sources/flags-pipeline.md))

Compose `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` does not pass these flags today (`docker-compose.yml` dragonfly service). A client batch of tens of commands stays under both defaults. Do not raise the flags for e2e unless a test actually deadlocks.

## Huge pipeline deadlock (why a small client cap matters)

If a client writes a very large pipeline and does not read replies until the write finishes, Dragonfly can fill the socket, stop reading, and wait on a blocked write — the client still writing. Official flag text points at that failure and at discussion #3997. A client that **flushes the batch then reads N replies** (instead of a million-command `--pipe` with no concurrent read) avoids that shape. ([flags](https://www.dragonflydb.io/docs/managing-dragonfly/flags), [.sources/flags-pipeline.md](.sources/flags-pipeline.md))

## Not atomic; mixed verbs; EVAL is still EVAL

Vendor write-up: pipelining is not atomic and has no isolation; commands in the batch may not all succeed together; MULTI/EXEC is the atomic wrapper (out of scope here). Dragonfly documents itself as a Redis drop-in for this pattern, with extra parallel execution of pipelined commands across threads. ([blog](https://www.dragonflydb.io/blog/batch-operations-in-dragonfly), [.sources/batch-operations.md](.sources/batch-operations.md))

No vendor page states that a `-ERR` on one pipelined command suppresses later replies. Follow Redis (`ext_redis_pipelining`): read all N replies; treat `-` as that element's error.

EVAL inside a pipeline is still one EVAL: Dragonfly still rejects undeclared keys and still runs Lua 5.4 (no `table.maxn`). That is `ext_dragonfly_eval`, not a pipeline exception. KEYS must be listed on each EVAL in the batch. Lua 5.1-safe scripts remain required for Redis+Dragonfly dual proof.
