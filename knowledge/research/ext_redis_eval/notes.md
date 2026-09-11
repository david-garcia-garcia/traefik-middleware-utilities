# Redis EVAL wire behavior

Official Redis server-side Lua scripting via `EVAL`. Default engine is **Lua 5.1**. ([redis.io Scripting with Lua](https://redis.io/docs/latest/develop/programmability/eval-intro/), [.sources/eval-intro.md](.sources/eval-intro.md))

## Command shape

```
EVAL script numkeys [key [key ...]] [arg [arg ...]]
```

- Arg 1: Lua source (string bulk).
- Arg 2: `numkeys` — count of following key-name arguments.
- Next `numkeys` tokens populate the Lua global `KEYS`.
- Remaining tokens populate `ARGV`.

([redis.io EVAL](https://redis.io/docs/latest/commands/eval/), [.sources/eval-command.md](.sources/eval-command.md))

## Zero keys is legal

`numkeys` may be `0`. Keys are optional when the script uses only `ARGV`. Official example: `EVAL "return ARGV[1]" 0 hello` → bulk `"hello"`. ([redis.io EVAL — Examples](https://redis.io/docs/latest/commands/eval/#examples), [.sources/eval-command.md](.sources/eval-command.md))

All keys a script accesses must be listed in the `KEYS` arguments (cluster and correctness rule). ([redis.io EVAL](https://redis.io/docs/latest/commands/eval/), [.sources/eval-command.md](.sources/eval-command.md))

## Return type → RESP reply (default RESP2)

Script return values map to RESP2 as follows ([redis.io Lua API — Lua to RESP2 type conversion](https://redis.io/docs/latest/develop/programmability/lua-api/#lua-to-resp2-type-conversion), [.sources/lua-api-resp2.md](.sources/lua-api-resp2.md)):

| Lua return | Wire reply |
|------------|------------|
| number | integer (`:`), decimal part truncated |
| string | bulk string (`$`) |
| indexed array table | array (`*`), truncated at first `nil` element |
| `{ ok = "..." }` | status (`+`) |
| `{ err = "..." }` | error (`-`) |
| `false` | null bulk |
| `true` | integer `1` |

Examples: `EVAL "return 10" 0` → `(integer) 10`; nested tables → nested arrays; `return redis.call('get','foo')` preserves the underlying command reply type.

## Lua / redis.call errors → `-ERR Error running script…`

When `redis.call()` raises (syntax error, command error, etc.), Redis returns an **error reply** to the client. Historical and documented form:

```
-ERR Error running script (call to <sha>): <details>
```

Example from Redis scripting release notes: `EVAL "return {ok=os.currentdir()}" 0` → `(error) ERR Error running script (call to f_…): [string "func definition"]:2: attempt to call field 'currentdir' (a nil value)`. ([antirez scripting branch release](http://oldblog.antirez.com/post/scripting-branch-released.html), [.sources/scripting-branch-released.md](.sources/scripting-branch-released.md))

**Conflict:** Redis 7.x reshuffled script error strings (error code first, trailing `Error running script` suffix) per [redis/redis#10329](https://github.com/redis/redis/pull/10329). Implementers parsing `-ERR Error running script` should not assume identical text across Redis minor versions. Dragonfly still emits the classic prefix (see `ext_dragonfly_eval`).
