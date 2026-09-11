# Explore
IssueKey: 2026-09-11-kong-window-limiter

## Concepts

- **Windowed hit counter** — Redis integers per window, not a token/leaky bucket. Local memory is only the `sync_rate>0` buffer. Package `windowcounter/` (new). `windowcounter/` is not on dest (`master` HEAD); README still lists leaky `bucket/` as planned (`README.md`).
- **Sliding estimate** — `estimated = current + previous × (1 − elapsed/window)` ([Kong window types](https://developer.konghq.com/gateway/rate-limiting/window-types); ticket lock). At a new-window boundary, elapsed≈0 so previous still counts in full — that is what stops the fixed-window 2× dump. Fixed window is out of v1.
- **Window keys** — two Redis integers for one opaque caller key: `{opaqueKey}:{windowStartUnix}` and the previous start (`windowStart − windowSec`). OSS Kong embeds route/service/period (`get_local_key` in [policies/init.lua](https://github.com/Kong/kong/blob/master/kong/plugins/rate-limiting/policies/init.lua)); that prefixing is the caller’s job here. TTL is **two** window lengths so the previous key survives. See `knowledge/research/ext_kong_rate-limiting_sliding-sync/`.
- **SimpleRedis** — dest already has `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `Get`, `MGet`, `Close` (`simpleredis/simpleredis.go`). Missing keys on `Get` are `redis:miss`. Errors stay text (`redis:unreachable`, `redis:timeout`). No `go-redis`. No GET/SET race for the counter itself.
- **Exact vs buffered** — `sync_rate=0`: `Incr` every `Take`, `Expire` when the result is 1. `sync_rate>0`: admit from `redis_known + local_delta`; timer `Eval` of the Kong INCRBY + EXPIREAT-if-new script (`KEYS` declared; no `table.maxn`). Floor 20 ms ([Kong Advanced schema](https://developer.konghq.com/plugins/rate-limiting-advanced)). After flush, `redis_known` becomes the EVAL return (global count) and `local_delta` clears — that is how two clients share a limit without last-write-wins.
- **Reclaim hooks** — `reclaim.Hooks` is Sleep/Wake/Close on a stored value (`reclaim/table.go`, `knowledge/devdocs/std_go_reclaim.md`). The limiter is a value a middleware `Open`s; it does not import `reclaim.Table`.
- **Identity** — opaque key only. This library does not read HTTP, client address, user, tenant, or Host. The middleware that calls `Take` already owns that fact.
- **Tests** — unit: in-package fake TCP (SimpleRedis `startFakeRedis` style; that helper is unexported in `simpleredis_test.go`, so windowcounter tests own a copy). Live: `Init` + `Take` on real Redis and Dragonfly. Yaegi: compiled test owns start/skip; interpreted probe calls `Take`; GOPATH copy of non-test sources; stdlib only, `useunsafe` false. Pester/Traefik plugin is optional extra, not the behaviour suite.
- **CI gap** — `.github/workflows/ci.yml` `test` job is `go test -v ./...` with no Redis/Dragonfly. Compose `redis`/`dragonfly` have no host ports (`docker-compose.yml`). Live tests must not depend on the Traefik Pester stack.

```
Take(key, limit, window)
        │
        ├─ sync_rate == 0 ── Incr(current) ─ Expire if 1 ─ Get(previous)
        │                         │
        │                         └─ estimated = cur + prev*(1-elapsed/window)
        │
        └─ sync_rate > 0 ── estimated from redis_known+local_delta (+ previous)
                              │
                              ├─ admit: bump local_delta
                              └─ timer Eval INCRBY + EXPIREAT-if-new
```

Gap measured: no `windowcounter/` tree on this worktree; README library row is still leaky bucket.

## Decisions

- New package `windowcounter/`. Caller injects `*simpleredis.SimpleRedis`. `New(redis, syncRate)`. `Take(key, limit, window)` returns `allowed bool, estimated float64, err`. `Allow` is the same method (alias). Prefixing is the caller’s job. No HTTP, 429, or sleep.
- Sliding only. Window length is a `time.Duration` with whole-second Redis TTL (`Expire`/`ExpireAt` are integer seconds). Sub-second windows are out of v1. Injectable `nowForTest` (`SetNowForTest`) for boundary math; production uses `time.Now`.
- Exact: `Incr` current, `Expire(current, 2*windowSec)` when result is 1, `Get` previous (`redis:miss` → 0). Increment first, then allow iff `estimated <= limit`. Denied hits still occupy the window (Kong).
- Buffered: per-key `redis_known` + `local_delta` (current) and a stored previous count. Flush EVAL matches the dest SimpleRedis Kong snippet (`exists` / `incrby` / `expireat` if new; `KEYS[1]`). `sync_rate < 0` fails `New`. `0 < sync_rate < 20ms` floors to 20ms.
- Limiter methods `Sleep` / `Wake` / `Close` for `reclaim.Hooks`. Sleep: flush pending deltas then stop the ticker. Wake: start the ticker. Close: after Sleep (reclaim always Sleeps first); do not `Close` the injected SimpleRedis (shared client). `sync_rate=0`: hooks are no-ops besides stopping any leftover work. Use `time.NewTicker` + stop channel, not `time.Tick`.
- Redis errors propagate. No fail-open, fail-close, or health gate.
- README library row and layout become `windowcounter/` (this primitive). Do not add `bucket/`.
- Live tests: env `WINDOWCOUNTER_LIVE_REDIS` and `WINDOWCOUNTER_LIVE_DRAGONFLY` (`host:port`). `testing.Short` or missing addrs → skip. CI `test` job starts both engines (GitHub Actions service containers; Dragonfly image already pinned in compose: `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`) and sets those env vars so the suite does **not** skip. Table-driven backend addr. When env is unset and not `-short`, compiled `TestMain` may `docker run` and skip if Docker is absent. Do not publish compose ports or reuse the Traefik integration job as the limiter suite.
- Yaegi live: same scenarios; compiled test starts/skips engines; probe in GOPATH calls `Take`. Copy `windowcounter` and `simpleredis` non-test sources.
- No Pester/Traefik plugin in this change (optional extra; bound the ask).
- EVALSHA later. No `go-redis`. Usage packet `knowledge/devdocs/std_go_windowcounter.md` in propose/implement (no API on dest to document yet). Spec host: new `std_go_windowcounter_*` leaves under `std` / `go` (`openspec/specs/map.md`).

## Open questions

- Q: Who owns the rate-limit identity (client address, user, tenant, Host)?
  Rank: additive asked — Desired names an opaque key and forbids HTTP inside the library
  Decision: resolved — the caller. `Take` receives the opaque key; the library does not read the request or reconstruct a client address.
  By: explore

- Q: Exact Redis key scheme for current vs previous window counters?
  Rank: additive asked — Desired sliding formula needs two counters; this change creates the keys
  Decision: assumed — `{opaqueKey}:{windowStartUnix}` and `{opaqueKey}:{windowStartUnix-windowSec}`; TTL `2*windowSec`. Caller prefixes. Not OSS `ratelimit:route:service:…`.
  By: explore

- Q: Does the flush timer register through `reclaim.Table` or a standalone Sleep/Wake/Close on the limiter?
  Rank: additive asked — Desired names reclaim-style Sleep/Wake/Close
  Decision: assumed — methods on the limiter; caller passes `reclaim.Hooks{Sleep: lim.Sleep, Wake: lim.Wake, Close: lim.Close}`. Limiter does not import `reclaim`.
  By: explore

- Q: How do live tests discover Redis/Dragonfly addrs in CI while keeping `-short` skip locally?
  Rank: additive asked — Desired live suite + CI must not skip
  Decision: assumed — `WINDOWCOUNTER_LIVE_REDIS` / `WINDOWCOUNTER_LIVE_DRAGONFLY`; skip on `testing.Short` or empty; CI `test` job service containers set both. Optional `TestMain` `docker run` when env empty and not short.
  By: explore

- Q: Does one limiter instance own a SimpleRedis client, or do callers inject shared clients?
  Rank: additive asked — Desired two-client `sync_rate>0` share without last-write-wins
  Decision: assumed — inject `*simpleredis.SimpleRedis`. Two limiter instances with two clients hit the same Redis keys. Limiter.Close does not close the client.
  By: explore

- Q: Do denied Takes still increment the counter?
  Rank: additive asked — Desired “N Takes then deny”
  Decision: assumed — increment first (Incr or local_delta), then allow iff estimated <= limit. The Nth admit has estimated N; N+1 is denied and still counted.
  By: explore

- Q: What does `usage` in the Take/Allow return mean?
  Rank: additive asked — Desired “allowed + usage”
  Decision: assumed — `estimated float64` after this Take (sliding formula). Not remaining, not integer-ceiled.
  By: explore

- Q: May exact mode `Get` the previous window even though the ticket listed Eval/Incr/Expire only?
  Rank: additive asked — Desired sliding formula needs previous; dest already has Get
  Decision: assumed — `Get`/`MGet` for previous (miss=0). Counter updates stay Incr/Eval. Not a GET/SET race.
  By: explore

- Q: Ship a Pester/Traefik plugin for the limiter in this change?
  Rank: additive incidental — ticket marks it optional extra, not a substitute
  Decision: assumed — skip. Direct `go test` live + Yaegi live are the proof.
  By: explore
