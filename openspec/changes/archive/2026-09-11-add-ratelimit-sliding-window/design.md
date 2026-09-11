## Context

DestBranch (`origin/master`) already has `simpleredis/` with `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `Get`, `MGet`, `Close`, and `reclaim.Hooks` (`Sleep`, `Wake`, `Close`). There is no `windowcounter/` package. README still lists leaky `bucket/` as Planned. Compose Redis/Dragonfly have no host ports; CI `test` is `go test -v ./...` without engines. Kong facts: `knowledge/research/ext_kong_rate-limiting_sliding-sync/`. Explore decisions: `devstate/explore.md`. See proposal.md for why. Specs: `std_go_windowcounter_sliding-take`, `std_go_windowcounter_sync-flush`.

## Goals / Non-Goals

**Goals:**
- One Yaegi-safe `windowcounter/` package that injects SimpleRedis and exposes Take plus reclaim-shaped hooks.
- Prove window math on a fake TCP Redis and on live Redis + Dragonfly from `go test` (including Yaegi).
- Wire CI so live tests do not skip.

**Non-Goals:**
- Token bucket, leaky bucket, `bucket/`, fixed window, EVALSHA, `go-redis`.
- HTTP, 429, sleep/throttle, fail-open/fail-close, health gate.
- Pester/Traefik plugin.
- Importing `reclaim` from `windowcounter`.
- Closing the injected SimpleRedis from the limiter.

## Decisions

1. **Package `windowcounter/`.** Dest siblings are folder=package (`reclaim/`, `simpleredis/`). README planned `bucket/` is the wrong primitive. Alternative: implement under `bucket/` or `ratelimit/` — rejected; the job is a windowed hit counter, not Traefik RateLimit and not a leaky/token bucket.

2. **Inject `*simpleredis.SimpleRedis`.** `New(redis *simpleredis.SimpleRedis, syncRate time.Duration) (*Limiter, error)`. Two instances with two clients share Redis keys. Alternative: limiter owns `Init`/`Close` of Redis — rejected; SimpleRedis is already the session owner and tests need two clients.

3. **Opaque key + window-start suffix.** Redis keys `{opaqueKey}:{windowStartUnix}` and previous start. TTL `2*windowSec`. Caller prefixes. Alternative: copy Kong OSS `ratelimit:route:service:…` — rejected; prefixing is the caller's job.

4. **Increment then compare.** Take always counts the hit, then allows iff `estimated <= limit`. Matches Kong occupancy of denied hits.

5. **Exact vs buffered on one type.** `sync_rate==0`: `Incr` + `Expire` when result is 1, `Get` previous. `sync_rate>0`: local `redis_known`/`local_delta`, timer EVAL of the dest Kong snippet (`KEYS[1]` declared). Floor 20 ms. Negative `sync_rate` errors. Alternative: two types — rejected; one limiter is what a middleware `Open`s.

6. **Hooks on the limiter, not `reclaim.Table`.** Methods `Sleep`/`Wake`/`Close`. Sleep flushes then stops `time.NewTicker`. Wake starts the ticker. Close after Sleep does not close Redis. Alternative: limiter imports `reclaim` — rejected; the table stores the limiter; nested tables are the wrong owner.

7. **Test clock `SetNowForTest`.** Production uses `time.Now`. Boundary tests MUST NOT wait a real window. Name says test.

8. **Live addrs via env; CI service containers.** `WINDOWCOUNTER_LIVE_REDIS` / `WINDOWCOUNTER_LIVE_DRAGONFLY`. Skip on short or empty. CI `test` job: Redis 7 alpine + Dragonfly `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` as services, env set, no `-short`. Optional TestMain `docker run` when env empty and not short. Alternative: reuse Traefik compose — rejected; those services have no host ports and Pester is not the suite.

9. **Yaegi GOPATH copies both packages.** Compiled test starts/skips engines; interpreted probe calls `Take`. stdlib only, `useunsafe` false. Fake TCP lives in `windowcounter` tests (SimpleRedis helper is unexported).

## Risks / Trade-offs

- [Buffered clients undercount each other between flushes] → Mitigation: that is `sync_rate`; tests prove additive EVAL, not zero overshoot.
- [Dragonfly rejects undeclared EVAL keys] → Mitigation: `KEYS[1]` only; no `table.maxn`.
- [Live tests skip in CI] → Mitigation: service containers + env; no `-short` on the test job.
- [Flush goroutine survives Traefik reload] → Mitigation: Sleep/Wake/Close; no `time.Tick`; tests assert Close exits the goroutine.
- [Windows/local without Docker] → Mitigation: skip when addrs missing; CI still proves both engines.

## Migration Plan

New library. No production deploy. Rollback is revert the branch. Middleware authors switch from ad-hoc counters after this lands; not this change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
