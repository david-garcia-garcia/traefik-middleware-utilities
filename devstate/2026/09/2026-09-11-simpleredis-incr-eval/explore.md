# Explore
IssueKey: 2026-09-11-simpleredis-incr-eval

## Concepts

- **SimpleRedis** — existing Yaegi-safe stdlib RESP client on dest (`simpleredis/simpleredis.go`). Exported today: `Init`, `Get`, `MGet`, `Set` (SET+EX), `Del`, `Close`. No limiter, no `go-redis`.
- **exec / writeCommand / readReply** — the only wire path. New verbs are thin `exec` wrappers plus integer parse. Do not copy CrowdSec again; do not add a sibling `redis/` package.
- **Integer reply** — `readReply` already maps `:` to one `[]byte` of decimal digits (`simpleredis.go` `+`/`: ` branch). `Incr`/`IncrBy` parse that with `strconv.ParseInt`. Missing INCR key is not `redis:miss` (server creates `0` then increments).
- **Array reply** — `*` loop today requires `$` bulk (`readBulk` rejects other heads as `redis:issue?`). MGET on dest is the only live array consumer. EVAL `return {n}` or mixed tables need `$` / `:` / `+` elements; nested `*` stays out of scope.
- **Fake Redis** — `startFakeRedis` (`simpleredis_test.go`) understands AUTH/SELECT/GET/MGET/SET; unknown commands including DEL reply `+OK`. No integer replies. Unit tests extend this (and `startStaticRedis` for canned EVAL encode tests). No Lua VM, no miniredis.
- **Yaegi probe** — `yaegi_test.go` `clientprobe.RoundTrip` is Init/Set/Get/Del against the compiled fake. Traefik is not started there.
- **Traefik e2e** — `e2e/simpleredisprobe` SET+GET on every `/redis` request; compose has `redis:7-alpine` only; Pester Describe asserts `X-SimpleRedis-Value: ok`. Reclaim whoami `/a` `/b` must keep running.
- **Kong window dialect (consumer, not this change)** — exact path `INCR` then `EXPIRE` when result is 1; buffered flush is one EVAL of INCRBY + EXPIREAT-if-new. This ticket only grows the client so that library can speak those commands.

Gap measured (not a runtime crash): `go test ./simpleredis/...` passed (2026-09-11). Exported methods stop at `Del` (`simpleredis.go:88-129`). Probe and Pester never call Incr/Eval.

## Decisions

- Grow `SimpleRedis` with `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval` as in the requirement. Reuse `exec`. Internal `parseIntegerReply` for `:` payloads; garbage → `redis:issue?`. `Expire`/`ExpireAt` treat `:0` and `:1` as success (`nil`).
- Extend `readReply` `*` so each element may be `$`, `:`, or `+` (null bulk stays a nil slot). Nested `*` or `-` → `redis:issue?` and `reusable false` when the rest of the stream cannot be skipped. Do not change MGET’s call site; Redis MGET still returns only bulks.
- Fake: INCR/INCRBY on the existing `map[string]string` digit store, reply `:<n>`. EXPIRE/EXPIREAT record `lastExpire` and reply `:1` (TTL may be a no-op). EVAL matches the Kong incrby+expireat script string used by tests; otherwise use `startStaticRedis` canned replies. Unknown commands must not stay `+OK` for INCR (that would hide a missing handler).
- Spec host is existing `std_go_simpleredis_resp-commands`. Add INCR/EXPIRE/EVAL requirements; do not weaken GET/MGET/SET/DEL. Yaegi requirement grows to Incr+Eval. Traefik e2e requirement grows to both backends.
- E2E (human addendum, not the handoff’s optional line): one compose stack, two live engines. Do not rename `reclaim-e2e` (existing debt `knowledge/debt/2026-09-11-rename-reclaim-e2e-compose.md`).
- Probe stays one plugin. `Config.Host` already selects the engine. ServeHTTP runs every SimpleRedis verb (existing Get/MGet/Set/Del plus the new five) on per-request keys and sets one response header per verb. EVAL body is the Kong incrby+expireat snippet with `KEYS[1]` declared (Dragonfly requires declared keys).
- No rate limiter, EVALSHA, pipeline, TLS, Unix sockets, pool-size change, or EXISTS Go method.

## Open questions

- Q: Which Dragonfly image and tag does compose pin?
  Rank: additive asked — Desired E2E addendum names a Dragonfly backend; this change creates the service
  Decision: resolved — `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`; hostname `dragonfly`, internal 6379, no password; `ulimits.memlock: -1`; no host-port publish (same as redis); do not use Docker Hub `dragonflydb/dragonfly`
  By: propose

- Q: How does the same Pester Describe prove Redis and Dragonfly without breaking reclaim e2e?
  Rank: additive asked — Desired E2E addendum; new whoami route this change creates; existing `/redis` and `/a` `/b` stay
  Decision: resolved — one reclaim-e2e compose; dragonfly + `/dragonfly`; keep `/redis` and `/a` `/b`; SimpleRedis Describe does not stop whoami-a/b
  By: implement

- Q: What probe shape proves every SimpleRedis interaction under Yaegi?
  Rank: additive asked — Desired E2E plus Affected `e2e/simpleredisprobe/`
  Decision: resolved — one ServeHTTP runs Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval; unique keys per request; one header per verb; Eval uses Kong KEYS snippet
  By: implement

- Q: Does the Kong EVAL script return a top-level integer, so array parsing is only for other Eval callers?
  Rank: additive asked — Desired parser change; criterion names `$`/`: `/`+` array elements
  Decision: resolved — `*` accepts `$`/`: `/`+`; nested `*` is `redis:issue?` with reusable false; e2e Eval returns integer `3`
  By: implement
