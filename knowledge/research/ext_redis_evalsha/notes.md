# Redis EVALSHA and NOSCRIPT

Official Redis execute-by-digest path and the miss error clients match. Default engine is still **Lua 5.1**. Digest is **SHA-1 of the script bytes**, lowercase hex, 40 characters. ([redis.io EVALSHA](https://redis.io/docs/latest/commands/evalsha/), [.sources/evalsha-command.md](.sources/evalsha-command.md); [redis.io eval-intro — Script cache](https://redis.io/docs/latest/develop/programmability/eval-intro/), [.sources/eval-intro-evalsha.md](.sources/eval-intro-evalsha.md); [redis@7.2.4:src/eval.c `sha1hex`](https://github.com/redis/redis/blob/7.2.4/src/eval.c), [.sources/eval-c-sha1hex.md](.sources/eval-c-sha1hex.md))

## Command shape

```
EVALSHA sha1 numkeys [key [key ...]] [arg [arg ...]]
```

Otherwise identical to `EVAL`. `numkeys` may be `0`. Keys populate Lua `KEYS`; remaining tokens populate `ARGV`. ([EVALSHA](https://redis.io/docs/latest/commands/evalsha/), [.sources/evalsha-command.md](.sources/evalsha-command.md))

`EVAL` itself stores the body in the same cache, keyed by that SHA-1. `SCRIPT LOAD` loads without running. ([eval-intro](https://redis.io/docs/latest/develop/programmability/eval-intro/), [.sources/eval-intro-evalsha.md](.sources/eval-intro-evalsha.md))

## Cache is volatile

The script cache is not part of the keyspace and is not persisted. It is cleared on restart, replica failover, or `SCRIPT FLUSH`. Clients must tolerate a miss at any time. ([eval-intro — Cache volatility](https://redis.io/docs/latest/develop/programmability/eval-intro/), [.sources/eval-intro-evalsha.md](.sources/eval-intro-evalsha.md))

`SCRIPT EXISTS sha` returns `1` when that digest is in the cache, `0` after flush or if never loaded. Official docs name `SCRIPT FLUSH` as the way to empty the cache for client-library tests. ([eval-intro — The SCRIPT command](https://redis.io/docs/latest/develop/programmability/eval-intro/), [.sources/eval-intro-evalsha.md](.sources/eval-intro-evalsha.md))

## NOSCRIPT wire text

On a missing digest, Redis returns an error reply whose payload **starts with `NOSCRIPT`**.

**Conflict (docs vs implementation):** current eval-intro example is `(error) NOSCRIPT No matching script` with no trailer. Redis 6/7 `redis-cli` and `shared.noscripterr` emit `NOSCRIPT No matching script. Please use EVAL.` ([eval-intro](https://redis.io/docs/latest/develop/programmability/eval-intro/), [.sources/eval-intro-evalsha.md](.sources/eval-intro-evalsha.md); [redis/redis#9286](https://github.com/redis/redis/issues/9286), [.sources/issue-9286-noscript.md](.sources/issue-9286-noscript.md); [redis@7.2.4:src/eval.c `addReplyErrorObject(c, shared.noscripterr)`](https://github.com/redis/redis/blob/7.2.4/src/eval.c), [.sources/eval-c-sha1hex.md](.sources/eval-c-sha1hex.md))

Follow **source** for what Redis 7 sends. A client `strings.HasPrefix(err, "NOSCRIPT")` matches both the short docs example and the `Please use EVAL.` trailer. Scripting tests assert `NOSCRIPT*`. ([redis tests/unit/scripting.tcl](https://github.com/redis/redis/blob/7.2.4/tests/unit/scripting.tcl), [.sources/scripting-tcl.md](.sources/scripting-tcl.md))

`EVALSHA` with a SHA that is not 40 hex characters also returns `shared.noscripterr` (same prefix). ([eval.c `evalShaCommand`](https://github.com/redis/redis/blob/7.2.4/src/eval.c), [.sources/eval-c-sha1hex.md](.sources/eval-c-sha1hex.md))

This is not the Lua `ERR Error running script…` family. Do not parse NOSCRIPT as a Lua runtime error.

## Client fallback (pattern, not a Redis command)

Official guidance: on NOSCRIPT, supply the body (`SCRIPT LOAD` then `EVALSHA`, or `EVAL` which also fills the cache). Pipelined `EVALSHA` cannot recover a miss between commands; this product does not pipeline Eval. ([eval-intro](https://redis.io/docs/latest/develop/programmability/eval-intro/), [.sources/eval-intro-evalsha.md](.sources/eval-intro-evalsha.md))

go-redis v9.21.0 `NewScript` SHA-1s the source with `crypto/sha1` + `encoding/hex`. `Script.Run` sends `EVALSHA` then `EVAL` when the error is `ErrNoScript` / prefix `NOSCRIPT`. ([go-redis@v9.21.0:script.go](https://github.com/redis/go-redis/blob/v9.21.0/script.go), [.sources/go-redis-script.go.md](.sources/go-redis-script.go.md))
