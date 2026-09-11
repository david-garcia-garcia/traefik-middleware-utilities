# Explore
IssueKey: 2026-09-11-leaky-bucket

## Concepts

Need: a fill clock. Dest already has Traefik refill (`tokenbucket/`) and Kong sliding hits (`windowcounter/`). README tells callers not to mix those two. There is no water meter.

```
  pour n ──► water ── leak×Δt ──► 0
                 │
                 └── if water+n > capacity → deny, do not pour
```

Reproduced (absence): `leakybucket/` is not in the worktree; `go list ./leakybucket` fails; README library table and layout have only reclaim, SimpleRedis, windowcounter, tokenbucket.

| Clock | Package | Stored | Admit |
|-------|---------|--------|-------|
| Traefik refill | `tokenbucket/` | `{last, tokens}` hash, snapshot HSET every Allow | idle fills to burst |
| Kong sliding | `windowcounter/` | window integers, INCR / INCRBY | hits in a window |
| Classic meter | **missing** | water + last | pour until cap, then wait for leak |

ITU-T I.371 Annex A.2 and Wikipedia’s **meter** (not the FIFO queue) match the ticket formula: drain first, refuse the pour that would overflow, leave water unchanged (`knowledge/research/ext_leaky-bucket_meter/`). `tokens ≈ capacity − water` is remaining room, not what we store.

Kong `sync_rate` + OSS INCRBY timer is the **flush transport** only (`ext_kong_rate-limiting_sliding-sync`). Steal delta + timer. Do not steal window keys or sliding weights. Traefik Lua `HSET last,tokens` every Allow (`tokenbucket/lua.go`) is the pattern the ticket forbids for replicas: last-write-wins / double-leak.

Memory is a full exact backend (`tokenbucket/memory.go` mutex map), not only Kong’s `local_delta` buffer. When `sync_rate>0`, `local_pours` is that buffer on the Redis limiter.

Identity: this library does not read client address, user, tenant, or Host. Opaque key. Same as both siblings.

Usage packets exist for the siblings (`std_go_tokenbucket.md`, `std_go_windowcounter.md`). No `std_go_leakybucket.md` yet — produce after the package exists (propose / devdocsimpact). Language terms below are taken from those packets plus `ext_leaky-bucket_meter`; they are not invented.

## Decisions

- **Package:** `leakybucket/` (`package leakybucket`). Import `github.com/david-garcia-garcia/traefik-middleware-utilities/leakybucket`. Do not fold into `tokenbucket/` or `windowcounter/`. Do not invert `tokenbucket/lua.go`.
- **Clock:** `water(t) = max(0, water(t0) + poured − leak×Δt)`, then refuse if `water+n > capacity` (do not pour). Empty = water 0 = room up to `capacity`. Unix microseconds for `last` and elapsed, same as `tokenbucket/clock.go`, so Go and Lua `tonumber` agree. Leak is water per second (float64). Capacity is float64. Pour `n` is int64 (≥ 1).
- **API:** `Add(key, n)` pours `n`. `Take(key)` is `Add(key, 1)`. `Level(key)` applies leak and returns water; it does not pour. `Add`/`Take` return `allowed`, `level`, `untilNotFull` (duration until water would be below capacity enough for one more unit; 0 if already not full). No HTTP, 429, fail-open/fail-close, health-gate, or `time.Sleep`.
- **Constructors:** `NewMemory(leak, capacity, ttl)` and `NewRedis(client, leak, capacity, syncRate, ttl)`. `ttl` min 1s (lazy expire in memory; Redis `EXPIRE`). Negative `sync_rate` fails. Positive below 20 ms floors to 20 ms (`windowcounter` / Kong Advanced). `sync_rate=0` is exact EVAL every pour (and Level with delta 0).
- **Redis encoding:** one key, HASH fields `water` and `last`. EVAL: HGETALL, leak once with ARGV now, add delta, cap, HSET, EXPIRE. KEYS lists the key. No `table.maxn`. Go never `SET`/`HSET` that hash from a replica. `simpleredis.Eval` only. No `EVALSHA`. Errors stay `redis:unreachable` / `redis:timeout`.
- **Buffered admit:** per key `redisWater`, `lastSync`, `localPours`. Admit from `leaked(redisWater, lastSync, now) + localPours`. Timer EVAL leak once then add `localPours`. After success, local state becomes EVAL’s `{water, last}` and `localPours=0`. Over-allow is at most one sync interval of pours per replica, not unbounded.
- **Reclaim:** Redis limiter exports `Sleep`/`Wake`/`Close` like `windowcounter.Limiter` (flush then stop ticker; Wake restarts when `sync_rate>0`; Close does not close SimpleRedis). Memory has no ticker and does not export those hooks.
- **Tests:** same bar as `tokenbucket/` / `windowcounter/`: fake TCP + in-memory unit; live Redis+Dragonfly table-driven addrs; Yaegi GOPATH interp (`useunsafe` false) calling `Take`. Env `LEAKYBUCKET_LIVE_REDIS` / `LEAKYBUCKET_LIVE_DRAGONFLY`. CI already starts `:6379` and `:6380`; add those env vars. Pester optional, not a substitute.
- **OpenSpec (propose):** new family `std` / `go` / `leakybucket` — pour clock vs Redis sync-flush. Change kebab `add-leakybucket`. Do not edit sibling specs.

## Open questions

- Q: How is `{water, last}` encoded in Redis so EVAL can leak once then add a delta without a replica SET?
  Rank: additive asked — new key this change creates; Desired names EVAL leak-then-add, EXPIRE, never SET a replica blob
  Decision: assumed — HASH fields `water` (float string) and `last` (unix microseconds) on one KEYS[1]; EVAL HGETALL / leak / add / HSET / EXPIRE; Go never writes the hash except through that script.
  By: explore

- Q: Is time-until-not-full a required return on Add/Take or omitted?
  Rank: additive asked — Desired says optional on the new API
  Decision: assumed — always return it as the third value (duration until there is room for one more unit; 0 if already not full). Level does not return it.
  By: explore

- Q: Are Add and Take aliases, or is Take n=1 only?
  Rank: additive asked — Desired: Take/Add pours n default 1; Go has no default args
  Decision: assumed — `Add(key, n int64)` pours n (`n < 1` errors); `Take(key)` is Add(key, 1).
  By: explore

- Q: What units make in-memory and Redis agree on admit/deny for the same leak/capacity sequence?
  Rank: additive asked — Desired same leak/capacity on both stores
  Decision: assumed — leak float64 water/second, capacity float64, n int64, elapsed unix microseconds, Lua `tonumber` on the same ARGV as Go.
  By: explore

- Q: What live env var names, and does CI reuse the existing Redis/Dragonfly services?
  Rank: additive asked — Desired live e2e both engines; CI already starts both
  Decision: assumed — `LEAKYBUCKET_LIVE_REDIS` and `LEAKYBUCKET_LIVE_DRAGONFLY`; CI sets them to `127.0.0.1:6379` and `127.0.0.1:6380`; skip under `-short` or unset locally; CI must not skip.
  By: explore

- Q: Does the in-memory store expose Sleep/Wake/Close?
  Rank: additive asked — Desired reclaim hooks so the flush goroutine dies on Traefik reload
  Decision: assumed — only the Redis limiter (the type that may start a ticker) exports Sleep/Wake/Close; memory has no ticker.
  By: explore

- Q: Who already owns client address, user, tenant, Host, or trust hop for the limiter key?
  Rank: additive asked — Desired opaque key; caller prefixes; no HTTP
  Decision: resolved — none in this library. Caller owns identity and prefixes the key. Same as `tokenbucket` and `windowcounter`.
  By: explore

- Q: Where does Redis EXPIRE ttl come from?
  Rank: additive asked — Desired script includes EXPIRE; sibling tokenbucket requires ttl ≥ 1s
  Decision: assumed — constructor `ttl` on both stores, min 1s; script EXPIRE that many seconds; missing hash is empty water.
  By: explore
