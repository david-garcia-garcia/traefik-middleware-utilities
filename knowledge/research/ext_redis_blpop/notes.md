# Redis BLPOP holds one connection

Official Redis blocking list pop. Used here as the live I/O-timeout stall: an empty list plus a long server timeout holds **that client connection** until the client's `SetDeadline` fires. It does not freeze the rest of the server. ([redis.io BLPOP](https://redis.io/docs/latest/commands/blpop/), [.sources/blpop.md](.sources/blpop.md))

## Blocks the connection, not the process

`BLPOP key [key ...] timeout` pops from the first non-empty list. If none of the keys exist, Redis **blocks that connection** until another client pushes, or until `timeout` seconds elapse. Timeout `0` waits forever. A non-zero timeout that expires with no push returns a nil array. ([redis.io BLPOP — Blocking behavior](https://redis.io/docs/latest/commands/blpop/#blocking-behavior), [.sources/blpop.md](.sources/blpop.md))

Other clients on the same instance keep running. That is the property that makes this safe on CI Redis shared with `windowcounter` and `tokenbucket` (`go test ./...` runs packages in parallel; CI points both at `127.0.0.1:6379`).

## Do not stall via EVAL or CLIENT PAUSE

Lua scripts are atomic: **all server activities are blocked for the script's entire runtime**. A busy-loop `EVAL` would stall every other package's live tests on the shared instance. ([redis.io Scripting with Lua](https://redis.io/docs/latest/develop/programmability/eval-intro/), [.sources/eval-intro-atomic.md](.sources/eval-intro-atomic.md); existing `ext_redis_eval`)

`CLIENT PAUSE timeout-ms` suspends **all** clients for that many milliseconds (`ALL` is the default). Flags include `admin` and `noscript`. Wrong tool on a shared CI Redis. ([redis.io CLIENT PAUSE](https://redis.io/docs/latest/commands/client-pause/), [.sources/client-pause.md](.sources/client-pause.md))

`DEBUG` is an internal testing command (`noscript`, `@dangerous`). Redis Software/Cloud mark it unsupported. This ticket already forbids `DEBUG SLEEP`. ([redis.io DEBUG](https://redis.io/docs/latest/commands/debug/), [.sources/debug.md](.sources/debug.md))

## SimpleRedis proof shape

SimpleRedis has no exported `BLPOP`. Same-package live tests call unexported `exec` with `BLPOP`, a unique empty key, and a server timeout of several seconds, with `IoTimeout` tens of milliseconds. The client must return `redis:timeout` before the server unblocks. Do not add a public BLPOP verb. Do not `Eval` a BLPOP.
