# Handoff: SimpleRedis INCR / EXPIRE / EVAL

Copy this whole file to the implementation agent. This is **not** the Kong window-counter library. It only grows the existing Yaegi-safe Redis client so a later limiter can speak Kong’s Redis dialect.

Do not implement a rate limiter, leaky bucket, token bucket, or sync_rate flusher in this change.

## Why

A later library will copy **Kong’s Redis rate-limit store**: windowed integer counters, local deltas, flush with `INCRBY` + set TTL only when the key is new. Exact mode (`sync_rate = 0`) is one `INCR` per hit and `EXPIRE` when the result is `1`.

CrowdSec SimpleRedis (what we copied) only speaks GET, MGET, SET+EX, DEL. That cannot implement the limiter. Dragonfly and Redis both fully support the commands below; the gap is this client.

---

## What the client has today

Exported:

| Method | Wire |
|---|---|
| `Init(host, pass, database string)` | no dial |
| `Get` | `GET` |
| `MGet` | `MGET` |
| `Set` | `SET key data EX seconds` |
| `Del` | `DEL` |
| `Close` | drain pool |

`exec` → `writeCommand` (RESP array of bulk strings) → `readReply`.

`readReply` today:

- `+` / `:` → one `[]byte` (payload after the type byte). Integers already parse as decimal digits.
- `-` → `replyError`
- `$` → bulk or `redis:miss`
- `*` → **each element must be `$` bulk**. Integer (`:`) or status (`+`) elements return `redis:issue?`. Nested arrays are not parsed.

Pool, AUTH, SELECT, errors (`redis:unreachable`, `redis:miss`, `redis:timeout`, `redis:noauth`, `redis:issue?`) stay as they are.

`startFakeRedis` understands AUTH, SELECT, GET, MGET, SET. Unknown commands reply `+OK` (including DEL). It has **no** INCR/EXPIRE/EVAL and **no** integer replies.

---

## What Kong actually sends (the consumer)

Exact path (one request):

```
INCR   <window-key>
EXPIRE <window-key> <ttl-seconds>     # only when INCR returns 1
```

Kong OSS wraps the first hit as Lua `INCR` + `EXPIRE` if `result_incr == 1`, and returns `result_incr - 1` as the pre-increment usage.

Buffered flush (timer, many keys):

```lua
-- KEYS[1] = key, ARGV[1] = delta (integer), ARGV[2] = expire-at unix seconds
local exists = redis.call("exists", KEYS[1])
redis.call("incrby", KEYS[1], ARGV[1])
if exists == 0 then
  redis.call("expireat", KEYS[1], ARGV[2])
end
```

Wire: `EVAL <script> 1 <key> <delta> <unix>`. One key, declared in `KEYS` (required on Dragonfly). Script must be Lua 5.1-safe (Dragonfly is 5.4: **no `table.maxn`**). This script is fine.

`INCR` / `INCRBY` do **not** refresh TTL. Conditional expire must be atomic with the increment → **EVAL**, not INCR then EXPIRE as two client commands.

---

## Gaps to close (this change)

### Must

1. **`Eval(script string, keys []string, args []string) ([][]byte, error)`**
   - Send `EVAL` `script` `numkeys` then keys then args. `numkeys` is the decimal count as a bulk string (`strconv.Itoa(len(keys))`).
   - Empty `keys` and empty `args` are legal (`EVAL "return 1" 0`).
   - Return the same `[][]byte` `exec` already uses: an integer reply is one element of decimal digits (existing `:` branch).
   - Script body may contain newlines; `writeCommand` already bulk-encodes that (SET tests).
   - Redis `-` errors stay `replyError` (Lua `ERR Error running script ...` is not AUTH-class).
   - Do not add `EVALSHA` / `SCRIPT LOAD` in this change.

2. **`Incr(name string) (int64, error)`** and **`IncrBy(name string, delta int64) (int64, error)`**
   - `INCR key` / `INCRBY key <delta>`.
   - Parse the `:` integer with `strconv.ParseInt`. Garbage payload → `redis:issue?`.
   - Missing key: Redis/Dragonfly create `0` then increment (first `INCR` returns `1`). Not a miss.
   - `IncrBy` with `delta == 0` still sends the command (Redis allows it).

3. **`Expire(name string, seconds int64) error`** and **`ExpireAt(name string, unixSeconds int64) error`**
   - `EXPIRE key seconds` / `EXPIREAT key unix`.
   - Integer reply `:0` or `:1` is success. Do not treat `:0` (key missing) as `redis:miss` unless you have a spec reason; Kong ignores the expire result. Returning `nil` on both 0 and 1 is enough.
   - Negative / zero seconds: send what the caller passed; do not invent clamping (Redis deletes the key; that is the server’s job).

### Parser (must, small)

`readReply` array (`*`) only accepts `$` elements. A Lua `return {n}` or mixed EVAL table will fail.

For this limiter, EVAL’s last `redis.call` is an integer, so Redis returns a top-level `:`. That already works.

Still: if `Eval` is documented as “same as exec”, extend the `*` loop so each element may be `$`, `:`, or `+` (store payload bytes; null bulk `$ -1` stays a nil slot like MGET). Nested `*` is out of scope; if the element head is `*` or `-`, return `redis:issue?` without desyncing the stream if you cannot skip — prefer failing clean (`reusable false`) on unknown nested types.

Do not break MGET: MGET elements are only bulks.

### Must not (this change)

| Out | Why |
|---|---|
| Pipeline / MULTI / EXEC | Kong pipelines many EVALs; v1 limiter can `Eval` in a loop. Later. |
| `EVALSHA` | Optimization. |
| `EXISTS` as a Go method | Only inside Lua. |
| `GET` of counters | Exact path uses INCR, not GET. GET already exists. |
| TLS, Unix sockets, `go-redis`, miniredis | Unchanged from SimpleRedis. |
| Raising `ioTimeout` / `maxIdleConns` | Not required for tiny scripts. |
| Window counter, sync_rate, reclaim timer | Next library. |
| Traefik token-bucket Lua / `HGETALL` / `HSET` | Not this consumer. |
| Changing Init/Close/pool | Close comment may list the new methods; behaviour unchanged. |

---

## Suggested surface (Yaegi-friendly)

Concrete types, no interfaces unless tests need a fake. Match existing names:

```go
func (sr *SimpleRedis) Incr(name string) (int64, error)
func (sr *SimpleRedis) IncrBy(name string, delta int64) (int64, error)
func (sr *SimpleRedis) Expire(name string, seconds int64) error
func (sr *SimpleRedis) ExpireAt(name string, unixSeconds int64) error
func (sr *SimpleRedis) Eval(script string, keys []string, args []string) ([][]byte, error)
```

`Eval` keys/args as `[]string` (not `[][]byte`) keeps call sites readable; convert to `[]byte` at `exec`. An `int64` helper that parses a single `:` reply from `exec` is internal (`parseIntegerReply`) — tests-only names if it is only used from tests.

`Close` / package comment: include the new verbs.

---

## Tests

Extend **`startFakeRedis`** (do not add miniredis):

- `INCR` / `INCRBY`: integer store (string digits in the existing `map[string]string` is fine). Reply `:<n>\r\n`. Missing key starts at 0.
- `EXPIRE` / `EXPIREAT`: may be no-op TTL in the fake (record last command like SET) and reply `:1`.
- `EVAL`: **do not** embed a Lua VM. For unit tests, either:
  - `startStaticRedis` with a canned `:` reply for Eval tests that only care about the client encoder, and/or
  - fake recognizes a **fixed** script string used by tests (the Kong incrby+expireat snippet) and applies INCRBY + optional EXPIREAT on `KEYS[1]`, then replies with the new integer.

Prove:

- `Incr` missing key → 1; second `Incr` → 2
- `IncrBy(key, 5)` missing → 5
- `Incr` of a non-integer value → Redis would `-ERR`; fake can send `-ERR value is not an integer` → caller gets that text, not `redis:issue?` unless parse fails
- `Expire` / `ExpireAt` send the right argv (assert `lastExpire` like `lastSet`)
- `Eval` writes `EVAL`, script, `1`, key, delta, unix (assert captured argv)
- Eval integer reply parsed as one `[][]byte` element `"7"`
- Existing Get/Set/Del/MGet tests still pass
- Yaegi: interpreted `Incr` + `Eval` against the compiled fake (extend `yaegi_test.go` / `clientprobe`, stdlib only, `useunsafe` false, do not start Traefik)

Pester/compose Redis: optional one extra probe header or keep out of this change if e2e only SET/GET. Prefer a small compiled-test + Yaegi coverage; do not require a new Traefik plugin unless the change already touches `e2e/simpleredisprobe`.

`go test ./simpleredis/...` must pass. Then Yaegi tests in that package.

---

## Spec deltas (`std_go_simpleredis_resp-commands`)

ADDED:

- `Incr` / `IncrBy` send those commands and return the integer; missing key is not `redis:miss`.
- `Expire` / `ExpireAt` send those commands; integer 0 or 1 is success.
- `Eval` sends `EVAL` with `numkeys` = `len(keys)`; integer or bulk reply is returned as `[][]byte`; Lua/server error lines are returned as errors (AUTH-class still `redis:noauth`).
- Interpreter tests prove `Incr` and `Eval` under Yaegi v0.16.1 against a compiled fake, no Traefik.

Do not weaken Get/Set/Del scenarios.

---

## Consume before produce

Reuse `exec`, `writeCommand`, `readReply`, `borrow`/`release`, `ioError`, `replyError`. New methods are thin `exec` wrappers plus integer parse. Do not copy the CrowdSec client again. Do not start a `redis/` package beside `simpleredis/`.

---

## Done when

- Specs updated and the change validates.
- `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval` exist and are tested (compiled + Yaegi).
- GET/MGET/SET/DEL unchanged.
- No limiter package, no `go-redis`, no EVALSHA, no pipeline.
- A later agent can implement Kong’s two Lua/INCR paths using only this API against Redis or Dragonfly.

---

## Human addendum (required)

Make sure we have E2E tests that probe that all the redis interactions work with BOTH redis and dragonfly.

This addendum is part of Desired. The handoff said Pester/compose Redis was optional; the human now requires E2E covering every SimpleRedis interaction against both Redis and Dragonfly.

Do not implement a rate limiter. Do not add EVALSHA, pipeline, go-redis, miniredis.
